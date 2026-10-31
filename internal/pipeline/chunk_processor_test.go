package pipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"sync"
	"testing"

	"github.com/aegis/aegis/internal/chunker"
	"github.com/aegis/aegis/internal/compression"
	"github.com/aegis/aegis/internal/dedup"
)

type testEncryptor struct{}

func (testEncryptor) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	return append([]byte("encrypted:"), plaintext...), nil
}

type testStore struct {
	mu    sync.Mutex
	calls int
}

func (s *testStore) Store(_ context.Context, _ [32]byte, encrypted []byte) ([]dedup.ShardLocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
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
