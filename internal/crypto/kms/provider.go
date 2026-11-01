// Package kms contains pluggable key-encryption-key providers.
package kms

import (
	"context"
	"errors"
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
