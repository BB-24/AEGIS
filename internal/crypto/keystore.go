package crypto

import (
	"context"
	"errors"
	"fmt"

	"github.com/aegis/aegis/internal/crypto/kms"
)

var ErrRotationUnsupported = kms.ErrRotationUnsupported

// KeyManager provides the key operations used by chunk envelope encryption.
type KeyManager interface {
	WrapKey(ctx context.Context, dek []byte) (wrappedDEK []byte, keyID string, err error)
	UnwrapKey(ctx context.Context, wrappedDEK []byte, keyID string) ([]byte, error)
	RotateKEK(ctx context.Context) error
}

// ProviderKeyManager adapts a KMS provider to the chunk key-management API.
type ProviderKeyManager struct {
	provider kms.Provider
}

var _ KeyManager = (*ProviderKeyManager)(nil)

// NewKeyManager adapts a KMS provider for use by the AES-GCM chunk encryptor.
func NewKeyManager(provider kms.Provider) (*ProviderKeyManager, error) {
	if provider == nil {
		return nil, errors.New("KMS provider is nil")
	}
	return &ProviderKeyManager{provider: provider}, nil
}

// WrapKey encrypts a DEK with the configured KMS provider.
func (m *ProviderKeyManager) WrapKey(ctx context.Context, dek []byte) ([]byte, string, error) {
	if err := validateContext(ctx); err != nil {
		return nil, "", err
	}
	if len(dek) != DEKSize {
		return nil, "", fmt.Errorf("DEK must be %d bytes, got %d", DEKSize, len(dek))
	}
	keyMaterial := append([]byte(nil), dek...)
	defer zeroize(keyMaterial)
	wrapped, keyID, err := m.provider.WrapKey(ctx, keyMaterial)
	if err != nil {
		zeroize(wrapped)
		return nil, "", fmt.Errorf("KMS wrap DEK: %w", err)
	}
	if len(wrapped) == 0 || keyID == "" {
		zeroize(wrapped)
		return nil, "", errors.New("KMS provider returned an empty wrapped key or key ID")
	}
	return wrapped, keyID, nil
}

// UnwrapKey decrypts a wrapped DEK through the KMS provider.
func (m *ProviderKeyManager) UnwrapKey(ctx context.Context, wrappedDEK []byte, keyID string) ([]byte, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if len(wrappedDEK) == 0 {
		return nil, errors.New("wrapped DEK is empty")
	}
	if keyID == "" {
		return nil, errors.New("KMS key ID is empty")
	}
	dek, err := m.provider.UnwrapKey(ctx, wrappedDEK, keyID)
	if err != nil {
		zeroize(dek)
		return nil, fmt.Errorf("KMS unwrap DEK: %w", err)
	}
	if len(dek) != DEKSize {
		zeroize(dek)
		return nil, fmt.Errorf("KMS returned a DEK with invalid size %d", len(dek))
	}
	return dek, nil
}

// RotateKEK delegates rotation to providers that implement KEK rotation.
func (m *ProviderKeyManager) RotateKEK(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := m.provider.RotateKey(ctx); err != nil {
		return fmt.Errorf("rotate KMS KEK: %w", err)
	}
	return nil
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("operation canceled: %w", err)
	}
	return nil
}
