package kms

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
)

type fakeAzureKeys struct {
	dek       []byte
	wrapErr   error
	unwrapErr error
	getErr    error
	rotateErr error
}

func (f *fakeAzureKeys) GetKey(context.Context, string, string, *azkeys.GetKeyOptions) (azkeys.GetKeyResponse, error) {
	if f.getErr != nil {
		return azkeys.GetKeyResponse{}, f.getErr
	}
	return azkeys.GetKeyResponse{KeyBundle: azkeys.KeyBundle{Key: &azkeys.JSONWebKey{KID: (*azkeys.ID)(stringPointer("https://vault/keys/aegis/v1"))}}}, nil
}

func (f *fakeAzureKeys) WrapKey(_ context.Context, _, _ string, params azkeys.KeyOperationParameters, _ *azkeys.WrapKeyOptions) (azkeys.WrapKeyResponse, error) {
	if f.wrapErr != nil {
		return azkeys.WrapKeyResponse{}, f.wrapErr
	}
	f.dek = append([]byte(nil), params.Value...)
	return azkeys.WrapKeyResponse{KeyOperationResult: azkeys.KeyOperationResult{Result: []byte("wrapped"), KID: (*azkeys.ID)(stringPointer("https://vault/keys/aegis/v1"))}}, nil
}

func (f *fakeAzureKeys) UnwrapKey(_ context.Context, _, _ string, _ azkeys.KeyOperationParameters, _ *azkeys.UnwrapKeyOptions) (azkeys.UnwrapKeyResponse, error) {
	if f.unwrapErr != nil {
		return azkeys.UnwrapKeyResponse{}, f.unwrapErr
	}
	return azkeys.UnwrapKeyResponse{KeyOperationResult: azkeys.KeyOperationResult{Result: append([]byte(nil), f.dek...)}}, nil
}

func (f *fakeAzureKeys) RotateKey(context.Context, string, *azkeys.RotateKeyOptions) (azkeys.RotateKeyResponse, error) {
	if f.rotateErr != nil {
		return azkeys.RotateKeyResponse{}, f.rotateErr
	}
	return azkeys.RotateKeyResponse{KeyBundle: azkeys.KeyBundle{Key: &azkeys.JSONWebKey{KID: (*azkeys.ID)(stringPointer("https://vault/keys/aegis/v2"))}}}, nil
}

func stringPointer(value string) *string { return &value }

func TestAzureProviderWrapUnwrapAndRotate(t *testing.T) {
	client := &fakeAzureKeys{}
	provider, err := NewAzureProviderWithClient(client, AzureConfig{
		VaultURL: "https://vault", KeyName: "aegis", KeyVersion: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	wrapped, keyID, err := provider.WrapKey(context.Background(), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	if string(wrapped) != "wrapped" || keyID != "https://vault/keys/aegis/v1" {
		t.Fatalf("wrap result = (%q, %q)", wrapped, keyID)
	}
	decoded, err := provider.UnwrapKey(context.Background(), wrapped, keyID)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("UnwrapKey = (%d bytes, %v)", len(decoded), err)
	}
	if err := provider.HealthCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := provider.RotateKey(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.GetKeyID() != "https://vault/keys/aegis/v2" {
		t.Fatalf("key ID after rotation = %q", provider.GetKeyID())
	}
}

func TestAzureProviderRejectsMismatchedKeyAndPropagatesErrors(t *testing.T) {
	client := &fakeAzureKeys{}
	provider, err := NewAzureProviderWithClient(client, AzureConfig{VaultURL: "https://vault", KeyName: "aegis", KeyVersion: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UnwrapKey(context.Background(), []byte("wrapped"), "https://other/keys/aegis/v1"); err == nil {
		t.Fatal("accepted a mismatched key ID")
	}
	client.wrapErr = errors.New("offline")
	if _, _, err := provider.WrapKey(context.Background(), make([]byte, 32)); err == nil {
		t.Fatal("ignored Azure WrapKey error")
	}
	client.wrapErr = nil
	client.unwrapErr = errors.New("offline")
	if _, err := provider.UnwrapKey(context.Background(), []byte("wrapped"), provider.GetKeyID()); err == nil {
		t.Fatal("ignored Azure UnwrapKey error")
	}
	client.unwrapErr = nil
	client.getErr = errors.New("offline")
	if err := provider.HealthCheck(context.Background()); err == nil {
		t.Fatal("ignored Azure health error")
	}
	client.rotateErr = errors.New("denied")
	if err := provider.RotateKey(context.Background()); err == nil {
		t.Fatal("ignored Azure rotation error")
	}
	if _, err := NewAzureProviderWithClient(nil, AzureConfig{}); err == nil {
		t.Fatal("accepted nil Azure client")
	}
}
