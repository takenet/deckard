package metrics

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	prometheusclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/project"
	"github.com/takenet/deckard/internal/queue/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
)

var (
	prometheusStarted bool
	mutex             = sync.Mutex{}

	meter      otelmetric.Meter
	MetricsMap *QueueMetricsMap

	// Keep tracking of features used by clients to be able to deprecate them
	CodeUsage instrument.Int64Counter

	// Housekeeper
	HousekeeperExceedingStorageRemoved instrument.Int64Counter
	HousekeeperExceedingCacheRemoved   instrument.Int64Counter
	HousekeeperTTLStorageRemoved       instrument.Int64Counter
	HousekeeperTTLCacheRemoved         instrument.Int64Counter
	HousekeeperUnlock                  instrument.Int64Counter
	HousekeeperTaskLatency             instrument.Int64Histogram
	HousekeeperOldestMessage           instrument.Int64ObservableGauge
	HousekeeperTotalElements           instrument.Int64ObservableGauge

	// Message Pool
	QueueTimeout           instrument.Int64Counter
	QueueAck               instrument.Int64Counter
	QueueNack              instrument.Int64Counter
	QueueEmptyQueue        instrument.Int64Counter
	QueueEmptyQueueStorage instrument.Int64Counter
	QueueNotFoundInStorage instrument.Int64Counter

	// Storage
	StorageLatency instrument.Int64Histogram

	// Cache
	CacheLatency instrument.Int64Histogram

	// Auditor
	AuditorAddToStoreLatency instrument.Int64Histogram
	AuditorStoreLatency      instrument.Int64Histogram

	exporter *prometheus.Exporter
	registry *WrappedRegistry
)

func panicInstrumentationError(err error) {
	if err != nil {
		logger.S(context.Background()).Panicf("Failed to initialize instrument: %v", err)
	}
}

func init() {
	registry = NewWrappedRegistry(prometheusclient.NewRegistry(), createDefaultMetrics()...)

	var err error
	exporter, err = prometheus.New(
		prometheus.WithRegisterer(registry),
		prometheus.WithoutUnits(),
		prometheus.WithAggregationSelector(func(ik metric.InstrumentKind) aggregation.Aggregation {
			switch ik {
			case metric.InstrumentKindHistogram:
				return aggregation.ExplicitBucketHistogram{
					Boundaries: getHistogramBuckets(),
					NoMinMax:   false,
				}
			}

			return metric.DefaultAggregationSelector(ik)
		}))

	if err != nil {
		logger.S(context.Background()).Errorf("Failed to initialize prometheus exporter %v", err)
	}

	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(resource.Environment()),
	)

	global.SetMeterProvider(provider)

	createMetrics()
}

func ListenAndServe() {
	if !config.MetricsEnabled.GetBool() || prometheusStarted {
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	http.Handle(config.MetricsPath.Get(), promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: config.MetricsOpenMetricsEnabled.GetBool(),
	}))

	prometheusStarted = true

	port := fmt.Sprintf(":%d", config.MetricsPort.GetInt())

	logger.S(context.Background()).Debugf("Listening to metrics at %s%s", port, config.MetricsPath.Get())

	err := http.ListenAndServe(port, nil)

	if err != nil {
		logger.S(context.Background()).Error("Error starting prometheus exporter.", err)
	}
}

func createDefaultMetrics() []*dto.LabelPair {
	result := make([]*dto.LabelPair, 0)

	strPtr := func(s string) *string { return &s }

	otelAttributes := os.Getenv("OTEL_RESOURCE_ATTRIBUTES")

	if !strings.Contains(otelAttributes, "service.version") {
		if otelAttributes != "" {
			otelAttributes = otelAttributes + ","
		}

		otelAttributes = otelAttributes + "service.version=" + project.Version
	}

	attributes := strings.Split(otelAttributes, ",")
	for _, attribute := range attributes {
		parts := strings.Split(attribute, "=")

		if len(parts) != 2 {
			continue
		}

		result = append(result, &dto.LabelPair{
			Name:  strPtr(strings.Replace(parts[0], ".", "_", -1)),
			Value: strPtr(parts[1]),
		})
	}

	otelServiceName := os.Getenv("OTEL_SERVICE_NAME")
	if otelServiceName == "" {
		otelServiceName = project.Name
	}

	result = append(result, &dto.LabelPair{
		Name:  strPtr("service_name"),
		Value: strPtr(otelServiceName),
	})

	return result
}

