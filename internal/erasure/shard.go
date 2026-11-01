package erasure

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

const (
	ShardHeaderSize = 64
	ShardVersion    = 1
)

var shardMagic = [4]byte{'A', 'E', 'G', 'I'}

var castagnoliTable = crc32.MakeTable(crc32.Castagnoli)

// Shard holds decoded header metadata and the shard payload.
type Shard struct {
	Index        int
	OriginalSize uint64
	ChunkHash    [32]byte
	Payload      []byte
}

// MarshalShard creates a versioned frame and computes its CRC32C payload
// checksum. Header metadata occupies exactly 64 bytes.
func MarshalShard(shard Shard) ([]byte, error) {
	if shard.Index < 0 || shard.Index >= TotalShards {
		return nil, fmt.Errorf("shard index %d is out of range", shard.Index)
	}
	if uint64(len(shard.Payload)) > uint64(^uint32(0)) {
		return nil, errors.New("shard payload exceeds format limit")
	}
	expectedSize := shardPayloadSize(shard.OriginalSize)
	if uint64(len(shard.Payload)) != expectedSize {
		return nil, errors.New("shard payload size does not match original size")
	}
	if len(shard.Payload) > maxInt()-ShardHeaderSize {
		return nil, errors.New("shard frame exceeds platform limit")
	}

	frame := make([]byte, ShardHeaderSize+len(shard.Payload))
	copy(frame[ShardHeaderSize:], shard.Payload)
	if err := writeShardHeader(frame, shard); err != nil {
		return nil, err
	}
	return frame, nil
}

func writeShardHeader(frame []byte, shard Shard) error {
	if len(frame) < ShardHeaderSize {
		return errors.New("shard frame is shorter than its header")
	}
	payload := frame[ShardHeaderSize:]
	if shard.Index < 0 || shard.Index >= TotalShards {
		return fmt.Errorf("shard index %d is out of range", shard.Index)
	}
	if uint64(len(payload)) > uint64(^uint32(0)) {
		return errors.New("shard payload exceeds format limit")
	}
	expectedSize := shardPayloadSize(shard.OriginalSize)
	if uint64(len(payload)) != expectedSize {
		return errors.New("shard payload size does not match original size")
	}
	if len(payload) != len(shard.Payload) {
		return errors.New("shard frame payload length does not match payload")
	}
	copy(frame[0:4], shardMagic[:])
	frame[4] = ShardVersion
	frame[5] = byte(shard.Index)
	frame[6] = TotalShards
	frame[7] = DataShards
	frame[8] = ParityShards
	copy(frame[12:44], shard.ChunkHash[:])
	binary.BigEndian.PutUint32(frame[44:48], crc32.Checksum(payload, castagnoliTable))
	binary.BigEndian.PutUint64(frame[48:56], shard.OriginalSize)
	// The payload length is bounded to uint32 above.
	binary.BigEndian.PutUint32(frame[56:60], uint32(len(payload))) //nolint:gosec
	return nil
}

// ParseShard validates the frame format and checksum before returning payload.
func ParseShard(frame []byte) (Shard, error) {
	if len(frame) < ShardHeaderSize {
		return Shard{}, errors.New("shard frame is shorter than its header")
	}
	if err := validateShardHeader(frame); err != nil {
		return Shard{}, err
	}
	payloadSize := binary.BigEndian.Uint32(frame[56:60])
	if uint64(len(frame)) != uint64(payloadSize)+ShardHeaderSize {
		return Shard{}, errors.New("shard payload length does not match frame")
	}
	payload := frame[ShardHeaderSize:]
	expectedChecksum := binary.BigEndian.Uint32(frame[44:48])
	if crc32.Checksum(payload, castagnoliTable) != expectedChecksum {
		return Shard{}, errors.New("shard payload CRC32C mismatch")
	}
	originalSize := binary.BigEndian.Uint64(frame[48:56])
	if uint64(payloadSize) != shardPayloadSize(originalSize) {
		return Shard{}, errors.New("shard payload length does not match original size")
	}
	var hash [32]byte
	copy(hash[:], frame[12:44])
	return Shard{
		Index:        int(frame[5]),
		OriginalSize: originalSize,
		ChunkHash:    hash,
		Payload:      payload,
	}, nil
}

func validateShardHeader(frame []byte) error {
	if !hasShardMagic(frame) {
		return errors.New("invalid shard magic")
	}
	if frame[4] != ShardVersion {
		return fmt.Errorf("unsupported shard version %d", frame[4])
	}
	if !hasValidShardGeometry(frame) {
		return errors.New("invalid shard geometry")
	}
	if !reservedShardBytesAreZero(frame) {
		return errors.New("nonzero reserved shard header bytes")
	}
	return nil
}

func hasShardMagic(frame []byte) bool {
	return frame[0] == shardMagic[0] && frame[1] == shardMagic[1] &&
		frame[2] == shardMagic[2] && frame[3] == shardMagic[3]
}

func hasValidShardGeometry(frame []byte) bool {
	return frame[5] < TotalShards && frame[6] == TotalShards &&
		frame[7] == DataShards && frame[8] == ParityShards
}

func reservedShardBytesAreZero(frame []byte) bool {
	return frame[9] == 0 && frame[10] == 0 && frame[11] == 0 &&
		frame[60] == 0 && frame[61] == 0 && frame[62] == 0 && frame[63] == 0
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

func shardPayloadSize(size uint64) uint64 {
	shardSize := size / DataShards
	if size%DataShards != 0 {
		shardSize++
	}
	return shardSize
}
