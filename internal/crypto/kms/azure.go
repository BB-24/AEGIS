package kms

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
)

const defaultAzureWrapAlgorithm = azkeys.EncryptionAlgorithmRSAOAEP256

type AzureConfig struct {
	VaultURL   string
	KeyName    string
	KeyVersion string
	KeyID      string
	Algorithm  azkeys.EncryptionAlgorithm
}

type azureKeysClient interface {
	GetKey(context.Context, string, string, *azkeys.GetKeyOptions) (azkeys.GetKeyResponse, error)
	WrapKey(context.Context, string, string, azkeys.KeyOperationParameters, *azkeys.WrapKeyOptions) (azkeys.WrapKeyResponse, error)
	UnwrapKey(context.Context, string, string, azkeys.KeyOperationParameters, *azkeys.UnwrapKeyOptions) (azkeys.UnwrapKeyResponse, error)
	RotateKey(context.Context, string, *azkeys.RotateKeyOptions) (azkeys.RotateKeyResponse, error)
}

// AzureProvider uses Azure Key Vault RSA keys to wrap DEKs with RSA-OAEP-256.
type AzureProvider struct {
	client    azureKeysClient
	vaultURL  string
	keyName   string
	algorithm azkeys.EncryptionAlgorithm
	mu        sync.RWMutex
	keyID     string
	version   string
}

var _ Provider = (*AzureProvider)(nil)

func NewAzureProvider(ctx context.Context, config AzureConfig) (*AzureProvider, error) {
	if strings.TrimSpace(config.VaultURL) == "" || strings.TrimSpace(config.KeyName) == "" {
		return nil, errors.New("Azure Key Vault URL and key name are required")
	}
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("create Azure credential: %w", err)
	}
	client, err := azkeys.NewClient(config.VaultURL, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create Azure Key Vault client: %w", err)
	}
	provider, err := NewAzureProviderWithClient(client, config)
	if err != nil {
		return nil, err
	}
	key, err := client.GetKey(ctx, config.KeyName, config.KeyVersion, nil)
	if err != nil {
		return nil, fmt.Errorf("get Azure Key Vault key: %w", err)
	}
	if key.KeyBundle.Key == nil || key.KeyBundle.Key.KID == nil {
		return nil, errors.New("Azure Key Vault returned no key identifier")
	}
	provider.setKeyID(string(*key.KeyBundle.Key.KID))
	return provider, nil
}

func NewAzureProviderWithClient(client azureKeysClient, config AzureConfig) (*AzureProvider, error) {
	if client == nil {
		return nil, errors.New("Azure Key Vault client is nil")
	}
	if strings.TrimSpace(config.VaultURL) == "" || strings.TrimSpace(config.KeyName) == "" {
		return nil, errors.New("Azure Key Vault URL and key name are required")
	}
	if config.Algorithm == "" {
		config.Algorithm = defaultAzureWrapAlgorithm
	}
	if config.KeyVersion != "" {
		config.KeyID = config.VaultURL + "/keys/" + config.KeyName + "/" + config.KeyVersion
	}
	return &AzureProvider{
		client:    client,
		vaultURL:  strings.TrimRight(config.VaultURL, "/"),
		keyName:   config.KeyName,
		algorithm: config.Algorithm,
		keyID:     config.KeyID,
		version:   config.KeyVersion,
	}, nil
}

func (p *AzureProvider) WrapKey(ctx context.Context, dek []byte) ([]byte, string, error) {
	if err := validateContext(ctx); err != nil {
		return nil, "", err
	}
	if len(dek) != 32 {
		return nil, "", fmt.Errorf("DEK must be 32 bytes, got %d", len(dek))
	}
	version := p.currentVersion()
	if version == "" {
		return nil, "", errors.New("Azure Key Vault key version is unavailable")
	}
	plaintext := append([]byte(nil), dek...)
	defer wipe(plaintext)
	out, err := p.client.WrapKey(ctx, p.keyName, version, azkeys.KeyOperationParameters{
		Algorithm: &p.algorithm,
		Value:     plaintext,
	}, nil)
	if err != nil {
		return nil, "", fmt.Errorf("Azure Key Vault WrapKey: %w", err)
	}
	if len(out.Result) == 0 {
		return nil, "", errors.New("Azure Key Vault returned an empty wrapped DEK")
	}
	keyID := p.GetKeyID()
	if out.KID != nil {
		keyID = string(*out.KID)
		p.setKeyID(keyID)
	}
	wrapped := append([]byte(nil), out.Result...)
	return wrapped, keyID, nil
}

func (p *AzureProvider) UnwrapKey(ctx context.Context, wrapped []byte, keyID string) ([]byte, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if len(wrapped) == 0 {
		return nil, errors.New("wrapped DEK is empty")
	}
	version, err := azureKeyVersion(keyID, p.vaultURL, p.keyName)
	if err != nil {
		return nil, err
	}
	ciphertext := append([]byte(nil), wrapped...)
	defer wipe(ciphertext)
	out, err := p.client.UnwrapKey(ctx, p.keyName, version, azkeys.KeyOperationParameters{
		Algorithm: &p.algorithm,
		Value:     ciphertext,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("Azure Key Vault UnwrapKey: %w", err)
	}
	if len(out.Result) != 32 {
		wipe(out.Result)
		return nil, errors.New("Azure Key Vault returned a DEK with invalid size")
	}
	dek := append([]byte(nil), out.Result...)
	wipe(out.Result)
	return dek, nil
}

func (p *AzureProvider) GetKeyID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.keyID
}

func (p *AzureProvider) currentVersion() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.version
}

func (p *AzureProvider) setKeyID(keyID string) {
	parts := strings.Split(strings.TrimRight(keyID, "/"), "/")
	p.mu.Lock()
	p.keyID = keyID
	if len(parts) >= 2 && parts[len(parts)-2] == p.keyName {
		p.version = parts[len(parts)-1]
	}
	p.mu.Unlock()
}

func (p *AzureProvider) RotateKey(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	out, err := p.client.RotateKey(ctx, p.keyName, nil)
	if err != nil {
		return fmt.Errorf("rotate Azure Key Vault key: %w", err)
	}
	if out.KeyBundle.Key == nil || out.KeyBundle.Key.KID == nil {
		return errors.New("Azure Key Vault rotation returned no key identifier")
	}
	p.setKeyID(string(*out.KeyBundle.Key.KID))
	return nil
}

func (p *AzureProvider) HealthCheck(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if p.currentVersion() == "" {
		return errors.New("Azure Key Vault key version is unavailable")
	}
	_, err := p.client.GetKey(ctx, p.keyName, p.currentVersion(), nil)
	if err != nil {
		return fmt.Errorf("Azure Key Vault health check: %w", err)
	}
	return nil
}

func azureKeyVersion(keyID, vaultURL, keyName string) (string, error) {
	prefix := strings.TrimRight(vaultURL, "/") + "/keys/" + keyName + "/"
	if !strings.HasPrefix(keyID, prefix) {
		return "", errors.New("Azure Key Vault key ID does not match configured vault key")
	}
	version := strings.TrimPrefix(keyID, prefix)
	if version == "" || strings.Contains(version, "/") {
		return "", errors.New("Azure Key Vault key ID has no valid version")
	}
	return version, nil
}
