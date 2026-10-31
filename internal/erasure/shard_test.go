package erasure

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

func TestShardHeaderAndChecksum(t *testing.T) {
	payload := []byte("payload")
	hash := sha256.Sum256([]byte("whole chunk"))
	frame, err := MarshalShard(Shard{
		Index: 4, OriginalSize: 42, ChunkHash: hash,
		Payload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != ShardHeaderSize+len(payload) {
		t.Fatalf("frame size = %d, want %d", len(frame), ShardHeaderSize+len(payload))
	}
	if got := binary.BigEndian.Uint32(frame[44:48]); got != crc32.Checksum(payload, castagnoliTable) {
		t.Fatalf("header checksum = %08x", got)
	}
	parsed, err := ParseShard(frame)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Index != 4 || parsed.OriginalSize != 42 || parsed.ChunkHash != hash ||
		!bytes.Equal(parsed.Payload, payload) {
		t.Fatalf("parsed shard does not match input: %+v", parsed)
	}
}

func TestParseShardRejectsMalformedFrames(t *testing.T) {
	frame, err := MarshalShard(Shard{
		Index: 0, OriginalSize: 1, Payload: []byte{1},
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func([]byte) []byte{
		"short header": func(frame []byte) []byte { return frame[:ShardHeaderSize-1] },
		"magic":        func(frame []byte) []byte { frame[0] ^= 1; return frame },
		"version":      func(frame []byte) []byte { frame[4]++; return frame },
		"index":        func(frame []byte) []byte { frame[5] = TotalShards; return frame },
		"geometry":     func(frame []byte) []byte { frame[7]++; return frame },
		"reserved":     func(frame []byte) []byte { frame[9] = 1; return frame },
		"length":       func(frame []byte) []byte { frame[56]++; return frame },
		"checksum":     func(frame []byte) []byte { frame[44]++; return frame },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			candidate := append([]byte(nil), frame...)
			candidate = mutate(candidate)
			if _, err := ParseShard(candidate); err == nil {
				t.Fatal("ParseShard accepted malformed frame")
			}
		})
	}
}

func TestMarshalShardRejectsInvalidMetadata(t *testing.T) {
	tests := []Shard{
		{Index: -1},
		{Index: TotalShards},
		{Index: 0, OriginalSize: 7, Payload: []byte{1}},
	}
	for _, shard := range tests {
		if _, err := MarshalShard(shard); err == nil {
			t.Errorf("MarshalShard accepted invalid metadata: %+v", shard)
		}
	}
}
