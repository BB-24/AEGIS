package erasure

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/klauspost/reedsolomon"
)

const (
	DataShards   = 6
	ParityShards = 3
	TotalShards  = DataShards + ParityShards
)

var (
	ErrInsufficientShards = errors.New("at least 6 valid shards are required")
	ErrInvalidShardCount  = errors.New("exactly 9 shard slots are required")
)

// Encoder splits encrypted chunk bytes into six data shards and three parity
// shards. Returned shards include their versioned header and checksum.
type Encoder interface {
	Encode(data []byte) ([][]byte, error)
	Decode(shards [][]byte) ([]byte, error)
	ShardCount() int
	DataShardCount() int
	ParityShardCount() int
}

type reedSolomonEncoder struct {
	codec reedsolomon.Encoder
}

var _ Encoder = (*reedSolomonEncoder)(nil)

func NewEncoder() (Encoder, error) {
	codec, err := reedsolomon.New(DataShards, ParityShards)
	if err != nil {
		return nil, fmt.Errorf("create Reed-Solomon codec: %w", err)
	}
	return &reedSolomonEncoder{codec: codec}, nil
}

func (e *reedSolomonEncoder) Encode(data []byte) ([][]byte, error) {
	payloadSize := int(shardPayloadSize(uint64(len(data))))
	if uint64(payloadSize) > uint64(^uint32(0)) {
		return nil, errors.New("shard payload exceeds shard format limit")
	}

	shards := make([][]byte, TotalShards)
	payloads := make([][]byte, TotalShards)
	for index := range shards {
		shards[index] = make([]byte, ShardHeaderSize+payloadSize)
		payloads[index] = shards[index][ShardHeaderSize:]
	}
	if len(data) > 0 {
		for index := 0; index < DataShards; index++ {
			start := index * payloadSize
			end := min(start+payloadSize, len(data))
			if start < len(data) {
				copy(payloads[index], data[start:end])
			}
		}
		if err := e.codec.Encode(payloads); err != nil {
			return nil, fmt.Errorf("generate parity shards: %w", err)
		}
	}

	chunkHash := sha256.Sum256(data)
	for index, frame := range shards {
		if err := writeShardHeader(frame, Shard{
			Index:        index,
			OriginalSize: uint64(len(data)),
			ChunkHash:    chunkHash,
			Payload:      frame[ShardHeaderSize:],
		}); err != nil {
			return nil, fmt.Errorf("encode shard %d: %w", index, err)
		}
	}
	return shards, nil
}

func (e *reedSolomonEncoder) Decode(shardFrames [][]byte) ([]byte, error) {
	if len(shardFrames) != TotalShards {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidShardCount, len(shardFrames))
	}

	payloads := make([][]byte, TotalShards)
	seen := make([]bool, TotalShards)
	var originalSize uint64
	var chunkHash [sha256.Size]byte
	haveMetadata := false
	validCount := 0

	for _, frame := range shardFrames {
		if len(frame) == 0 {
			continue
		}
		shard, err := ParseShard(frame)
		if err != nil {
			continue
		}
		if seen[shard.Index] {
			return nil, fmt.Errorf("duplicate shard index %d", shard.Index)
		}
		seen[shard.Index] = true
		if !haveMetadata {
			originalSize, chunkHash, haveMetadata = shard.OriginalSize, shard.ChunkHash, true
		} else if shard.OriginalSize != originalSize || shard.ChunkHash != chunkHash {
			return nil, errors.New("shard metadata does not match")
		}
		payloads[shard.Index] = shard.Payload
		validCount++
	}

	if validCount < DataShards {
		return nil, fmt.Errorf("%w: got %d", ErrInsufficientShards, validCount)
	}
	maxInt := uint64(^uint(0) >> 1)
	if originalSize > maxInt {
		return nil, errors.New("original size exceeds platform limit")
	}
	expectedPayloadSize := shardPayloadSize(originalSize)
	for index, payload := range payloads {
		if payload != nil && uint64(len(payload)) != expectedPayloadSize {
			return nil, fmt.Errorf("shard %d payload size does not match metadata", index)
		}
	}

	if originalSize == 0 {
		if sha256.Sum256(nil) != chunkHash {
			return nil, errors.New("reconstructed chunk hash mismatch")
		}
		return []byte{}, nil
	}
	dataShardsPresent := true
	for index := 0; index < DataShards; index++ {
		if payloads[index] == nil {
			dataShardsPresent = false
			break
		}
	}
	if !dataShardsPresent {
		if err := e.codec.ReconstructData(payloads); err != nil {
			return nil, fmt.Errorf("reconstruct data shards: %w", err)
		}
	}

	decoded := make([]byte, int(originalSize))
	offset := 0
	for index := 0; index < DataShards && offset < len(decoded); index++ {
		offset += copy(decoded[offset:], payloads[index])
	}
	if offset != len(decoded) {
		return nil, errors.New("reconstructed data shards are shorter than original size")
	}
	if sha256.Sum256(decoded) != chunkHash {
		return nil, errors.New("reconstructed chunk hash mismatch")
	}
	return decoded, nil
}

func (*reedSolomonEncoder) ShardCount() int       { return TotalShards }
func (*reedSolomonEncoder) DataShardCount() int   { return DataShards }
func (*reedSolomonEncoder) ParityShardCount() int { return ParityShards }
