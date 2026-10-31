package kms

import (
	"context"
	"errors"
	"fmt"
	"strings"

	cloudkms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/googleapis/gax-go/v2"
)

type GCPConfig struct {
	KeyName string
}

type gcpKMSAPI interface {
	Encrypt(context.Context, *kmspb.EncryptRequest, ...gax.CallOption) (*kmspb.EncryptResponse, error)
	Decrypt(context.Context, *kmspb.DecryptRequest, ...gax.CallOption) (*kmspb.DecryptResponse, error)
	GetCryptoKey(context.Context, *kmspb.GetCryptoKeyRequest, ...gax.CallOption) (*kmspb.CryptoKey, error)
	CreateCryptoKeyVersion(context.Context, *kmspb.CreateCryptoKeyVersionRequest, ...gax.CallOption) (*kmspb.CryptoKeyVersion, error)
	UpdateCryptoKeyPrimaryVersion(context.Context, *kmspb.UpdateCryptoKeyPrimaryVersionRequest, ...gax.CallOption) (*kmspb.CryptoKey, error)
}

// GCPProvider uses Google Cloud KMS CryptoKeys and application-default credentials.
type GCPProvider struct {
	client  gcpKMSAPI
	keyName string
	closer  interface{ Close() error }
}

var _ Provider = (*GCPProvider)(nil)

func NewGCPProvider(ctx context.Context, config GCPConfig) (*GCPProvider, error) {
	if strings.TrimSpace(config.KeyName) == "" {
		return nil, errors.New("Google Cloud KMS CryptoKey resource name is required")
	}
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	client, err := cloudkms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create Google Cloud KMS client: %w", err)
	}
	provider, err := NewGCPProviderWithClient(client, config.KeyName)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	provider.closer = client
	return provider, nil
}

func NewGCPProviderWithClient(client gcpKMSAPI, keyName string) (*GCPProvider, error) {
	if client == nil {
		return nil, errors.New("Google Cloud KMS client is nil")
	}
	if strings.TrimSpace(keyName) == "" {
		return nil, errors.New("Google Cloud KMS CryptoKey resource name is empty")
	}
	return &GCPProvider{client: client, keyName: keyName}, nil
}

func (p *GCPProvider) WrapKey(ctx context.Context, dek []byte) ([]byte, string, error) {
	if err := validateContext(ctx); err != nil {
		return nil, "", err
	}
	if len(dek) != 32 {
		return nil, "", fmt.Errorf("DEK must be 32 bytes, got %d", len(dek))
	}
	plaintext := append([]byte(nil), dek...)
	defer wipe(plaintext)
	out, err := p.client.Encrypt(ctx, &kmspb.EncryptRequest{Name: p.keyName, Plaintext: plaintext})
	if err != nil {
		return nil, "", fmt.Errorf("Google Cloud KMS Encrypt: %w", err)
	}
	if out == nil || len(out.Ciphertext) == 0 {
		return nil, "", errors.New("Google Cloud KMS returned an empty ciphertext")
	}
	return append([]byte(nil), out.Ciphertext...), p.keyName, nil
}

func (p *GCPProvider) UnwrapKey(ctx context.Context, wrapped []byte, keyID string) ([]byte, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if len(wrapped) == 0 || keyID != p.keyName {
		return nil, errors.New("wrapped DEK or matching Google Cloud KMS key ID is required")
	}
	ciphertext := append([]byte(nil), wrapped...)
	defer wipe(ciphertext)
	out, err := p.client.Decrypt(ctx, &kmspb.DecryptRequest{Name: p.keyName, Ciphertext: ciphertext})
	if err != nil {
		return nil, fmt.Errorf("Google Cloud KMS Decrypt: %w", err)
	}
	if out == nil || len(out.Plaintext) != 32 {
		if out != nil {
			wipe(out.Plaintext)
		}
		return nil, errors.New("Google Cloud KMS returned a DEK with invalid size")
	}
	dek := append([]byte(nil), out.Plaintext...)
	wipe(out.Plaintext)
	return dek, nil
}

func (p *GCPProvider) GetKeyID() string { return p.keyName }

func (p *GCPProvider) RotateKey(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	version, err := p.client.CreateCryptoKeyVersion(ctx, &kmspb.CreateCryptoKeyVersionRequest{
		Parent:           p.keyName,
		CryptoKeyVersion: &kmspb.CryptoKeyVersion{},
	})
	if err != nil {
		return fmt.Errorf("create Google Cloud KMS key version: %w", err)
	}
	if version == nil {
		return errors.New("Google Cloud KMS created no key version")
	}
	versionID := version.GetName()[strings.LastIndex(version.GetName(), "/")+1:]
	if versionID == "" {
		return errors.New("Google Cloud KMS returned a key version without an ID")
	}
	_, err = p.client.UpdateCryptoKeyPrimaryVersion(ctx, &kmspb.UpdateCryptoKeyPrimaryVersionRequest{
		Name:               p.keyName,
		CryptoKeyVersionId: versionID,
	})
	if err != nil {
		return fmt.Errorf("set Google Cloud KMS primary key version: %w", err)
	}
	return nil
}

func (p *GCPProvider) HealthCheck(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	key, err := p.client.GetCryptoKey(ctx, &kmspb.GetCryptoKeyRequest{Name: p.keyName})
	if err != nil {
		return fmt.Errorf("Google Cloud KMS health check: %w", err)
	}
	if key == nil || key.GetPrimary() == nil {
		return errors.New("Google Cloud KMS key has no primary version")
	}
	return nil
}

func (p *GCPProvider) Close() error {
	if p.closer == nil {
		return nil
	}
	return p.closer.Close()
}
