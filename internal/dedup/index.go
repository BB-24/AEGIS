package dedup

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	ErrChunkExists   = errors.New("chunk already exists")
	ErrChunkNotFound = errors.New("chunk not found")
	ErrNoReferences  = errors.New("chunk has no references")
	ErrClosed        = errors.New("dedup index is closed")
)

var chunksBucket = []byte("chunks")

// ChunkState describes the lifecycle state of a content-addressed chunk.
type ChunkState string

const (
	Available ChunkState = "available"
	Repairing ChunkState = "repairing"
	Missing   ChunkState = "missing"
	Deleted   ChunkState = "deleted"
)

// ShardLocation identifies a stored shard belonging to a chunk.
type ShardLocation struct {
	NodeID     string `json:"node_id"`
	ShardIndex int    `json:"shard_index"`
	Location   string `json:"location"`
	Size       int64  `json:"size"`
}

// ChunkMetadata records storage and reference information for a chunk.
type ChunkMetadata struct {
	Hash           [32]byte        `json:"hash"`
	LogicalSize    int64           `json:"logical_size"`
	CompressedSize int64           `json:"compressed_size"`
	EncryptedSize  int64           `json:"encrypted_size"`
	RefCount       int64           `json:"ref_count"`
	ShardLocations []ShardLocation `json:"shard_locations,omitempty"`
	State          ChunkState      `json:"state"`
}

// DedupStats summarizes active chunks in the index.
type DedupStats struct {
	UniqueChunks    int64
	TotalReferences int64
	LogicalBytes    int64
	CompressedBytes int64
	EncryptedBytes  int64
}

// DedupIndex stores content metadata and reference counts.
type DedupIndex interface {
	Lookup(hash [32]byte) (*ChunkMetadata, bool)
	Add(hash [32]byte, meta ChunkMetadata) error
	IncRef(hash [32]byte) error
	DecRef(hash [32]byte) (bool, error)
	Stats() DedupStats
}

// Index maintains an in-memory cache backed by BoltDB when opened with Open.
// The cache is loaded at startup and updated only after durable writes succeed.
type Index struct {
	mu      sync.RWMutex
	entries map[[32]byte]ChunkMetadata
	db      *bolt.DB
	closed  bool
}

var _ DedupIndex = (*Index)(nil)

// NewMemoryIndex creates a concurrency-safe, non-persistent index.
func NewMemoryIndex() *Index {
	return &Index{entries: make(map[[32]byte]ChunkMetadata)}
}

// Open opens a persistent BoltDB index and loads its contents into memory.
func Open(path string) (*Index, error) {
	if path == "" {
		return nil, errors.New("dedup index path is empty")
	}

	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("open dedup index: %w", err)
	}

	index := &Index{
		entries: make(map[[32]byte]ChunkMetadata),
		db:      db,
	}
	err = db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(chunksBucket)
		if err != nil {
			return err
		}
		return bucket.ForEach(func(key, value []byte) error {
			if len(key) != 32 {
				return fmt.Errorf("invalid dedup index key length %d", len(key))
			}
			var hash [32]byte
			copy(hash[:], key)
			var meta ChunkMetadata
			if err := json.Unmarshal(value, &meta); err != nil {
				return fmt.Errorf("decode metadata for chunk %x: %w", key, err)
			}
			if meta.Hash != hash {
				return fmt.Errorf("metadata hash does not match key %x", key)
			}
			index.entries[hash] = cloneMetadata(meta)
			return nil
		})
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("load dedup index: %w", err)
	}
	return index, nil
}

// Close closes the persistent database. It is a no-op for a memory-only index.
func (i *Index) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.closed {
		return nil
	}
	i.closed = true
	if i.db == nil {
		return nil
	}
	if err := i.db.Close(); err != nil {
		return fmt.Errorf("close dedup index: %w", err)
	}
	return nil
}

