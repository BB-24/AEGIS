package kms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	vaultapi "github.com/hashicorp/vault/api"
)

func TestVaultProviderWrapUnwrapRotateAndHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/transit/encrypt/aegis":
			var body struct {
				Plaintext string `json:"plaintext"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			_, _ = w.Write([]byte(`{"data":{"ciphertext":"vault:v1:` + body.Plaintext + `"}}`))
		case "/v1/transit/decrypt/aegis":
			var body struct {
				Ciphertext string `json:"ciphertext"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			_, _ = w.Write([]byte(`{"data":{"plaintext":"` + body.Ciphertext[len("vault:v1:"):] + `"}}`))
		case "/v1/transit/keys/aegis/rotate", "/v1/sys/health":
			if r.URL.Path == "/v1/sys/health" {
				_, _ = w.Write([]byte(`{"initialized":true,"sealed":false}`))
			} else {
				_, _ = w.Write([]byte(`{}`))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	config := vaultapi.DefaultConfig()
	config.Address = server.URL
	client, err := vaultapi.NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewVaultProviderWithClient(client, "transit", "aegis")
	if err != nil {
		t.Fatal(err)
	}
	dek := make([]byte, 32)
	wrapped, keyID, err := provider.WrapKey(context.Background(), dek)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := provider.UnwrapKey(context.Background(), wrapped, keyID)
	if err != nil || string(decoded) != string(dek) {
		t.Fatalf("Vault roundtrip = (%d bytes, %v)", len(decoded), err)
	}
	if err := provider.RotateKey(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := provider.HealthCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestVaultProviderValidationAndErrors(t *testing.T) {
	if _, err := NewVaultProvider(VaultConfig{}); err == nil {
		t.Fatal("accepted missing Vault key name")
	}
	if _, err := NewVaultProviderWithClient(nil, "transit", "key"); err == nil {
		t.Fatal("accepted nil Vault client")
	}
	server := httptest.NewServer(http.NotFoundHandler())
	config := vaultapi.DefaultConfig()
	config.Address = server.URL
	client, err := vaultapi.NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	server.Close()
	if _, err := NewVaultProviderWithClient(client, "", "key"); err == nil {
		t.Fatal("accepted empty transit path")
	}
	provider, err := NewVaultProviderWithClient(client, "transit", "key")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := provider.WrapKey(context.Background(), []byte("short")); err == nil {
		t.Fatal("accepted short DEK")
	}
	if _, _, err := provider.WrapKey(context.Background(), make([]byte, 32)); err == nil {
		t.Fatal("ignored unconfigured Vault client failure")
	}
	if _, err := provider.UnwrapKey(context.Background(), []byte("wrapped"), "wrong"); err == nil {
		t.Fatal("accepted mismatched Vault key ID")
	}
	if err := provider.HealthCheck(context.Background()); err == nil {
		t.Fatal("ignored unconfigured Vault health failure")
	}
	if err := provider.RotateKey(context.Background()); err == nil {
		t.Fatal("ignored unconfigured Vault rotation failure")
	}
}
