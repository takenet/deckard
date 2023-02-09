package metrics

import (
	"strings"

	prom "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

var (
	_ prom.Gatherer   = &WrappedRegistry{}
	_ prom.Registerer = &WrappedRegistry{}
)

type WrappedRegistry struct {
	labels       []*dto.LabelPair
	promRegistry *prom.Registry
}

func NewWrappedRegistry(promRegistry *prom.Registry, labels ...*dto.LabelPair) *WrappedRegistry {
	return &WrappedRegistry{
		labels:       labels,
		promRegistry: promRegistry,
	}
}

func (wr *WrappedRegistry) Gather() ([]*dto.MetricFamily, error) {
	families, err := wr.promRegistry.Gather()
	if err != nil {
		return nil, err
	}

	for _, f := range families {
		if *f.Name == "target_info" {
			continue
		}

		for _, m := range f.Metric {
			m.Label = append(m.Label, wr.labels...)
		}
	}

	// See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/3071
	var returnFamilies []*dto.MetricFamily
	for _, f := range families {
		if strings.Contains(*f.Name, "rpc_server_duration") {
			continue
		}

		returnFamilies = append(returnFamilies, f)
	}

	return returnFamilies, nil
}

func (wr *WrappedRegistry) Register(collector prom.Collector) error {
	return wr.promRegistry.Register(collector)
}

func (wr *WrappedRegistry) MustRegister(collector ...prom.Collector) {
	wr.promRegistry.MustRegister(collector...)
}

func (wr *WrappedRegistry) Unregister(collector prom.Collector) bool {
	return wr.promRegistry.Unregister(collector)
}
