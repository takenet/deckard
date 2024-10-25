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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	instrument "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
)

var (
	prometheusStarted bool
	mutex             = sync.Mutex{}

	meter      metric.Meter
	MetricsMap *QueueMetricsMap

	// Keep tracking of features used by clients to be able to deprecate them
	CodeUsage metric.Int64Counter

	// Housekeepermetric
	HousekeeperExceedingStorageRemoved metric.Int64Counter
	HousekeeperExceedingCacheRemoved   metric.Int64Counter
	HousekeeperTTLStorageRemoved       metric.Int64Counter
	HousekeeperTTLCacheRemoved         metric.Int64Counter
	HousekeeperUnlock                  metric.Int64Counter
	HousekeeperTaskLatency             metric.Int64Histogram
	HousekeeperOldestMessage           metric.Int64ObservableGauge
	HousekeeperTotalElements           metric.Int64ObservableGauge

	// Message Pool
	QueueTimeout           metric.Int64Counter
	QueueAck               metric.Int64Counter
	QueueNack              metric.Int64Counter
	QueueEmptyQueue        metric.Int64Counter
	QueueEmptyQueueStorage metric.Int64Counter
	QueueNotFoundInStorage metric.Int64Counter

	// Storage
	StorageLatency metric.Int64Histogram

	// Cache
	CacheLatency metric.Int64Histogram

	// Auditor
	AuditorAddToStoreLatency metric.Int64Histogram
	AuditorStoreLatency      metric.Int64Histogram

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
		prometheus.WithAggregationSelector(func(ik instrument.InstrumentKind) instrument.Aggregation {
			switch ik {
			case instrument.InstrumentKindHistogram:
				return instrument.AggregationExplicitBucketHistogram{
					Boundaries: getHistogramBuckets(),
					NoMinMax:   false,
				}
			}

			return instrument.DefaultAggregationSelector(ik)
		}))

	if err != nil {
		logger.S(context.Background()).Errorf("Failed to initialize prometheus exporter %v", err)
	}

	provider := instrument.NewMeterProvider(
		instrument.WithReader(exporter),
		instrument.WithResource(resource.Environment()),
	)

	otel.SetMeterProvider(provider)

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
	meter = otel.GetMeterProvider().Meter(project.Name)

	MetricsMap = NewQueueMetricsMap()

	// Housekeeper

	var err error

	CodeUsage, err = meter.Int64Counter(
		"deckard_code_usage",
		metric.WithDescription("Number of times a specific feature was used by a client"),
	)
	panicInstrumentationError(err)

	HousekeeperExceedingStorageRemoved, err = meter.Int64Counter(
		"deckard_exceeding_messages_removed",
		metric.WithDescription("Number of messages removed from storage for exceeding maximum queue size"),
	)
	panicInstrumentationError(err)

	HousekeeperExceedingCacheRemoved, err = meter.Int64Counter(
		"deckard_exceeding_messages_cache_removed",
		metric.WithDescription("Number of messages removed from cache for exceeding maximum queue size"),
	)
	panicInstrumentationError(err)

	HousekeeperTaskLatency, err = meter.Int64Histogram(
		"deckard_housekeeper_task_latency",
		metric.WithDescription("Time in milliseconds to complete a housekeeper task."),
		metric.WithUnit("ms"),
	)
	panicInstrumentationError(err)

	HousekeeperOldestMessage, err = meter.Int64ObservableGauge(
		"deckard_oldest_message",
		metric.WithDescription("Time the oldest queue message was used."),
		metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
			return metrifyOldestMessages(obs)
		}))
	panicInstrumentationError(err)

	HousekeeperTotalElements, err = meter.Int64ObservableGauge(
		"deckard_total_messages",
		metric.WithDescription("Number of messages a queue has."),
		metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
			return metrifyTotalElements(obs)
		}))
	panicInstrumentationError(err)

	HousekeeperTTLStorageRemoved, err = meter.Int64Counter(
		"deckard_ttl_messages_removed",
		metric.WithDescription("Number of messages removed from storage for ttl"),
	)
	panicInstrumentationError(err)

	HousekeeperTTLCacheRemoved, err = meter.Int64Counter(
		"deckard_ttl_messages_cache_removed",
		metric.WithDescription("Number of messages removed from cache for ttl"),
	)
	panicInstrumentationError(err)

	HousekeeperUnlock, err = meter.Int64Counter(
		"deckard_message_unlock",
		metric.WithDescription("Number of unlocked messages."),
	)
	panicInstrumentationError(err)

	// Queue

	QueueTimeout, err = meter.Int64Counter(
		"deckard_message_timeout",
		metric.WithDescription("Number of message timeouts"),
	)
	panicInstrumentationError(err)

	QueueAck, err = meter.Int64Counter(
		"deckard_ack",
		metric.WithDescription("Number of acks received"),
	)
	panicInstrumentationError(err)

	QueueNack, err = meter.Int64Counter(
		"deckard_nack",
		metric.WithDescription("Number of nacks received"),
	)
	panicInstrumentationError(err)

	QueueEmptyQueue, err = meter.Int64Counter(
		"deckard_messages_empty_queue",
		metric.WithDescription("Number of times a pull is made against an empty queue."),
	)
	panicInstrumentationError(err)

	QueueEmptyQueueStorage, err = meter.Int64Counter(
		"deckard_messages_empty_queue_not_found_storage",
		metric.WithDescription("Number of times a pull is made against an empty queue because the messages were not found in the storage."),
	)
	panicInstrumentationError(err)

	QueueNotFoundInStorage, err = meter.Int64Counter(
		"deckard_messages_not_found_in_storage",
		metric.WithDescription("Number of messages in cache but not found in the storage."),
	)
	panicInstrumentationError(err)

	// Storage

	StorageLatency, err = meter.Int64Histogram(
		"deckard_storage_latency",
		metric.WithDescription("Storage access latency"),
		metric.WithUnit("ms"),
	)
	panicInstrumentationError(err)

	// Cache

	CacheLatency, err = meter.Int64Histogram(
		"deckard_cache_latency",
		metric.WithDescription("Cache access latency"),
		metric.WithUnit("ms"),
	)
	panicInstrumentationError(err)

	// Auditor

	AuditorAddToStoreLatency, err = meter.Int64Histogram(
		"deckard_auditor_store_add_latency",
		metric.WithDescription("Latency to add an entry to be saved by the audit storer"),
		metric.WithUnit("ms"),
	)
	panicInstrumentationError(err)

	AuditorStoreLatency, err = meter.Int64Histogram(
		"deckard_auditor_store_latency",
		metric.WithDescription("Latency sending elements to the audit database"),
		metric.WithUnit("ms"),
	)
	panicInstrumentationError(err)
}

func metrifyOldestMessages(obs metric.Int64Observer) error {
	return metrify(obs, &MetricsMap.OldestElement)
}

func metrifyTotalElements(obs metric.Int64Observer) error {
	return metrify(obs, &MetricsMap.TotalElements)
}

func metrify(obs metric.Int64Observer, data *map[string]int64) error {
	for key := range *data {
		obs.Observe((*data)[key], metric.WithAttributeSet(attribute.NewSet(attribute.String("queue", key))))
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
