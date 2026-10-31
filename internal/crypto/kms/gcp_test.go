package kms

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/googleapis/gax-go/v2"
)

type fakeGCPKMS struct {
	dek        []byte
	encryptErr error
	decryptErr error
	getErr     error
	createErr  error
	updateErr  error
}

func (f *fakeGCPKMS) Encrypt(_ context.Context, req *kmspb.EncryptRequest, _ ...gax.CallOption) (*kmspb.EncryptResponse, error) {
	if f.encryptErr != nil {
		return nil, f.encryptErr
	}
	f.dek = append([]byte(nil), req.Plaintext...)
	return &kmspb.EncryptResponse{Ciphertext: []byte("wrapped")}, nil
}

func (f *fakeGCPKMS) Decrypt(context.Context, *kmspb.DecryptRequest, ...gax.CallOption) (*kmspb.DecryptResponse, error) {
	if f.decryptErr != nil {
		return nil, f.decryptErr
	}
	return &kmspb.DecryptResponse{Plaintext: append([]byte(nil), f.dek...)}, nil
}

func (f *fakeGCPKMS) GetCryptoKey(context.Context, *kmspb.GetCryptoKeyRequest, ...gax.CallOption) (*kmspb.CryptoKey, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &kmspb.CryptoKey{Primary: &kmspb.CryptoKeyVersion{}}, nil
}

func (f *fakeGCPKMS) CreateCryptoKeyVersion(context.Context, *kmspb.CreateCryptoKeyVersionRequest, ...gax.CallOption) (*kmspb.CryptoKeyVersion, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &kmspb.CryptoKeyVersion{Name: "projects/p/locations/l/keyRings/r/cryptoKeys/k/cryptoKeyVersions/2"}, nil
}

func (f *fakeGCPKMS) UpdateCryptoKeyPrimaryVersion(context.Context, *kmspb.UpdateCryptoKeyPrimaryVersionRequest, ...gax.CallOption) (*kmspb.CryptoKey, error) {
	return &kmspb.CryptoKey{}, f.updateErr
}

func TestGCPProviderWrapUnwrapHealthAndRotate(t *testing.T) {
	const keyName = "projects/p/locations/l/keyRings/r/cryptoKeys/k"
	client := &fakeGCPKMS{}
	provider, err := NewGCPProviderWithClient(client, keyName)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, keyID, err := provider.WrapKey(context.Background(), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	if string(wrapped) != "wrapped" || keyID != keyName {
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
	if err := provider.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestGCPProviderPropagatesErrorsAndValidates(t *testing.T) {
	client := &fakeGCPKMS{encryptErr: errors.New("offline")}
	provider, err := NewGCPProviderWithClient(client, "projects/p/locations/l/keyRings/r/cryptoKeys/k")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := provider.WrapKey(context.Background(), make([]byte, 32)); err == nil {
		t.Fatal("ignored Google KMS Encrypt error")
	}
	if _, _, err := provider.WrapKey(context.Background(), []byte("short")); err == nil {
		t.Fatal("accepted short DEK")
	}
	client.encryptErr = nil
	client.decryptErr = errors.New("offline")
	if _, err := provider.UnwrapKey(context.Background(), []byte("wrapped"), provider.GetKeyID()); err == nil {
		t.Fatal("ignored Google KMS Decrypt error")
	}
	client.decryptErr = nil
	if _, err := provider.UnwrapKey(context.Background(), []byte("wrapped"), "other"); err == nil {
		t.Fatal("accepted mismatched key name")
	}
	client.getErr = errors.New("offline")
	if err := provider.HealthCheck(context.Background()); err == nil {
		t.Fatal("ignored Google KMS health error")
	}
	client.createErr = errors.New("offline")
	if err := provider.RotateKey(context.Background()); err == nil {
		t.Fatal("ignored Google KMS version creation error")
	}
	client.createErr = nil
	client.updateErr = errors.New("denied")
	if err := provider.RotateKey(context.Background()); err == nil {
		t.Fatal("ignored Google KMS primary version error")
	}
	if _, err := NewGCPProviderWithClient(nil, "key"); err == nil {
		t.Fatal("accepted nil Google KMS client")
	}
}
