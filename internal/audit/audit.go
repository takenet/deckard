package audit

//go:generate mockgen -destination=../mocks/mock_audit.go -package=mocks -source=audit.go

// TODO: Make possible to send audit to other places instead of ElasticSearch, like Kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/metrics"
	"github.com/takenet/deckard/internal/project"
	"github.com/takenet/deckard/internal/queue/entities"
	"github.com/takenet/deckard/internal/queue/utils"
	"go.uber.org/zap"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
)

const esIndex = project.Name
const indexAction = "{ \"index\" : { \"_index\" : \"" + esIndex + "\"} }\n"

var maxSendTime = time.Minute * 5
var maxEntriesSize = 200

type Entry struct {
	ID                string    `json:"id"`
	Queue             string    `json:"queue"`
	QueuePrefix       string    `json:"queue_prefix"`
	QueueSuffix       string    `json:"queue_suffix"`
	LastScoreSubtract float64   `json:"last_score_subtract"`
	Timestamp         time.Time `json:"timestamp"`
	Breakpoint        string    `json:"breakpoint"`
	Signal            Signal    `json:"signal"`
	Reason            string    `json:"reason"`
	LockMs            int64     `json:"lock_ms"`
}

type Auditor interface {
	Store(ctx context.Context, entry Entry)
}

type AuditorImpl struct {
	api           *esapi.API
	entries       chan Entry
	waitGroup     *sync.WaitGroup
	enabled       bool
	senderStarted bool
}

// NewAuditor returns a new auditor with a elastic search.
// If the audit is disabled by config, it returns a noop AuditorImpl
func NewAuditor(waitGroup *sync.WaitGroup) (*AuditorImpl, error) {
	if !config.AuditEnabled.GetBool() {
		return &AuditorImpl{
			enabled:   false,
			waitGroup: waitGroup,
		}, nil
	}

	esClient, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{config.ElasticAddress.Get()},
		Username:  config.ElasticUser.Get(),
		Password:  config.ElasticPassword.Get(),
	})

	if err != nil {
		zap.L().Error("Error creating audit client", zap.Error(err))

		return nil, err
	}

	api := esapi.New(esClient)

	pong, err := api.Ping()
	if err != nil {
		zap.L().Error("Error creating audit client", zap.Error(err))

		return nil, err
	}

	if pong.IsError() {
		zap.S().Error("Error creating audit client", pong.String())

		return nil, fmt.Errorf("error with ping request")
	}

	return &AuditorImpl{
		api:       api,
		entries:   make(chan Entry, 10000),
		waitGroup: waitGroup,
		enabled:   true,
	}, nil
}

// StartSender sends the audit logs from executed operations to Elastic Search.
func (a *AuditorImpl) StartSender(ctx context.Context) {
	if !a.enabled {
		return
	}

	a.waitGroup.Add(1)
	defer a.waitGroup.Done()

	entriesToSend := make([]Entry, 0, maxEntriesSize)

	for {
		a.senderStarted = true

		select {
		case entry := <-a.entries:
			entriesToSend = append(entriesToSend, entry)
			if len(entriesToSend) == maxEntriesSize {
				a.send(ctx, entriesToSend...)

				entriesToSend = make([]Entry, 0, maxEntriesSize)
			}
		case <-time.After(maxSendTime):
			if len(entriesToSend) > 0 {
				a.send(ctx, entriesToSend...)

				entriesToSend = make([]Entry, 0, maxEntriesSize)
			}
		case <-ctx.Done():
			zap.S().Info("Sending remaining audit data.")

			close(a.entries)

			a.send(ctx, entriesToSend...)

			return
		}
	}
}

func (a *AuditorImpl) send(ctx context.Context, entries ...Entry) {
	if len(entries) == 0 {
		return
	}

	start := time.Now()
	defer func() {
		metrics.AuditorStoreLatency.Record(ctx, utils.ElapsedTime(start))
	}()

	body := ""

	for i, entry := range entries {
		marshalled, err := json.Marshal(entry)
		if err != nil {
			logger.S(ctx).Error("Error transforming entry. Skipping it.", err)
			continue
		}

		if i > 0 {
			body += "\n"
		}

		body += fmt.Sprint(indexAction, string(marshalled))
	}

	logger.S(ctx).Debugf("Sending %d entries to audit", len(entries))

	opt := a.api.Bulk
	response, err := a.api.Bulk(strings.NewReader(body+"\n"), opt.WithRefresh("true"), opt.WithIndex(esIndex))

	if err != nil {
		logger.S(ctx).Error("Error sending audit data.", err)
		return
	}

	if response.IsError() {
		logger.S(ctx).Error("Error sending audit data.", response.String())
		return
	}

	// exhaust body so connection can be reused.
	_, _ = ioutil.ReadAll(response.Body)
	_ = response.Body.Close()
}

func (a *AuditorImpl) Store(ctx context.Context, entry Entry) {
	if !a.enabled {
		return
	}

	entry.Timestamp = time.Now()

	queuePrefix, queueSuffix := entities.GetQueueParts(entry.Queue)

	entry.QueuePrefix = queuePrefix

	if queueSuffix != "" {
		entry.QueueSuffix = queueSuffix
	}

	defer func() {
		metrics.AuditorAddToStoreLatency.Record(ctx, utils.ElapsedTime(entry.Timestamp))
	}()

	a.entries <- entry
}
