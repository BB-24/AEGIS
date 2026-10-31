// Package kms contains pluggable key-encryption-key providers.
package kms

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

var ErrRotationUnsupported = errors.New("KMS provider does not support manual key rotation")

// Provider wraps and unwraps data-encryption keys using an external KEK.
type Provider interface {
	WrapKey(ctx context.Context, dek []byte) (wrappedDEK []byte, keyID string, err error)
	UnwrapKey(ctx context.Context, wrappedDEK []byte, keyID string) ([]byte, error)
	GetKeyID() string
	RotateKey(ctx context.Context) error
	HealthCheck(ctx context.Context) error
}

// randomDataKey generates a fresh AES-256 data key for compatibility callers.
func randomDataKey() ([]byte, error) {
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		wipe(dek)
		return nil, fmt.Errorf("generate data key: %w", err)
	}
	return dek, nil
}
