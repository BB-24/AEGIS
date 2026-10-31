package kms

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestInstrumentedProviderRecordsOperationsWithoutKeyLabels(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewMetrics(registry)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}
	instrumented, err := NewInstrumentedProvider(provider, metrics, "local")
	if err != nil {
		t.Fatal(err)
	}
	wrapped, keyID, err := instrumented.WrapKey(context.Background(), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := instrumented.UnwrapKey(context.Background(), wrapped, keyID); err != nil {
		t.Fatal(err)
	}
	if err := instrumented.HealthCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := instrumented.RotateKey(context.Background()); err != nil {
		t.Fatal(err)
	}
	failing, err := NewInstrumentedProvider(&testProvider{err: errors.New("provider failure")}, metrics, "failing")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := failing.WrapKey(context.Background(), make([]byte, 32)); err == nil {
		t.Fatal("expected provider error")
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 2 {
		t.Fatalf("registered metric families = %d, want 2", len(families))
	}
	if _, err := NewMetrics(registry); err == nil {
		t.Fatal("accepted duplicate metric registration")
	}
	if _, err := NewInstrumentedProvider(nil, metrics, "local"); err == nil {
		t.Fatal("accepted nil provider")
	}
	if _, err := NewInstrumentedProvider(provider, nil, "local"); err == nil {
		t.Fatal("accepted nil metrics")
	}
	if _, err := NewInstrumentedProvider(provider, metrics, ""); err == nil {
		t.Fatal("accepted empty provider name")
	}
}

func TestNewMetricsRequiresRegisterer(t *testing.T) {
	if _, err := NewMetrics(nil); err == nil {
		t.Fatal("accepted nil registerer")
	}
}
