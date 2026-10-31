package kms

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

type VaultConfig struct {
	Address     string
	Token       string
	TransitPath string
	KeyName     string
}

// VaultProvider wraps DEKs through the Vault Transit secrets engine.
type VaultProvider struct {
	client      *vault.Client
	transitPath string
	keyName     string
}

var _ Provider = (*VaultProvider)(nil)

func NewVaultProvider(config VaultConfig) (*VaultProvider, error) {
	if strings.TrimSpace(config.KeyName) == "" {
		return nil, errors.New("Vault Transit key name is required")
	}
	if config.TransitPath == "" {
		config.TransitPath = "transit"
	}
	config.TransitPath = strings.Trim(config.TransitPath, "/")
	if config.TransitPath == "" {
		return nil, errors.New("Vault Transit path is empty")
	}
	vaultConfig := vault.DefaultConfig()
	if config.Address != "" {
		vaultConfig.Address = config.Address
	}
	client, err := vault.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("create Vault client: %w", err)
	}
	if config.Token != "" {
		client.SetToken(config.Token)
	}
	return NewVaultProviderWithClient(client, config.TransitPath, config.KeyName)
}

func NewVaultProviderWithClient(client *vault.Client, transitPath, keyName string) (*VaultProvider, error) {
	if client == nil {
		return nil, errors.New("Vault client is nil")
	}
	if strings.Trim(transitPath, "/") == "" || strings.TrimSpace(keyName) == "" {
		return nil, errors.New("Vault Transit path and key name are required")
	}
	return &VaultProvider{
		client:      client,
		transitPath: strings.Trim(transitPath, "/"),
		keyName:     keyName,
	}, nil
}

func (p *VaultProvider) WrapKey(ctx context.Context, dek []byte) ([]byte, string, error) {
	if err := validateContext(ctx); err != nil {
		return nil, "", err
	}
	if len(dek) != 32 {
		return nil, "", fmt.Errorf("DEK must be 32 bytes, got %d", len(dek))
	}
	secret, err := p.client.Logical().WriteWithContext(ctx, p.transitPath+"/encrypt/"+p.keyName, map[string]interface{}{
		"plaintext": base64.StdEncoding.EncodeToString(dek),
	})
	if err != nil {
		return nil, "", fmt.Errorf("Vault Transit encrypt: %w", err)
	}
	ciphertext, ok := secretString(secret, "ciphertext")
	if !ok {
		return nil, "", errors.New("Vault Transit returned no ciphertext")
	}
	return []byte(ciphertext), p.GetKeyID(), nil
}

func (p *VaultProvider) UnwrapKey(ctx context.Context, wrapped []byte, keyID string) ([]byte, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if len(wrapped) == 0 || keyID != p.GetKeyID() {
		return nil, errors.New("wrapped DEK or matching Vault key ID is required")
	}
	secret, err := p.client.Logical().WriteWithContext(ctx, p.transitPath+"/decrypt/"+p.keyName, map[string]interface{}{
		"ciphertext": string(wrapped),
	})
	if err != nil {
		return nil, fmt.Errorf("Vault Transit decrypt: %w", err)
	}
	encoded, ok := secretString(secret, "plaintext")
	if !ok {
		return nil, errors.New("Vault Transit returned no plaintext")
	}
	dek, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode Vault Transit plaintext: %w", err)
	}
	if len(dek) != 32 {
		wipe(dek)
		return nil, errors.New("Vault Transit returned a DEK with invalid size")
	}
	return dek, nil
}

func secretString(secret *vault.Secret, key string) (string, bool) {
	if secret == nil || secret.Data == nil {
		return "", false
	}
	value, ok := secret.Data[key].(string)
	return value, ok
}

func (p *VaultProvider) GetKeyID() string {
	return fmt.Sprintf("vault:%s:%s", p.transitPath, p.keyName)
}

func (p *VaultProvider) RotateKey(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	_, err := p.client.Logical().WriteWithContext(ctx, p.transitPath+"/keys/"+p.keyName+"/rotate", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("rotate Vault Transit key: %w", err)
	}
	return nil
}

func (p *VaultProvider) HealthCheck(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	health, err := p.client.Sys().HealthWithContext(ctx)
	if err != nil {
		return fmt.Errorf("Vault health check: %w", err)
	}
	if health == nil || !health.Initialized || health.Sealed {
		return errors.New("Vault is not initialized or is sealed")
	}
	return nil
}
