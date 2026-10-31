package dedup

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestIndexAddLookupAndReferenceCounting(t *testing.T) {
	index := NewMemoryIndex()
	hash := [32]byte{1, 2, 3}
	meta := ChunkMetadata{
		Hash:           hash,
		LogicalSize:    100,
		CompressedSize: 60,
		EncryptedSize:  76,
		ShardLocations: []ShardLocation{{NodeID: "node-1", ShardIndex: 0}},
	}
	if err := index.Add(hash, meta); err != nil {
		t.Fatal(err)
	}
	if err := index.Add(hash, meta); !errors.Is(err, ErrChunkExists) {
		t.Fatalf("Add duplicate error = %v, want ErrChunkExists", err)
	}
	if err := index.IncRef(hash); err != nil {
		t.Fatal(err)
	}

	got, ok := index.Lookup(hash)
	if !ok || got.RefCount != 2 || got.State != Available {
		t.Fatalf("Lookup = (%+v, %v), want refcount 2 and available", got, ok)
	}
	got.ShardLocations[0].NodeID = "mutated"
	stored, _ := index.Lookup(hash)
	if stored.ShardLocations[0].NodeID != "node-1" {
		t.Fatal("Lookup exposed mutable index metadata")
	}

	if reclaim, err := index.DecRef(hash); err != nil || reclaim {
		t.Fatalf("DecRef = (%v, %v), want (false, nil)", reclaim, err)
	}
	if reclaim, err := index.DecRef(hash); err != nil || !reclaim {
		t.Fatalf("DecRef = (%v, %v), want (true, nil)", reclaim, err)
	}
	deleted, ok := index.Lookup(hash)
	if !ok || deleted.State != Deleted || deleted.RefCount != 0 {
		t.Fatalf("deleted metadata = (%+v, %v)", deleted, ok)
	}
	if stats := index.Stats(); stats.UniqueChunks != 0 || stats.TotalReferences != 0 {
		t.Fatalf("Stats for deleted chunk = %+v, want zero active chunks", stats)
	}
	if err := index.Add(hash, meta); err != nil {
		t.Fatalf("Add should replace a tombstone: %v", err)
	}
}

func TestPersistentIndexReopens(t *testing.T) {
	path := t.TempDir() + "/dedup.db"
	hash := [32]byte{4, 5, 6}
	index, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.Add(hash, ChunkMetadata{LogicalSize: 42, CompressedSize: 21}); err != nil {
		t.Fatal(err)
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	meta, ok := reopened.Lookup(hash)
	if !ok || meta.RefCount != 1 || meta.Hash != hash || meta.LogicalSize != 42 {
		t.Fatalf("reopened metadata = (%+v, %v)", meta, ok)
	}
}

func TestIndexConcurrentReferenceUpdates(t *testing.T) {
	index := NewMemoryIndex()
	hash := [32]byte{7}
	if err := index.Add(hash, ChunkMetadata{}); err != nil {
		t.Fatal(err)
	}

	const workers = 24
	const increments = 50
	var group sync.WaitGroup
	errors := make(chan error, workers)
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			for n := 0; n < increments; n++ {
				if err := index.IncRef(hash); err != nil {
					errors <- fmt.Errorf("worker %d increment: %w", worker, err)
					return
				}
			}
		}(worker)
	}
	group.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	meta, ok := index.Lookup(hash)
	if !ok || meta.RefCount != 1+workers*increments {
		t.Fatalf("reference count = %+v, want %d", meta, 1+workers*increments)
	}
}

func TestIndexRejectsMismatchedHash(t *testing.T) {
	index := NewMemoryIndex()
	if err := index.Add([32]byte{1}, ChunkMetadata{Hash: [32]byte{2}}); err == nil {
		t.Fatal("Add accepted metadata with a mismatched hash")
	}
}
