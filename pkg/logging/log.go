package logging

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
)

func NewPrometheusHook() *PrometheusHook {
	counter := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "log_statements_total",
			Help: "Number of log statements, differentiated by log level.",
		},
		[]string{"level"},
	)

	return &PrometheusHook{
		counter: counter,
	}
}

type PrometheusHook struct {
	counter *prometheus.CounterVec
}

func (h *PrometheusHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *PrometheusHook) Fire(e *logrus.Entry) error {
	h.counter.WithLabelValues(e.Level.String()).Inc()
	return nil
}
