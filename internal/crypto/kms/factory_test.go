package kms

import (
	"context"
	"path/filepath"
	"testing"
)

func TestProviderFactoryLocalAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kek.json")
	provider, err := NewProvider(context.Background(), Config{
		Provider: "LOCAL",
		Local:    LocalConfig{KeyFile: path},
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.GetKeyID() == "" {
		t.Fatal("factory returned provider without a key ID")
	}
	if _, err := NewProvider(context.Background(), Config{Provider: "local"}); err == nil {
		t.Fatal("accepted local config without key file")
	}
	if _, err := NewProvider(context.Background(), Config{}); err == nil {
		t.Fatal("accepted missing provider")
	}
	if _, err := NewProvider(context.Background(), Config{Provider: "unsupported"}); err == nil {
		t.Fatal("accepted unsupported provider")
	}
}