// Lookup returns a copy of the metadata if the hash has an index entry.
// Deleted entries remain visible so callers can distinguish tombstones.
func (i *Index) Lookup(hash [32]byte) (*ChunkMetadata, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	meta, ok := i.entries[hash]
	if !ok {
		return nil, false
	}
	copy := cloneMetadata(meta)
	return &copy, true
}

// Add registers a chunk with one reference. Reusing a deleted hash replaces
// its tombstone; adding an active hash returns ErrChunkExists.
func (i *Index) Add(hash [32]byte, meta ChunkMetadata) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := i.ensureOpen(); err != nil {
		return err
	}
	if meta.Hash != ([32]byte{}) && meta.Hash != hash {
		return errors.New("chunk metadata hash does not match index key")
	}
	if meta.RefCount < 0 {
		return errors.New("chunk reference count cannot be negative")
	}
	if meta.RefCount == 0 {
		meta.RefCount = 1
	}
	if meta.State == "" || meta.State == Deleted {
		meta.State = Available
	}
	if existing, ok := i.entries[hash]; ok && existing.State != Deleted {
		return ErrChunkExists
	}

	meta.Hash = hash
	meta = cloneMetadata(meta)
	if err := i.persistLocked(hash, meta); err != nil {
		return err
	}
	i.entries[hash] = meta
	return nil
}

// IncRef increments the reference count of an active chunk.
func (i *Index) IncRef(hash [32]byte) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := i.ensureOpen(); err != nil {
		return err
	}
	meta, ok := i.entries[hash]
	if !ok || meta.State == Deleted {
		return ErrChunkNotFound
	}
	if meta.RefCount <= 0 {
		return ErrNoReferences
	}
	meta.RefCount++
	if err := i.persistLocked(hash, meta); err != nil {
		return err
	}
	i.entries[hash] = meta
	return nil
}

// DecRef decrements the reference count and tombstones the chunk at zero.
// It returns true when the caller should reclaim the stored chunk.
func (i *Index) DecRef(hash [32]byte) (bool, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := i.ensureOpen(); err != nil {
		return false, err
	}
	meta, ok := i.entries[hash]
	if !ok || meta.State == Deleted {
		return false, ErrChunkNotFound
	}
	if meta.RefCount <= 0 {
		return false, ErrNoReferences
	}
	meta.RefCount--
	reclaim := meta.RefCount == 0
	if reclaim {
		meta.State = Deleted
	}
	if err := i.persistLocked(hash, meta); err != nil {
		return false, err
	}
	i.entries[hash] = meta
	return reclaim, nil
}

// Stats returns aggregate metadata for chunks that still have references.
func (i *Index) Stats() DedupStats {
	i.mu.RLock()
	defer i.mu.RUnlock()
	var stats DedupStats
	for _, meta := range i.entries {
		if meta.RefCount <= 0 || meta.State == Deleted {
			continue
		}
		stats.UniqueChunks++
		stats.TotalReferences += meta.RefCount
		stats.LogicalBytes += meta.LogicalSize
		stats.CompressedBytes += meta.CompressedSize
		stats.EncryptedBytes += meta.EncryptedSize
	}
	return stats
}

func (i *Index) ensureOpen() error {
	if i.closed {
		return ErrClosed
	}
	return nil
}

func (i *Index) persistLocked(hash [32]byte, meta ChunkMetadata) error {
	if i.db == nil {
		return nil
	}
	value, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("encode metadata for chunk %x: %w", hash, err)
	}
	if err := i.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(chunksBucket)
		if bucket == nil {
			return errors.New("dedup index bucket is missing")
		}
		return bucket.Put(hash[:], value)
	}); err != nil {
		return fmt.Errorf("persist metadata for chunk %x: %w", hash, err)
	}
	return nil
}

func cloneMetadata(meta ChunkMetadata) ChunkMetadata {
	meta.ShardLocations = append([]ShardLocation(nil), meta.ShardLocations...)
	return meta
}
