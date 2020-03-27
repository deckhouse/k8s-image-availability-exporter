package registry_checker

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Collect implements prometheus.Collector.
func (rc *RegistryChecker) Collect(ch chan<- prometheus.Metric) {
	metrics := rc.imageStore.ExtractMetrics()

	for _, m := range metrics {
		ch <- m
	}
}

// Describe implements prometheus.Collector.
func (rc *RegistryChecker) Describe(_ chan<- *prometheus.Desc) {}
