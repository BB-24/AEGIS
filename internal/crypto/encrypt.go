package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
)

const (
	// DEKSize is the size in bytes of an AES-256 data encryption key.
	DEKSize = 32
	// NonceSize is the 96-bit nonce size recommended for GCM.
	NonceSize = 12
	// TagSize is the 128-bit GCM authentication tag size.
	TagSize = 16
)

// Encryptor encrypts and decrypts individual chunks with envelope encryption.
type Encryptor interface {
	EncryptChunk(plaintext []byte) (ciphertext, nonce, wrappedDEK []byte, keyID string, err error)
	DecryptChunk(ciphertext, nonce, wrappedDEK []byte, keyID string) ([]byte, error)
	GenerateDEK() ([]byte, error)
}

// EncryptedChunk contains ciphertext and the information needed to unwrap its DEK.
type EncryptedChunk struct {
	Ciphertext []byte
	Nonce      []byte
	WrappedDEK []byte
	KeyID      string
}

// AESGCM implements per-chunk AES-256-GCM and KMS envelope encryption.
type AESGCM struct {
	keyManager KeyManager
	random     io.Reader
	randomMu   sync.Mutex
}

var _ Encryptor = (*AESGCM)(nil)

// NewEncryptor creates an AES-256-GCM encryptor using crypto/rand.
func NewEncryptor(keyManager KeyManager) (*AESGCM, error) {
	if keyManager == nil {
		return nil, errors.New("key manager is nil")
	}
	return &AESGCM{keyManager: keyManager, random: rand.Reader}, nil
}

// NewEncryptorWithRandomSource allows a secure random source to be supplied.
// The reader is serialized; production callers should use NewEncryptor.
func NewEncryptorWithRandomSource(keyManager KeyManager, random io.Reader) (*AESGCM, error) {
	if keyManager == nil {
		return nil, errors.New("key manager is nil")
	}
	if random == nil {
		return nil, errors.New("random source is nil")
	}
	return &AESGCM{keyManager: keyManager, random: random}, nil
}

// GenerateDEK returns a cryptographically random 256-bit key.
func (e *AESGCM) GenerateDEK() ([]byte, error) {
	dek := make([]byte, DEKSize)
	e.randomMu.Lock()
	_, err := io.ReadFull(e.random, dek)
	e.randomMu.Unlock()
	if err != nil {
		zeroize(dek)
		return nil, fmt.Errorf("generate DEK: %w", err)
	}
	return dek, nil
}

// EncryptChunk encrypts plaintext and wraps a unique DEK using the key manager.
// Use EncryptChunkWithContext when request cancellation needs to reach the KMS.
func (e *AESGCM) EncryptChunk(plaintext []byte) (ciphertext, nonce, wrappedDEK []byte, keyID string, err error) {
	encrypted, err := e.EncryptChunkWithContext(context.Background(), plaintext)
	if err != nil {
		return nil, nil, nil, "", err
	}
	return encrypted.Ciphertext, encrypted.Nonce, encrypted.WrappedDEK, encrypted.KeyID, nil
}

// EncryptChunkWithContext performs envelope encryption with a cancellable KMS call.
func (e *AESGCM) EncryptChunkWithContext(ctx context.Context, plaintext []byte) (EncryptedChunk, error) {
	if err := validateContext(ctx); err != nil {
		return EncryptedChunk{}, err
	}
	dek, err := e.GenerateDEK()
	if err != nil {
		return EncryptedChunk{}, err
	}
	defer zeroize(dek)

	block, err := aes.NewCipher(dek)
	if err != nil {
		return EncryptedChunk{}, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptedChunk{}, fmt.Errorf("create AES-GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	e.randomMu.Lock()
	_, err = io.ReadFull(e.random, nonce)
	e.randomMu.Unlock()
	if err != nil {
		zeroize(nonce)
		return EncryptedChunk{}, fmt.Errorf("generate GCM nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	wrappedDEK, keyID, err := e.keyManager.WrapKey(ctx, dek)
	if err != nil {
		zeroize(ciphertext)
		zeroize(nonce)
		zeroize(wrappedDEK)
		return EncryptedChunk{}, fmt.Errorf("wrap chunk DEK: %w", err)
	}
	if len(wrappedDEK) == 0 || keyID == "" {
		zeroize(ciphertext)
		zeroize(nonce)
		zeroize(wrappedDEK)
		return EncryptedChunk{}, errors.New("key manager returned an empty wrapped DEK or key ID")
	}
	return EncryptedChunk{
		Ciphertext: ciphertext,
		Nonce:      nonce,
		WrappedDEK: wrappedDEK,
		KeyID:      keyID,
	}, nil
}

// DecryptChunk decrypts ciphertext using the wrapped per-chunk DEK.
func (e *AESGCM) DecryptChunk(ciphertext, nonce, wrappedDEK []byte, keyID string) ([]byte, error) {
	return e.DecryptChunkWithContext(context.Background(), ciphertext, nonce, wrappedDEK, keyID)
}

// DecryptChunkWithContext performs a cancellable KMS unwrap followed by GCM authentication.
func (e *AESGCM) DecryptChunkWithContext(ctx context.Context, ciphertext, nonce, wrappedDEK []byte, keyID string) ([]byte, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if len(nonce) != NonceSize {
		return nil, fmt.Errorf("GCM nonce must be %d bytes, got %d", NonceSize, len(nonce))
	}
	if len(ciphertext) < TagSize {
		return nil, fmt.Errorf("ciphertext is shorter than the GCM tag: %d bytes", len(ciphertext))
	}
	dek, err := e.keyManager.UnwrapKey(ctx, wrappedDEK, keyID)
	if err != nil {
		zeroize(dek)
		return nil, fmt.Errorf("unwrap chunk DEK: %w", err)
	}
	defer zeroize(dek)
	if len(dek) != DEKSize {
		return nil, fmt.Errorf("unwrapped DEK must be %d bytes, got %d", DEKSize, len(dek))
	}

	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("authenticate and decrypt chunk: %w", err)
	}
	return plaintext, nil
}

func zeroize(data []byte) {
	for i := range data {
		data[i] = 0
	}
	runtime.KeepAlive(data)
}
