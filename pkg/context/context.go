package context

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sirupsen/logrus"
)

type (
	loggerKey       struct{}
	errorCounterKey struct{}
	metricsChKey    struct{}
)

func WithLogger(ctx context.Context, logger *logrus.Entry) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

func GetLogger(ctx context.Context) *logrus.Entry {
	logger := ctx.Value(loggerKey{})

	if logger == nil {
		return logrus.NewEntry(logrus.StandardLogger())
	}

	return logger.(*logrus.Entry)
}

func WithErrorCounter(ctx context.Context, counter prometheus.Counter) context.Context {
	return context.WithValue(ctx, errorCounterKey{}, counter)
}

func IncrementErrorCounter(ctx context.Context) {
	counter := ctx.Value(errorCounterKey{})

	if counter == nil {
		panic(fmt.Errorf("context without error counter provided"))
	}

	counter.(prometheus.Counter).Inc()
}

func WithMetricsCh(ctx context.Context, metricsCh chan<- prometheus.Metric) context.Context {
	return context.WithValue(ctx, metricsChKey{}, metricsCh)
}

func GetMetricsCh(ctx context.Context) chan<- prometheus.Metric {
	metricsCh := ctx.Value(metricsChKey{})

	if metricsCh == nil {
		panic(fmt.Errorf("context without metrics channel provided"))
	}

	return metricsCh.(chan<- prometheus.Metric)
}
