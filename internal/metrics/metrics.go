package metrics

import (
	"context"
	"net/http"
	"os"
	"strings"

	prometheusclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/project"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/resource"
)

var (
	prometheusStarted bool

	meter      otelmetric.Meter
	MetricsMap *MessagePoolMetricsMap

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
	MessagePoolTimeout           instrument.Int64Counter
	MessagePoolAck               instrument.Int64Counter
	MessagePoolNack              instrument.Int64Counter
	MessagePoolEmptyQueue        instrument.Int64Counter
	MessagePoolEmptyQueueStorage instrument.Int64Counter
	MessagePoolNotFoundInStorage instrument.Int64Counter

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
					Boundaries: []float64{0, 1, 2, 5, 10, 15, 20, 30, 35, 50, 100, 200, 400, 600, 800, 1000, 1500, 2000, 5000, 10000, 15000, 50000},
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

	if !prometheusStarted {
		prometheusStarted = true

		http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		}))

		go func() {
			err := http.ListenAndServe(":22022", nil)

			if err != nil {
				logger.S(context.Background()).Error("Error starting prometheus exporter.", err)
			}
		}()
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

	MetricsMap = NewMessagePoolMetricsMap()

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

	// MessagePool

	MessagePoolTimeout, err = meter.Int64Counter(
		"deckard_message_timeout",
		instrument.WithDescription("Number of message timeouts"),
	)
	panicInstrumentationError(err)

	MessagePoolAck, err = meter.Int64Counter(
		"deckard_ack",
		instrument.WithDescription("Number of acks received"),
	)
	panicInstrumentationError(err)

	MessagePoolNack, err = meter.Int64Counter(
		"deckard_nack",
		instrument.WithDescription("Number of nacks received"),
	)
	panicInstrumentationError(err)

	MessagePoolEmptyQueue, err = meter.Int64Counter(
		"deckard_messages_empty_queue",
		instrument.WithDescription("Number of times a pull is made against an empty queue."),
	)
	panicInstrumentationError(err)

	MessagePoolEmptyQueueStorage, err = meter.Int64Counter(
		"deckard_messages_empty_queue_not_found_storage",
		instrument.WithDescription("Number of times a pull is made against an empty queue because the messages were not found in the storage."),
	)
	panicInstrumentationError(err)

	MessagePoolNotFoundInStorage, err = meter.Int64Counter(
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
		instrument.WithDescription("Latency to add an entry to be saved by storer sender"),
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
