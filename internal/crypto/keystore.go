package crypto

import (
	"context"
	"errors"
	"fmt"
)

var ErrRotationUnsupported = errors.New("KMS provider does not support KEK rotation")

// KMSProvider wraps and unwraps keys using a KMS-managed key encryption key.
type KMSProvider interface {
	Encrypt(ctx context.Context, plaintext []byte) (ciphertext []byte, keyID string, err error)
	Decrypt(ctx context.Context, ciphertext []byte, keyID string) ([]byte, error)
	GenerateDataKey(ctx context.Context, keySpec string) (plaintext, ciphertext []byte, keyID string, err error)
}

// KeyManager provides the key operations used by chunk envelope encryption.
type KeyManager interface {
	WrapKey(ctx context.Context, dek []byte) (wrappedDEK []byte, keyID string, err error)
	UnwrapKey(ctx context.Context, wrappedDEK []byte, keyID string) ([]byte, error)
	RotateKEK(ctx context.Context) error
}

// ProviderKeyManager adapts a KMS provider to the chunk key-management API.
type ProviderKeyManager struct {
	provider KMSProvider
}

var _ KeyManager = (*ProviderKeyManager)(nil)

// NewKeyManager adapts a KMS provider for use by the AES-GCM chunk encryptor.
func NewKeyManager(provider KMSProvider) (*ProviderKeyManager, error) {
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
	wrapped, keyID, err := m.provider.Encrypt(ctx, keyMaterial)
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
	dek, err := m.provider.Decrypt(ctx, wrappedDEK, keyID)
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
	rotator, ok := m.provider.(interface {
		RotateKEK(context.Context) error
	})
	if !ok {
		return ErrRotationUnsupported
	}
	if err := rotator.RotateKEK(ctx); err != nil {
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
