// Package kms contains KMS provider implementations.
package kms

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"sync"
)

const (
	AES256KeySpec  = "AES_256"
	localKeyPrefix = "local-kek-"
)

// LocalProvider is a process-local development/test KMS. It is not a
// production key-management service and loses all keys when the process exits.
type LocalProvider struct {
	mu       sync.RWMutex
	random   io.Reader
	randomMu sync.Mutex
	keys     map[string][]byte
	activeID string
	nextID   uint64
}

var _ interface {
	Encrypt(context.Context, []byte) ([]byte, string, error)
	Decrypt(context.Context, []byte, string) ([]byte, error)
	GenerateDataKey(context.Context, string) ([]byte, []byte, string, error)
	RotateKEK(context.Context) error
} = (*LocalProvider)(nil)

// NewLocalProvider creates an ephemeral local provider with a random AES-256 KEK.
func NewLocalProvider() (*LocalProvider, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate local KEK: %w", err)
	}
	provider, err := NewLocalProviderWithKey(key, localKeyPrefix+"1")
	wipe(key)
	if err != nil {
		return nil, err
	}
	provider.random = rand.Reader
	provider.nextID = 2
	return provider, nil
}

// NewLocalProviderWithKey creates an ephemeral provider using a supplied key.
// It is primarily intended for deterministic tests; callers must provide a
// securely generated 32-byte key and protect the source material.
func NewLocalProviderWithKey(key []byte, keyID string) (*LocalProvider, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("local KEK must be 32 bytes, got %d", len(key))
	}
	if keyID == "" {
		return nil, errors.New("local KEK ID is empty")
	}
	return &LocalProvider{
		random:   rand.Reader,
		keys:     map[string][]byte{keyID: append([]byte(nil), key...)},
		activeID: keyID,
		nextID:   1,
	}, nil
}

// Encrypt wraps plaintext with the active local KEK using AES-GCM.
func (p *LocalProvider) Encrypt(ctx context.Context, plaintext []byte) ([]byte, string, error) {
	if err := validateContext(ctx); err != nil {
		return nil, "", err
	}
	p.mu.RLock()
	keyID := p.activeID
	key := append([]byte(nil), p.keys[keyID]...)
	p.mu.RUnlock()
	defer wipe(key)

	wrapped, err := p.seal(key, plaintext)
	if err != nil {
		return nil, "", fmt.Errorf("local KMS encrypt: %w", err)
	}
	return wrapped, keyID, nil
}

// Decrypt unwraps ciphertext with the specified key version. Historical KEKs
// remain available after RotateKEK so older chunk DEKs continue to decrypt.
func (p *LocalProvider) Decrypt(ctx context.Context, ciphertext []byte, keyID string) ([]byte, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	p.mu.RLock()
	key := append([]byte(nil), p.keys[keyID]...)
	p.mu.RUnlock()
	if len(key) != 32 {
		return nil, fmt.Errorf("local KMS key ID %q not found", keyID)
	}
	defer wipe(key)
	plaintext, err := p.open(key, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("local KMS decrypt: %w", err)
	}
	return plaintext, nil
}

// GenerateDataKey returns a random AES-256 key and its wrapped form.
func (p *LocalProvider) GenerateDataKey(ctx context.Context, keySpec string) ([]byte, []byte, string, error) {
	if err := validateContext(ctx); err != nil {
		return nil, nil, "", err
	}
	if keySpec != AES256KeySpec {
		return nil, nil, "", fmt.Errorf("unsupported local data key spec %q", keySpec)
	}
	plaintext := make([]byte, 32)
	p.randomMu.Lock()
	_, err := io.ReadFull(p.random, plaintext)
	p.randomMu.Unlock()
	if err != nil {
		wipe(plaintext)
		return nil, nil, "", fmt.Errorf("generate local data key: %w", err)
	}
	wrapped, keyID, err := p.Encrypt(ctx, plaintext)
	if err != nil {
		wipe(plaintext)
		return nil, nil, "", err
	}
	return plaintext, wrapped, keyID, nil
}

// RotateKEK creates a new active KEK while retaining old versions for unwrap.
func (p *LocalProvider) RotateKEK(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	key := make([]byte, 32)
	p.randomMu.Lock()
	_, err := io.ReadFull(p.random, key)
	p.randomMu.Unlock()
	if err != nil {
		wipe(key)
		return fmt.Errorf("generate rotated local KEK: %w", err)
	}

	p.mu.Lock()
	keyID := localKeyPrefix + strconv.FormatUint(p.nextID, 10)
	for {
		if _, exists := p.keys[keyID]; !exists {
			break
		}
		p.nextID++
		keyID = localKeyPrefix + strconv.FormatUint(p.nextID, 10)
	}
	p.nextID++
	p.keys[keyID] = append([]byte(nil), key...)
	p.activeID = keyID
	p.mu.Unlock()
	wipe(key)
	return nil
}

func (p *LocalProvider) seal(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	p.randomMu.Lock()
	_, err = io.ReadFull(p.random, nonce)
	p.randomMu.Unlock()
	if err != nil {
		wipe(nonce)
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (p *LocalProvider) open(key, wrapped []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(wrapped) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("wrapped key is too short")
	}
	nonce := wrapped[:gcm.NonceSize()]
	return gcm.Open(nil, nonce, wrapped[gcm.NonceSize():], nil)
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("operation canceled: %w", err)
	}
	return nil
}

func wipe(data []byte) {
	for i := range data {
		data[i] = 0
	}
	runtime.KeepAlive(data)
}
