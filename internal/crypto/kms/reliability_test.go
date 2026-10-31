package kms

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testProvider struct {
	calls int
	err   error
}

func (p *testProvider) WrapKey(context.Context, []byte) ([]byte, string, error) {
	p.calls++
	if p.err != nil {
		return nil, "", p.err
	}
	return []byte("wrapped"), "test", nil
}

func (*testProvider) UnwrapKey(context.Context, []byte, string) ([]byte, error) {
	return make([]byte, 32), nil
}
func (*testProvider) GetKeyID() string                  { return "test" }
func (*testProvider) RotateKey(context.Context) error   { return nil }
func (*testProvider) HealthCheck(context.Context) error { return nil }

func TestResilientProviderRetriesAndOpensCircuit(t *testing.T) {
	transient := errors.New("transient")
	base := &testProvider{err: transient}
	policy := RetryPolicy{
		MaxAttempts:       2,
		InitialBackoff:    0,
		MaxBackoff:        0,
		CircuitThreshold:  1,
		CircuitResetAfter: 20 * time.Millisecond,
		ShouldRetry:       func(err error) bool { return errors.Is(err, transient) },
	}
	provider, err := NewResilientProvider(base, policy)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := provider.WrapKey(context.Background(), nil); !errors.Is(err, transient) {
		t.Fatalf("WrapKey error = %v, want transient", err)
	}
	if base.calls != 2 {
		t.Fatalf("attempt count = %d, want 2", base.calls)
	}
	if _, _, err := provider.WrapKey(context.Background(), nil); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("WrapKey with open circuit = %v", err)
	}
	time.Sleep(policy.CircuitResetAfter + time.Millisecond)
	base.err = nil
	if _, _, err := provider.WrapKey(context.Background(), nil); err != nil {
		t.Fatalf("half-open probe failed: %v", err)
	}
}

func TestResilientProviderValidationAndCancellation(t *testing.T) {
	if _, err := NewResilientProvider(nil, RetryPolicy{}); err == nil {
		t.Fatal("accepted nil provider")
	}
	base := &testProvider{}
	if _, err := NewResilientProvider(base, RetryPolicy{MaxAttempts: 0}); err == nil {
		t.Fatal("accepted invalid retry policy")
	}
	provider, err := NewResilientProvider(base, RetryPolicy{
		MaxAttempts: 1, CircuitThreshold: 1, CircuitResetAfter: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := provider.WrapKey(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("WrapKey error = %v, want context.Canceled", err)
	}
}
