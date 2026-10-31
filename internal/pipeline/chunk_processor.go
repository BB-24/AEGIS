package pipeline

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"

	"github.com/aegis/aegis/internal/chunker"
	"github.com/aegis/aegis/internal/compression"
	aegiscrypto "github.com/aegis/aegis/internal/crypto"
	"github.com/aegis/aegis/internal/dedup"
)

// Encryptor encrypts compressed chunk data. Production implementations should
// use authenticated encryption and manage keys outside this package.
type Encryptor interface {
	EncryptChunkWithContext(ctx context.Context, plaintext []byte) (aegiscrypto.EncryptedChunk, error)
}

// ChunkStore persists encrypted chunk bytes and returns their shard locations.
// Store must be idempotent for a content hash.
type ChunkStore interface {
	Store(ctx context.Context, hash [32]byte, encrypted []byte) ([]dedup.ShardLocation, error)
}

// ProcessedChunk describes the result of processing a chunk.
type ProcessedChunk struct {
	Metadata     dedup.ChunkMetadata
	Deduplicated bool
}

// Processor performs deduplication, compression, encryption, and storage.
type Processor struct {
	mu         sync.Mutex
	index      dedup.DedupIndex
	compressor compression.Compressor
	encryptor  Encryptor
	store      ChunkStore
}

// New creates a chunk processor with explicit storage and encryption dependencies.
func New(index dedup.DedupIndex, compressor compression.Compressor, encryptor Encryptor, store ChunkStore) (*Processor, error) {
	if index == nil {
		return nil, errors.New("dedup index is nil")
	}
	if compressor == nil {
		return nil, errors.New("chunk compressor is nil")
	}
	if encryptor == nil {
		return nil, errors.New("chunk encryptor is nil")
	}
	if store == nil {
		return nil, errors.New("chunk store is nil")
	}
	return &Processor{index: index, compressor: compressor, encryptor: encryptor, store: store}, nil
}

// Process stores a new chunk or increments the reference count of an available
// chunk. Calls on one Processor are serialized to prevent duplicate work.
func (p *Processor) Process(ctx context.Context, input chunker.Chunk) (ProcessedChunk, error) {
	if ctx == nil {
		return ProcessedChunk{}, errors.New("chunk processing context is nil")
	}
	if err := ctx.Err(); err != nil {
		return ProcessedChunk{}, fmt.Errorf("chunk processing canceled: %w", err)
	}
	if len(input.Data) == 0 {
		return ProcessedChunk{}, errors.New("chunk data is empty")
	}
	if input.Length != len(input.Data) {
		return ProcessedChunk{}, fmt.Errorf("chunk length %d does not match data length %d", input.Length, len(input.Data))
	}
	hash := sha256.Sum256(input.Data)
	if input.Hash != hash {
		return ProcessedChunk{}, errors.New("chunk SHA-256 does not match chunk data")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if existing, ok := p.index.Lookup(hash); ok {
		if existing == nil {
			return ProcessedChunk{}, fmt.Errorf("dedup index returned nil metadata for chunk %x", hash)
		}
		if existing.State == dedup.Available && existing.RefCount > 0 {
			if err := p.index.IncRef(hash); err != nil {
				return ProcessedChunk{}, fmt.Errorf("increment reference for chunk %x: %w", hash, err)
			}
			existing.RefCount++
			return ProcessedChunk{Metadata: *existing, Deduplicated: true}, nil
		}
		if existing.State != dedup.Deleted {
			return ProcessedChunk{}, fmt.Errorf("chunk %x is not available for deduplication (state %q)", hash, existing.State)
		}
	}

	compressed, err := p.compressor.Compress(input.Data)
	if err != nil {
		return ProcessedChunk{}, fmt.Errorf("compress chunk %x: %w", hash, err)
	}
	encrypted, err := p.encryptor.EncryptChunkWithContext(ctx, compressed)
	if err != nil {
		return ProcessedChunk{}, fmt.Errorf("encrypt chunk %x: %w", hash, err)
	}
	if len(encrypted.Ciphertext) == 0 || len(encrypted.Nonce) != aegiscrypto.NonceSize || len(encrypted.WrappedDEK) == 0 || encrypted.KeyID == "" {
		return ProcessedChunk{}, errors.New("encryptor returned incomplete chunk encryption metadata")
	}
	locations, err := p.store.Store(ctx, hash, encrypted.Ciphertext)
	if err != nil {
		return ProcessedChunk{}, fmt.Errorf("store chunk %x: %w", hash, err)
	}

	meta := dedup.ChunkMetadata{
		Hash:           hash,
		LogicalSize:    int64(len(input.Data)),
		CompressedSize: int64(len(compressed)),
		EncryptedSize:  int64(len(encrypted.Ciphertext)),
		Nonce:          encrypted.Nonce,
		WrappedDEK:     encrypted.WrappedDEK,
		KeyID:          encrypted.KeyID,
		RefCount:       1,
		ShardLocations: locations,
		State:          dedup.Available,
	}
	if err := p.index.Add(hash, meta); err != nil {
		if errors.Is(err, dedup.ErrChunkExists) {
			existing, ok := p.index.Lookup(hash)
			if ok && existing != nil && existing.State == dedup.Available && existing.RefCount > 0 {
				if incrementErr := p.index.IncRef(hash); incrementErr != nil {
					return ProcessedChunk{}, fmt.Errorf("register stored chunk %x: %w; increment existing reference: %v", hash, err, incrementErr)
				}
				existing.RefCount++
				return ProcessedChunk{Metadata: *existing, Deduplicated: true}, nil
			}
		}
		return ProcessedChunk{}, fmt.Errorf("register chunk %x after storage: %w", hash, err)
	}
	return ProcessedChunk{Metadata: meta}, nil
}
