package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/config"
)

var waitGroup = &sync.WaitGroup{}
var ctx = context.Background()

func TestNewAuditorWithoutServerShouldErrIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	config.AuditEnabled.Set(true)
	config.ElasticAddress.Set("http://localhost:9201/")

	_, err := NewAuditor(waitGroup)

	require.Error(t, err)
}

func TestNewAuditorWithServerShouldPingOkIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	config.AuditEnabled.Set(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer server.Close()

	config.ElasticAddress.Set(server.URL)

	_, err := NewAuditor(waitGroup)

	require.NoError(t, err)
}

func TestNewAuditorWithServerPongErrorShouldReturnErrorIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	config.AuditEnabled.Set(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	config.ElasticAddress.Set(server.URL)

	_, err := NewAuditor(waitGroup)

	require.Error(t, err)
}

func TestNewAuditorWithAuditDisabledShouldDoNothingIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	config.AuditEnabled.Set(false)

	var executed bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executed = true
	}))
	defer server.Close()

	config.ElasticAddress.Set(server.URL)

	auditor, err := NewAuditor(waitGroup)

	auditor.StartSender(ctx)
	auditor.Store(ctx, Entry{})

	require.NoError(t, err)
	require.False(t, executed)
}

func TestSenderShouldSendMaxEntriesIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithCancel(ctx)

	config.AuditEnabled.Set(true)
	maxEntriesSize = 2

	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			return
		}

		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		body = buf.String()
	}))
	defer server.Close()

	config.ElasticAddress.Set(server.URL)

	auditor, err := NewAuditor(waitGroup)
	require.NoError(t, err)

	go auditor.StartSender(ctx)

	for {
		if auditor.senderStarted {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	beforeStore := time.Now()

	firstEntry := Entry{ID: "1", Queue: "q1::1", LastScoreSubtract: 1, Breakpoint: "break1", Signal: ACK, Reason: "reason1", LockMs: 1}
	auditor.Store(ctx, firstEntry)
	secondEntry := Entry{ID: "2", Queue: "q2::2", LastScoreSubtract: 2, Breakpoint: "break2", Signal: NACK, Reason: "reason2", LockMs: 2}
	auditor.Store(ctx, secondEntry)

	for {
		if body != "" {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	result := strings.Split(body, "\n")

	require.Equal(t, indexAction, result[0]+"\n")
	require.Equal(t, indexAction, result[2]+"\n")
	require.Equal(t, "", result[len(result)-1])

	entry := &Entry{}

	// Check first entry
	err = json.Unmarshal([]byte(result[1]), entry)
	require.NoError(t, err)

	require.True(t, time.Now().After(entry.Timestamp) || time.Now().Equal(entry.Timestamp))
	require.True(t, entry.Timestamp.After(beforeStore) || entry.Timestamp.Equal(beforeStore))
	entry.Timestamp = time.Time{}
	firstEntry.QueuePrefix = "q1"
	firstEntry.QueueSuffix = "1"
	require.Equal(t, firstEntry, *entry)

	// Check second entry
	entry = &Entry{}

	err = json.Unmarshal([]byte(result[3]), entry)
	require.NoError(t, err)

	require.True(t, time.Now().After(entry.Timestamp) || time.Now().Equal(entry.Timestamp))
	require.True(t, entry.Timestamp.After(beforeStore) || entry.Timestamp.Equal(beforeStore))
	entry.Timestamp = time.Time{}
	secondEntry.QueuePrefix = "q2"
	secondEntry.QueueSuffix = "2"
	require.Equal(t, secondEntry, *entry)

	cancel()
	waitGroup.Wait()
}

func TestSenderShouldSendMaxTimeEntriesIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithCancel(ctx)

	config.AuditEnabled.Set(true)
	maxSendTime = 100 * time.Millisecond

	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			return
		}

		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		body = buf.String()
	}))
	defer server.Close()

	config.ElasticAddress.Set(server.URL)

	auditor, err := NewAuditor(waitGroup)
	require.NoError(t, err)

	go auditor.StartSender(ctx)

	for {
		if auditor.senderStarted {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	beforeStore := time.Now()

	firstEntry := Entry{ID: "1", Queue: "q1::1", LastScoreSubtract: 1, Breakpoint: "break1", Signal: ACK, Reason: "reason1", LockMs: 1}
	auditor.Store(ctx, firstEntry)

	for {
		if body != "" {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	result := strings.Split(body, "\n")

	require.Equal(t, indexAction, result[0]+"\n")
	require.Equal(t, "", result[len(result)-1])

	entry := &Entry{}

	// Check first entry
	err = json.Unmarshal([]byte(result[1]), entry)
	require.NoError(t, err)

	require.True(t, time.Now().After(entry.Timestamp) || time.Now().Equal(entry.Timestamp))
	require.True(t, entry.Timestamp.After(beforeStore) || entry.Timestamp.Equal(beforeStore))
	entry.Timestamp = time.Time{}
	firstEntry.QueuePrefix = "q1"
	firstEntry.QueueSuffix = "1"
	require.Equal(t, firstEntry, *entry)

	cancel()
	waitGroup.Wait()
}
