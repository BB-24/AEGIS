// Package kms contains KMS provider implementations.
package kms

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	filePath string
}

var _ interface {
	Encrypt(context.Context, []byte) ([]byte, string, error)
	Decrypt(context.Context, []byte, string) ([]byte, error)
	GenerateDataKey(context.Context, string) ([]byte, []byte, string, error)
	RotateKEK(context.Context) error
} = (*LocalProvider)(nil)

var _ Provider = (*LocalProvider)(nil)

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

// OpenLocalProvider loads or creates a file-backed development KEK. The file
// contains plaintext key material and must be protected by the host.
func OpenLocalProvider(path string) (*LocalProvider, error) {
	if path == "" {
		return nil, errors.New("local KMS key file path is empty")
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		provider, createErr := NewLocalProvider()
		if createErr != nil {
			return nil, createErr
		}
		provider.filePath = path
		provider.mu.Lock()
		persistErr := provider.persistLocked()
		provider.mu.Unlock()
		if persistErr != nil {
			provider.destroyKeys()
			return nil, persistErr
		}
		return provider, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read local KMS key file: %w", err)
	}
	defer wipe(data)
	var state localKeyFileState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse local KMS key file: %w", err)
	}
	if state.ActiveID == "" || len(state.Keys[state.ActiveID]) != 32 || state.NextID == 0 {
		return nil, errors.New("local KMS key file has invalid state")
	}
	provider := &LocalProvider{
		random:   rand.Reader,
		keys:     state.Keys,
		activeID: state.ActiveID,
		nextID:   state.NextID,
		filePath: path,
	}
	if err := os.Chmod(path, 0o600); err != nil {
		provider.destroyKeys()
		return nil, fmt.Errorf("restrict local KMS key file permissions: %w", err)
	}
	return provider, nil
}

type localKeyFileState struct {
	Keys     map[string][]byte `json:"keys"`
	ActiveID string            `json:"active_id"`
	NextID   uint64            `json:"next_id"`
}

func (p *LocalProvider) persistLocked() error {
	if p.filePath == "" {
		return nil
	}
	encoded, err := json.Marshal(localKeyFileState{
		Keys: p.keys, ActiveID: p.activeID, NextID: p.nextID,
	})
	if err != nil {
		return fmt.Errorf("encode local KMS key file: %w", err)
	}
	defer wipe(encoded)
	dir := filepath.Dir(p.filePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create local KMS key directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".aegis-kek-*")
	if err != nil {
		return fmt.Errorf("create local KMS key file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("restrict temporary KMS key file permissions: %w", err)
	}
	if _, err := temp.Write(encoded); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write local KMS key file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync local KMS key file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close local KMS key file: %w", err)
	}
	if err := os.Rename(tempPath, p.filePath); err != nil {
		if removeErr := os.Remove(p.filePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("replace local KMS key file: %w", err)
		}
		if retryErr := os.Rename(tempPath, p.filePath); retryErr != nil {
			return fmt.Errorf("replace local KMS key file: %w", retryErr)
		}
	}
	return nil
}

func (p *LocalProvider) destroyKeys() {
	p.mu.Lock()
	for id, key := range p.keys {
		wipe(key)
		delete(p.keys, id)
	}
	p.mu.Unlock()
}

// Encrypt wraps plaintext with the active local KEK using AES-GCM.
func (p *LocalProvider) WrapKey(ctx context.Context, plaintext []byte) ([]byte, string, error) {
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

// Encrypt is a compatibility alias for WrapKey.
func (p *LocalProvider) Encrypt(ctx context.Context, plaintext []byte) ([]byte, string, error) {
	return p.WrapKey(ctx, plaintext)
}

// Decrypt unwraps ciphertext with the specified key version. Historical KEKs
// remain available after RotateKEK so older chunk DEKs continue to decrypt.
func (p *LocalProvider) UnwrapKey(ctx context.Context, ciphertext []byte, keyID string) ([]byte, error) {
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

// Decrypt is a compatibility alias for UnwrapKey.
func (p *LocalProvider) Decrypt(ctx context.Context, ciphertext []byte, keyID string) ([]byte, error) {
	return p.UnwrapKey(ctx, ciphertext, keyID)
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
	previousID := p.activeID
	p.activeID = keyID
	if err := p.persistLocked(); err != nil {
		wipe(p.keys[keyID])
		delete(p.keys, keyID)
		p.activeID = previousID
		p.nextID--
		p.mu.Unlock()
		wipe(key)
		return err
	}
	p.mu.Unlock()
	wipe(key)
	return nil
}

// RotateKey satisfies Provider using the existing local rotation behavior.
func (p *LocalProvider) RotateKey(ctx context.Context) error { return p.RotateKEK(ctx) }

// GetKeyID returns the current active KEK version identifier.
func (p *LocalProvider) GetKeyID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.activeID
}

// HealthCheck validates context and verifies the active KEK is available.
func (p *LocalProvider) HealthCheck(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.keys[p.activeID]) != 32 {
		return errors.New("local KMS active KEK is unavailable")
	}
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
