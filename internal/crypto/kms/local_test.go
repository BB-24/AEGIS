package kms

import (
	"bytes"
	"context"
	"errors"
	"os"
	"runtime"
	"testing"
)

func TestLocalProviderEncryptDecrypt(t *testing.T) {
	provider, err := NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("wrapped data encryption key")
	ciphertext, keyID, err := provider.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := provider.Decrypt(context.Background(), ciphertext, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, plaintext) {
		t.Fatal("local provider roundtrip changed plaintext")
	}
}

func TestLocalProviderGenerateDataKey(t *testing.T) {
	provider, err := NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}
	plaintext, ciphertext, keyID, err := provider.GenerateDataKey(context.Background(), AES256KeySpec)
	if err != nil {
		t.Fatal(err)
	}
	if len(plaintext) != 32 || len(ciphertext) == 0 || keyID == "" {
		t.Fatalf("invalid generated key output: plain=%d wrapped=%d id=%q", len(plaintext), len(ciphertext), keyID)
	}
	decoded, err := provider.Decrypt(context.Background(), ciphertext, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, plaintext) {
		t.Fatal("generated plaintext key does not match unwrapped key")
	}
	if _, _, _, err := provider.GenerateDataKey(context.Background(), "AES_128"); err == nil {
		t.Fatal("accepted unsupported key spec")
	}
}

func TestLocalProviderRotationPreservesOldKeys(t *testing.T) {
	provider, err := NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}
	oldCiphertext, oldID, err := provider.Encrypt(context.Background(), []byte("old wrapped key"))
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.RotateKEK(context.Background()); err != nil {
		t.Fatal(err)
	}
	newCiphertext, newID, err := provider.Encrypt(context.Background(), []byte("new wrapped key"))
	if err != nil {
		t.Fatal(err)
	}
	if oldID == newID {
		t.Fatal("KEK rotation did not change the active key ID")
	}
	for _, test := range []struct {
		ciphertext []byte
		keyID      string
		want       string
	}{
		{oldCiphertext, oldID, "old wrapped key"},
		{newCiphertext, newID, "new wrapped key"},
	} {
		decoded, err := provider.Decrypt(context.Background(), test.ciphertext, test.keyID)
		if err != nil {
			t.Fatal(err)
		}
		if string(decoded) != test.want {
			t.Fatalf("decoded = %q, want %q", decoded, test.want)
		}
	}
}

func TestLocalProviderValidation(t *testing.T) {
	if _, err := NewLocalProviderWithKey([]byte("short"), "key"); err == nil {
		t.Fatal("accepted short KEK")
	}
	if _, err := NewLocalProviderWithKey(make([]byte, 32), ""); err == nil {
		t.Fatal("accepted empty key ID")
	}
	provider, err := NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}
	var nilContext context.Context
	if _, _, err := provider.Encrypt(nilContext, []byte("data")); err == nil {
		t.Fatal("accepted nil context")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := provider.Encrypt(ctx, []byte("data")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Encrypt error = %v, want context.Canceled", err)
	}
	if _, err := provider.Decrypt(context.Background(), []byte("short"), "missing"); err == nil {
		t.Fatal("accepted unknown KEK")
	}
	if _, err := provider.Decrypt(context.Background(), []byte("short"), "local-kek-1"); err == nil {
		t.Fatal("accepted short wrapped data")
	}
}

func TestLocalProviderRejectsTampering(t *testing.T) {
	provider, err := NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, keyID, err := provider.Encrypt(context.Background(), []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err := provider.Decrypt(context.Background(), ciphertext, keyID); err == nil {
		t.Fatal("accepted tampered wrapped data")
	}
}

func TestLocalProviderRandomSourceFailures(t *testing.T) {
	provider, err := NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}
	provider.random = bytes.NewReader(nil)
	if _, _, err := provider.Encrypt(context.Background(), []byte("data")); err == nil {
		t.Fatal("Encrypt succeeded with an exhausted nonce source")
	}
	if _, _, _, err := provider.GenerateDataKey(context.Background(), AES256KeySpec); err == nil {
		t.Fatal("GenerateDataKey succeeded with an exhausted random source")
	}
	if err := provider.RotateKEK(context.Background()); err == nil {
		t.Fatal("RotateKEK succeeded with an exhausted random source")
	}
}

func TestLocalProviderRejectsInvalidInternalCipherKeys(t *testing.T) {
	provider, err := NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.seal([]byte("bad"), []byte("plaintext")); err == nil {
		t.Fatal("seal accepted an invalid AES key")
	}
	if _, err := provider.open([]byte("bad"), []byte("wrapped")); err == nil {
		t.Fatal("open accepted an invalid AES key")
	}
}

func TestLocalProviderMissingActiveKeyAndRotationCollision(t *testing.T) {
	provider := &LocalProvider{
		random: bytes.NewReader(make([]byte, 32+12+32)),
		keys:   map[string][]byte{},
	}
	if _, _, err := provider.Encrypt(context.Background(), []byte("data")); err == nil {
		t.Fatal("Encrypt succeeded without an active KEK")
	}
	if _, _, _, err := provider.GenerateDataKey(context.Background(), AES256KeySpec); err == nil {
		t.Fatal("GenerateDataKey succeeded without an active KEK")
	}

	provider.keys[localKeyPrefix+"1"] = make([]byte, 32)
	provider.activeID = localKeyPrefix + "1"
	provider.nextID = 1
	if err := provider.RotateKEK(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.activeID == localKeyPrefix+"1" {
		t.Fatal("rotation failed to skip an existing key ID")
	}
}

func TestLocalProviderDataKeyAndRotationContextCancellation(t *testing.T) {
	provider, err := NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, err := provider.GenerateDataKey(ctx, AES256KeySpec); !errors.Is(err, context.Canceled) {
		t.Fatalf("GenerateDataKey error = %v, want context.Canceled", err)
	}
	if err := provider.RotateKEK(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("RotateKEK error = %v, want context.Canceled", err)
	}
}

func TestFileBackedLocalProviderPersistsKeyVersions(t *testing.T) {
	path := t.TempDir() + "/local-kek.json"
	provider, err := OpenLocalProvider(path)
	if err != nil {
		t.Fatal(err)
	}
	oldKeyID := provider.GetKeyID()
	oldWrapped, keyID, err := provider.WrapKey(context.Background(), []byte("old DEK"))
	if err != nil || keyID != oldKeyID {
		t.Fatalf("WrapKey = (%q, %v)", keyID, err)
	}
	if err := provider.RotateKey(context.Background()); err != nil {
		t.Fatal(err)
	}
	newKeyID := provider.GetKeyID()
	if newKeyID == oldKeyID {
		t.Fatal("rotation did not change active key ID")
	}
	reopened, err := OpenLocalProvider(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.GetKeyID() != newKeyID {
		t.Fatalf("reopened active key ID = %q, want %q", reopened.GetKeyID(), newKeyID)
	}
	decoded, err := reopened.UnwrapKey(context.Background(), oldWrapped, oldKeyID)
	if err != nil || string(decoded) != "old DEK" {
		t.Fatalf("old key version unwrap = (%q, %v)", decoded, err)
	}
	if err := reopened.HealthCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("key file permissions are too broad: %o", info.Mode().Perm())
	}
}