func createMetrics() {
	meter = global.MeterProvider().Meter(project.Name)

	MetricsMap = NewQueueMetricsMap()

	// Housekeeper

	var err error

	CodeUsage, err = meter.Int64Counter(
		"deckard_code_usage",
		instrument.WithDescription("Number of times a specific feature was used by a client"),
	)
	panicInstrumentationError(err)

	HousekeeperExceedingStorageRemoved, err = meter.Int64Counter(
		"deckard_exceeding_messages_removed",
		instrument.WithDescription("Number of messages removed from storage for exceeding maximum queue size"),
	)
	panicInstrumentationError(err)

	HousekeeperExceedingCacheRemoved, err = meter.Int64Counter(
		"deckard_exceeding_messages_cache_removed",
		instrument.WithDescription("Number of messages removed from cache for exceeding maximum queue size"),
	)
	panicInstrumentationError(err)

	HousekeeperTaskLatency, err = meter.Int64Histogram(
		"deckard_housekeeper_task_latency",
		instrument.WithDescription("Time in milliseconds to complete a housekeeper task."),
		instrument.WithUnit(unit.Milliseconds),
	)
	panicInstrumentationError(err)

	HousekeeperOldestMessage, err = meter.Int64ObservableGauge(
		"deckard_oldest_message",
		instrument.WithDescription("Time the oldest queue message was used."),
		instrument.WithInt64Callback(func(_ context.Context, obs instrument.Int64Observer) error {
			return metrifyOldestMessages(obs)
		}))
	panicInstrumentationError(err)

	HousekeeperTotalElements, err = meter.Int64ObservableGauge(
		"deckard_total_messages",
		instrument.WithDescription("Number of messages a queue has."),
		instrument.WithInt64Callback(func(_ context.Context, obs instrument.Int64Observer) error {
			return metrifyTotalElements(obs)
		}))
	panicInstrumentationError(err)

	HousekeeperTTLStorageRemoved, err = meter.Int64Counter(
		"deckard_ttl_messages_removed",
		instrument.WithDescription("Number of messages removed from storage for ttl"),
	)
	panicInstrumentationError(err)

	HousekeeperTTLCacheRemoved, err = meter.Int64Counter(
		"deckard_ttl_messages_cache_removed",
		instrument.WithDescription("Number of messages removed from cache for ttl"),
	)
	panicInstrumentationError(err)

	HousekeeperUnlock, err = meter.Int64Counter(
		"deckard_message_unlock",
		instrument.WithDescription("Number of unlocked messages."),
	)
	panicInstrumentationError(err)

	// Queue

	QueueTimeout, err = meter.Int64Counter(
		"deckard_message_timeout",
		instrument.WithDescription("Number of message timeouts"),
	)
	panicInstrumentationError(err)

	QueueAck, err = meter.Int64Counter(
		"deckard_ack",
		instrument.WithDescription("Number of acks received"),
	)
	panicInstrumentationError(err)

	QueueNack, err = meter.Int64Counter(
		"deckard_nack",
		instrument.WithDescription("Number of nacks received"),
	)
	panicInstrumentationError(err)

	QueueEmptyQueue, err = meter.Int64Counter(
		"deckard_messages_empty_queue",
		instrument.WithDescription("Number of times a pull is made against an empty queue."),
	)
	panicInstrumentationError(err)

	QueueEmptyQueueStorage, err = meter.Int64Counter(
		"deckard_messages_empty_queue_not_found_storage",
		instrument.WithDescription("Number of times a pull is made against an empty queue because the messages were not found in the storage."),
	)
	panicInstrumentationError(err)

	QueueNotFoundInStorage, err = meter.Int64Counter(
		"deckard_messages_not_found_in_storage",
		instrument.WithDescription("Number of messages in cache but not found in the storage."),
	)
	panicInstrumentationError(err)

	// Storage

	StorageLatency, err = meter.Int64Histogram(
		"deckard_storage_latency",
		instrument.WithDescription("Storage access latency"),
		instrument.WithUnit(unit.Milliseconds),
	)
	panicInstrumentationError(err)

	// Cache

	CacheLatency, err = meter.Int64Histogram(
		"deckard_cache_latency",
		instrument.WithDescription("Cache access latency"),
		instrument.WithUnit(unit.Milliseconds),
	)
	panicInstrumentationError(err)

	// Auditor

	AuditorAddToStoreLatency, err = meter.Int64Histogram(
		"deckard_auditor_store_add_latency",
		instrument.WithDescription("Latency to add an entry to be saved by the audit storer"),
		instrument.WithUnit(unit.Milliseconds),
	)
	panicInstrumentationError(err)

	AuditorStoreLatency, err = meter.Int64Histogram(
		"deckard_auditor_store_latency",
		instrument.WithDescription("Latency sending elements to the audit database"),
		instrument.WithUnit(unit.Milliseconds),
	)
	panicInstrumentationError(err)
}

func metrifyOldestMessages(obs instrument.Int64Observer) error {
	return metrify(obs, &MetricsMap.OldestElement)
}

func metrifyTotalElements(obs instrument.Int64Observer) error {
	return metrify(obs, &MetricsMap.TotalElements)
}

func metrify(obs instrument.Int64Observer, data *map[string]int64) error {
	for key := range *data {
		obs.Observe((*data)[key], attribute.String("queue", key))
	}

	return nil
}

func getHistogramBuckets() []float64 {
	buckets, err := parseHistogramBuckets(config.MetricsHistogramBuckets.Get())

	if len(buckets) == 0 || err != nil {
		logger.L(context.Background()).Error("Error parsing buckets. Using default", zap.Error(err))

		buckets, _ = parseHistogramBuckets(config.MetricsHistogramBuckets.GetDefault().(string))
	}

	return buckets
}

func parseHistogramBuckets(data string) ([]float64, error) {
	if data == "" {
		return []float64{}, nil
	}

	buckets := strings.Split(data, ",")
	bucketsFloat := make([]float64, len(buckets))

	for i, value := range buckets {
		var err error
		bucketsFloat[i], err = utils.StrToFloat64(strings.TrimSpace(value))

		if err != nil {
			return nil, err
		}
	}

	return bucketsFloat, nil
}
