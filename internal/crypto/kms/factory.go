package kms

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type LocalConfig struct {
	KeyFile string
}

// Config selects a provider and supplies its provider-specific settings.
type Config struct {
	Provider string
	AWS      AWSConfig
	Vault    VaultConfig
	Azure    AzureConfig
	GCP      GCPConfig
	Local    LocalConfig
}

// NewProvider constructs a provider from configuration. Cloud providers use
// their SDK's default credential chain rather than accepting key material here.
func NewProvider(ctx context.Context, config Config) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(config.Provider)) {
	case "aws":
		return NewAWSProvider(ctx, config.AWS)
	case "vault":
		return NewVaultProvider(config.Vault)
	case "azure":
		return NewAzureProvider(ctx, config.Azure)
	case "gcp":
		return NewGCPProvider(ctx, config.GCP)
	case "local":
		if strings.TrimSpace(config.Local.KeyFile) == "" {
			return nil, errors.New("local KMS key_file is required")
		}
		return OpenLocalProvider(config.Local.KeyFile)
	case "":
		return nil, errors.New("KMS provider is required")
	default:
		return nil, fmt.Errorf("unsupported KMS provider %q", config.Provider)
	}
}
