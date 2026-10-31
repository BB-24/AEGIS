package pipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"sync"
	"testing"

	"github.com/aegis/aegis/internal/chunker"
	"github.com/aegis/aegis/internal/compression"
	aegiscrypto "github.com/aegis/aegis/internal/crypto"
	"github.com/aegis/aegis/internal/crypto/kms"
	"github.com/aegis/aegis/internal/dedup"
)

type testEncryptor struct{}

func (testEncryptor) EncryptChunkWithContext(_ context.Context, plaintext []byte) (aegiscrypto.EncryptedChunk, error) {
	return aegiscrypto.EncryptedChunk{
		Ciphertext: append([]byte("encrypted:"), plaintext...),
		Nonce:      make([]byte, aegiscrypto.NonceSize),
		WrappedDEK: []byte("wrapped"),
		KeyID:      "test-key",
	}, nil
}

type testStore struct {
	mu         sync.Mutex
	calls      int
	lastStored []byte
}

func (s *testStore) Store(_ context.Context, _ [32]byte, encrypted []byte) ([]dedup.ShardLocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.lastStored = append(s.lastStored[:0], encrypted...)
	return []dedup.ShardLocation{{NodeID: "test-node", Location: "chunk", Size: int64(len(encrypted))}}, nil
}

func TestProcessorDeduplicatesAndProcessesNewChunks(t *testing.T) {
	index := dedup.NewMemoryIndex()
	compressor, err := compression.New(compression.Config{})
	if err != nil {
		t.Fatal(err)
	}
	store := &testStore{}
	processor, err := New(index, compressor, testEncryptor{}, store)
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte("aegis backup contents\n"), 2048)
	input := chunker.Chunk{Data: data, Length: len(data), Hash: sha256.Sum256(data)}

	first, err := processor.Process(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Deduplicated || first.Metadata.RefCount != 1 || first.Metadata.EncryptedSize == 0 {
		t.Fatalf("first process result = %+v", first)
	}
	if first.Metadata.KeyID != "test-key" || len(first.Metadata.Nonce) != aegiscrypto.NonceSize || string(first.Metadata.WrappedDEK) != "wrapped" {
		t.Fatalf("encryption metadata not retained: %+v", first.Metadata)
	}
	second, err := processor.Process(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Deduplicated || second.Metadata.RefCount != 2 {
		t.Fatalf("duplicate process result = %+v", second)
	}
	if store.calls != 1 {
		t.Fatalf("store calls = %d, want 1", store.calls)
	}
	if stats := index.Stats(); stats.UniqueChunks != 1 || stats.TotalReferences != 2 {
		t.Fatalf("index stats = %+v", stats)
	}
}

func TestProcessorWithAESGCMEncryptor(t *testing.T) {
	provider, err := kms.NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}
	keyManager, err := aegiscrypto.NewKeyManager(provider)
	if err != nil {
		t.Fatal(err)
	}
	encryptor, err := aegiscrypto.NewEncryptor(keyManager)
	if err != nil {
		t.Fatal(err)
	}
	compressor, err := compression.New(compression.Config{})
	if err != nil {
		t.Fatal(err)
	}
	store := &testStore{}
	processor, err := New(dedup.NewMemoryIndex(), compressor, encryptor, store)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("integration test chunk")
	input := chunker.Chunk{Data: data, Length: len(data), Hash: sha256.Sum256(data)}
	result, err := processor.Process(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	store.mu.Lock()
	ciphertext := append([]byte(nil), store.lastStored...)
	store.mu.Unlock()
	decodedCompressed, err := encryptor.DecryptChunk(
		ciphertext,
		result.Metadata.Nonce,
		result.Metadata.WrappedDEK,
		result.Metadata.KeyID,
	)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := compressor.Decompress(decodedCompressed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatal("pipeline encrypted output could not be restored to the input chunk")
	}
}

func TestProcessorValidatesChunkHash(t *testing.T) {
	compressor, err := compression.New(compression.Config{})
	if err != nil {
		t.Fatal(err)
	}
	processor, err := New(dedup.NewMemoryIndex(), compressor, testEncryptor{}, &testStore{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := processor.Process(context.Background(), chunker.Chunk{
		Data: []byte("data"), Length: 4, Hash: [32]byte{1},
	}); err == nil {
		t.Fatal("Process accepted an invalid SHA-256")
	}
}
