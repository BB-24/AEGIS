package kms

import (
	"context"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	duration *prometheus.HistogramVec
	errors   *prometheus.CounterVec
}

func NewMetrics(registerer prometheus.Registerer) (*Metrics, error) {
	if registerer == nil {
		return nil, errors.New("Prometheus registerer is nil")
	}
	metrics := &Metrics{
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "aegis",
			Subsystem: "kms",
			Name:      "operation_duration_seconds",
			Help:      "Duration of KMS provider operations.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"provider", "operation"}),
		errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "aegis",
			Subsystem: "kms",
			Name:      "operation_errors_total",
			Help:      "Number of failed KMS provider operations.",
		}, []string{"provider", "operation"}),
	}
	if err := registerer.Register(metrics.duration); err != nil {
		return nil, err
	}
	if err := registerer.Register(metrics.errors); err != nil {
		registerer.Unregister(metrics.duration)
		return nil, err
	}
	return metrics, nil
}

// InstrumentedProvider records latency and errors without exposing key data.
type InstrumentedProvider struct {
	provider Provider
	metrics  *Metrics
	name     string
}

func NewInstrumentedProvider(provider Provider, metrics *Metrics, name string) (*InstrumentedProvider, error) {
	if provider == nil || metrics == nil || name == "" {
		return nil, errors.New("provider, metrics, and provider name are required")
	}
	return &InstrumentedProvider{provider: provider, metrics: metrics, name: name}, nil
}

func (p *InstrumentedProvider) record(operation string, fn func() error) error {
	start := time.Now()
	err := fn()
	p.metrics.duration.WithLabelValues(p.name, operation).Observe(time.Since(start).Seconds())
	if err != nil {
		p.metrics.errors.WithLabelValues(p.name, operation).Inc()
	}
	return err
}

func (p *InstrumentedProvider) WrapKey(ctx context.Context, dek []byte) (wrapped []byte, keyID string, err error) {
	err = p.record("wrap", func() error {
		wrapped, keyID, err = p.provider.WrapKey(ctx, dek)
		return err
	})
	return
}

func (p *InstrumentedProvider) UnwrapKey(ctx context.Context, wrapped []byte, keyID string) (dek []byte, err error) {
	err = p.record("unwrap", func() error {
		dek, err = p.provider.UnwrapKey(ctx, wrapped, keyID)
		return err
	})
	return
}

func (p *InstrumentedProvider) GetKeyID() string { return p.provider.GetKeyID() }

func (p *InstrumentedProvider) RotateKey(ctx context.Context) error {
	return p.record("rotate", func() error { return p.provider.RotateKey(ctx) })
}

func (p *InstrumentedProvider) HealthCheck(ctx context.Context) error {
	return p.record("health", func() error { return p.provider.HealthCheck(ctx) })
}
