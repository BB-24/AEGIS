package kms

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("KMS provider circuit is open")

type RetryPolicy struct {
	MaxAttempts       int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	CircuitThreshold  int
	CircuitResetAfter time.Duration
	ShouldRetry       func(error) bool
}

// ResilientProvider retries transient faults with bounded backoff and opens a
// simple consecutive-failure circuit after the configured threshold.
type ResilientProvider struct {
	provider Provider
	policy   RetryPolicy
	mu       sync.Mutex
	failures int
	openAt   time.Time
	probe    bool
}

func NewResilientProvider(provider Provider, policy RetryPolicy) (*ResilientProvider, error) {
	if provider == nil {
		return nil, errors.New("KMS provider is nil")
	}
	if policy.MaxAttempts < 1 || policy.InitialBackoff < 0 || policy.MaxBackoff < policy.InitialBackoff ||
		policy.CircuitThreshold < 1 || policy.CircuitResetAfter <= 0 {
		return nil, errors.New("invalid KMS retry or circuit-breaker policy")
	}
	return &ResilientProvider{provider: provider, policy: policy}, nil
}

func (p *ResilientProvider) begin() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failures < p.policy.CircuitThreshold {
		return nil
	}
	if time.Since(p.openAt) < p.policy.CircuitResetAfter {
		return ErrCircuitOpen
	}
	if p.probe {
		return ErrCircuitOpen
	}
	p.probe = true
	return nil
}

func (p *ResilientProvider) finish(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.probe = false
	if err == nil {
		p.failures = 0
		p.openAt = time.Time{}
		return
	}
	p.failures++
	if p.failures >= p.policy.CircuitThreshold {
		p.openAt = time.Now()
	}
}

func (p *ResilientProvider) run(ctx context.Context, fn func() error) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := p.begin(); err != nil {
		return err
	}
	var err error
	backoff := p.policy.InitialBackoff
	for attempt := 0; attempt < p.policy.MaxAttempts; attempt++ {
		if err = ctx.Err(); err != nil {
			err = fmt.Errorf("KMS operation canceled: %w", err)
			break
		}
		err = fn()
		if err == nil || !p.retryable(err) || attempt+1 == p.policy.MaxAttempts {
			break
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			err = fmt.Errorf("KMS retry canceled: %w", ctx.Err())
		case <-timer.C:
		}
		if err != nil && errors.Is(err, context.Canceled) {
			break
		}
		backoff *= 2
		if backoff > p.policy.MaxBackoff {
			backoff = p.policy.MaxBackoff
		}
	}
	p.finish(err)
	return err
}

func (p *ResilientProvider) retryable(err error) bool {
	if p.policy.ShouldRetry != nil {
		return p.policy.ShouldRetry(err)
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return true
	}
	type temporaryError interface{ Temporary() bool }
	var temporary temporaryError
	return errors.As(err, &temporary) && temporary.Temporary()
}

func (p *ResilientProvider) WrapKey(ctx context.Context, dek []byte) (wrapped []byte, keyID string, err error) {
	err = p.run(ctx, func() error {
		wrapped, keyID, err = p.provider.WrapKey(ctx, dek)
		return err
	})
	return
}

func (p *ResilientProvider) UnwrapKey(ctx context.Context, wrapped []byte, keyID string) (dek []byte, err error) {
	err = p.run(ctx, func() error {
		dek, err = p.provider.UnwrapKey(ctx, wrapped, keyID)
		return err
	})
	return
}

func (p *ResilientProvider) GetKeyID() string { return p.provider.GetKeyID() }

func (p *ResilientProvider) RotateKey(ctx context.Context) error {
	return p.run(ctx, func() error { return p.provider.RotateKey(ctx) })
}

func (p *ResilientProvider) HealthCheck(ctx context.Context) error {
	return p.run(ctx, func() error { return p.provider.HealthCheck(ctx) })
}
