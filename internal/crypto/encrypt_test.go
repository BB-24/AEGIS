package crypto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/aegis/aegis/internal/crypto/kms"
)

const testKeyID = "test"

func newTestEncryptor(t testing.TB) (*AESGCM, *kms.LocalProvider) {
	t.Helper()
	provider, err := kms.NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}
	keyManager, err := NewKeyManager(provider)
	if err != nil {
		t.Fatal(err)
	}
	encryptor, err := NewEncryptor(keyManager)
	if err != nil {
		t.Fatal(err)
	}
	return encryptor, provider
}

func TestEncryptChunkRoundTrip(t *testing.T) {
	encryptor, _ := newTestEncryptor(t)
	plaintext := bytes.Repeat([]byte("backup chunk contents\n"), 4096)
	ciphertext, nonce, wrapped, keyID, err := encryptor.EncryptChunk(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if len(nonce) != NonceSize || len(ciphertext) != len(plaintext)+TagSize || len(wrapped) == 0 || keyID == "" {
		t.Fatalf("unexpected envelope sizes: ciphertext=%d nonce=%d wrapped=%d keyID=%q", len(ciphertext), len(nonce), len(wrapped), keyID)
	}
	decoded, err := encryptor.DecryptChunk(ciphertext, nonce, wrapped, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, plaintext) {
		t.Fatal("decrypted chunk differs from plaintext")
	}
}

func TestEncryptChunkUsesUniqueDEKsAndNonces(t *testing.T) {
	encryptor, _ := newTestEncryptor(t)
	deks := make(map[string]struct{}, 1000)
	nonces := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		ciphertext, nonce, wrapped, keyID, err := encryptor.EncryptChunk([]byte(fmt.Sprintf("chunk-%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		dek, err := encryptor.keyManager.UnwrapKey(context.Background(), wrapped, keyID)
		if err != nil {
			t.Fatal(err)
		}
		deks[string(dek)] = struct{}{}
		zeroize(dek)
		nonces[string(nonce)] = struct{}{}
		zeroize(ciphertext)
		zeroize(wrapped)
	}
	if len(deks) != 1000 {
		t.Fatalf("unique DEKs = %d, want 1000", len(deks))
	}
	if len(nonces) != 1000 {
		t.Fatalf("unique nonces = %d, want 1000", len(nonces))
	}
}

func TestDecryptChunkRejectsTampering(t *testing.T) {
	encryptor, _ := newTestEncryptor(t)
	ciphertext, nonce, wrapped, keyID, err := encryptor.EncryptChunk([]byte("authenticated data"))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("ciphertext", func(t *testing.T) {
		tampered := append([]byte(nil), ciphertext...)
		tampered[0] ^= 0x80
		if _, err := encryptor.DecryptChunk(tampered, nonce, wrapped, keyID); err == nil {
			t.Fatal("tampered ciphertext was accepted")
		}
	})
	t.Run("nonce", func(t *testing.T) {
		tampered := append([]byte(nil), nonce...)
		tampered[0] ^= 0x01
		if _, err := encryptor.DecryptChunk(ciphertext, tampered, wrapped, keyID); err == nil {
			t.Fatal("tampered nonce was accepted")
		}
	})
	t.Run("wrapped DEK", func(t *testing.T) {
		tampered := append([]byte(nil), wrapped...)
		tampered[len(tampered)-1] ^= 0x01
		if _, err := encryptor.DecryptChunk(ciphertext, nonce, tampered, keyID); err == nil {
			t.Fatal("tampered wrapped DEK was accepted")
		}
	})
	t.Run("wrong key ID", func(t *testing.T) {
		if _, err := encryptor.DecryptChunk(ciphertext, nonce, wrapped, "missing-key"); err == nil {
			t.Fatal("unknown KEK ID was accepted")
		}
	})
}

func TestDecryptChunkRejectsMalformedInputs(t *testing.T) {
	encryptor, _ := newTestEncryptor(t)
	if _, err := encryptor.DecryptChunk([]byte("short"), make([]byte, NonceSize), []byte("wrapped"), "key"); err == nil {
		t.Fatal("accepted ciphertext shorter than the GCM tag")
	}
	if _, err := encryptor.DecryptChunk(make([]byte, TagSize), []byte{1}, []byte("wrapped"), "key"); err == nil {
		t.Fatal("accepted an invalid nonce length")
	}
	if _, err := NewEncryptor(nil); err == nil {
		t.Fatal("accepted a nil key manager")
	}
	if _, err := NewEncryptorWithRandomSource(nil, bytes.NewReader(nil)); err == nil {
		t.Fatal("accepted a nil key manager with a random source")
	}
	if _, err := NewEncryptorWithRandomSource(encryptor.keyManager, nil); err == nil {
		t.Fatal("accepted a nil random source")
	}
}

func TestEncryptChunkSupportsEmptyPlaintext(t *testing.T) {
	encryptor, _ := newTestEncryptor(t)
	ciphertext, nonce, wrapped, keyID, err := encryptor.EncryptChunk(nil)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := encryptor.DecryptChunk(ciphertext, nonce, wrapped, keyID)
	if err != nil || len(plaintext) != 0 {
		t.Fatalf("empty plaintext roundtrip = (%x, %v)", plaintext, err)
	}
}

func TestEncryptChunkContextAndRandomFailures(t *testing.T) {
	encryptor, _ := newTestEncryptor(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := encryptor.EncryptChunkWithContext(ctx, []byte("data")); !errors.Is(err, context.Canceled) {
		t.Fatalf("EncryptChunkWithContext error = %v, want context.Canceled", err)
	}

	failedRandom, err := NewEncryptorWithRandomSource(encryptor.keyManager, bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := failedRandom.GenerateDEK(); err == nil {
		t.Fatal("GenerateDEK succeeded with an exhausted random source")
	}
	failedNonce, err := NewEncryptorWithRandomSource(encryptor.keyManager, bytes.NewReader(make([]byte, DEKSize)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := failedNonce.EncryptChunkWithContext(context.Background(), []byte("data")); err == nil {
		t.Fatal("encryption succeeded with an exhausted nonce source")
	}
	if _, err := encryptor.DecryptChunkWithContext(ctx, make([]byte, TagSize), make([]byte, NonceSize), []byte("wrapped"), "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("DecryptChunkWithContext error = %v, want context.Canceled", err)
	}
}

func TestDecryptChunkRejectsWrongKEK(t *testing.T) {
	encryptor, _ := newTestEncryptor(t)
	ciphertext, nonce, wrapped, keyID, err := encryptor.EncryptChunk([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	otherProvider, err := kms.NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}
	otherManager, err := NewKeyManager(otherProvider)
	if err != nil {
		t.Fatal(err)
	}
	otherEncryptor, err := NewEncryptor(otherManager)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := otherEncryptor.DecryptChunk(ciphertext, nonce, wrapped, keyID); err == nil {
		t.Fatal("decryption succeeded with a different KEK")
	}
}

func TestEncryptChunkSurvivesKEKRotation(t *testing.T) {
	encryptor, provider := newTestEncryptor(t)
	plaintext := []byte("still decryptable after key rotation")
	ciphertext, nonce, wrapped, keyID, err := encryptor.EncryptChunk(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if err := encryptor.keyManager.RotateKEK(context.Background()); err != nil {
		t.Fatal(err)
	}
	decoded, err := encryptor.DecryptChunk(ciphertext, nonce, wrapped, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, plaintext) {
		t.Fatal("rotation changed plaintext")
	}
	if _, _, err := provider.Encrypt(context.Background(), []byte("new key")); err != nil {
		t.Fatal(err)
	}
}

func TestEncryptorHandlesKeyManagerFailures(t *testing.T) {
	tests := []struct {
		name        string
		manager     testKeyManager
		encryptFail bool
	}{
		{name: "wrap error", manager: testKeyManager{wrapErr: errors.New("wrap failed")}, encryptFail: true},
		{name: "empty wrapped key", manager: testKeyManager{emptyWrapped: true}, encryptFail: true},
		{name: "unwrap error", manager: testKeyManager{unwrapErr: errors.New("unwrap failed")}},
		{name: "invalid unwrapped key", manager: testKeyManager{unwrapKey: []byte("short")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encryptor, err := NewEncryptor(test.manager)
			if err != nil {
				t.Fatal(err)
			}
			if test.encryptFail {
				if _, _, _, _, err := encryptor.EncryptChunk([]byte("data")); err == nil {
					t.Fatal("encryption succeeded with invalid key manager")
				}
				return
			}
			if _, err := encryptor.DecryptChunk(make([]byte, TagSize), make([]byte, NonceSize), []byte("wrapped"), "test"); err == nil {
				t.Fatal("decryption succeeded with invalid key manager")
			}
		})
	}
}

type testKeyManager struct {
	wrapErr      error
	unwrapErr    error
	unwrapKey    []byte
	emptyWrapped bool
}

func (m testKeyManager) WrapKey(_ context.Context, dek []byte) ([]byte, string, error) {
	if m.wrapErr != nil {
		return nil, "", m.wrapErr
	}
	if len(dek) == 0 {
		return nil, "", errors.New("empty input")
	}
	if m.emptyWrapped {
		return nil, testKeyID, nil
	}
	return []byte("wrapped"), testKeyID, nil
}

func (m testKeyManager) UnwrapKey(context.Context, []byte, string) ([]byte, error) {
	if m.unwrapErr != nil {
		return nil, m.unwrapErr
	}
	if m.unwrapKey != nil {
		return append([]byte(nil), m.unwrapKey...), nil
	}
	return nil, errors.New("no test key configured")
}

func (testKeyManager) RotateKEK(context.Context) error { return nil }

func TestAESGCMConcurrentRoundTrips(t *testing.T) {
	encryptor, _ := newTestEncryptor(t)
	const workers = 32
	var group sync.WaitGroup
	errs := make(chan error, workers)
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			plaintext := []byte(fmt.Sprintf("parallel chunk %d", worker))
			ciphertext, nonce, wrapped, keyID, err := encryptor.EncryptChunk(plaintext)
			if err != nil {
				errs <- err
				return
			}
			decoded, err := encryptor.DecryptChunk(ciphertext, nonce, wrapped, keyID)
			if err != nil {
				errs <- err
				return
			}
			if !bytes.Equal(decoded, plaintext) {
				errs <- fmt.Errorf("worker %d got mismatched plaintext", worker)
			}
		}(worker)
	}
	group.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func FuzzAESGCMRoundTrip(f *testing.F) {
	encryptor, _ := newTestEncryptor(f)
	f.Add([]byte("seed chunk"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, plaintext []byte) {
		ciphertext, nonce, wrapped, keyID, err := encryptor.EncryptChunk(plaintext)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := encryptor.DecryptChunk(ciphertext, nonce, wrapped, keyID)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(decoded, plaintext) {
			t.Fatal("roundtrip changed plaintext")
		}
	})
}
