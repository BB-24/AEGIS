package erasure

import (
	"bytes"
	"errors"
	"math/rand"
	"sync"
	"testing"
)

func newTestEncoder(t *testing.T) Encoder {
	t.Helper()
	encoder, err := NewEncoder()
	if err != nil {
		t.Fatal(err)
	}
	return encoder
}

func TestEncodeDecodeAndShardCounts(t *testing.T) {
	encoder := newTestEncoder(t)
	data := bytes.Repeat([]byte("encrypted chunk data"), 1700)
	shards, err := encoder.Encode(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(shards) != TotalShards {
		t.Fatalf("Encode returned %d shards, want %d", len(shards), TotalShards)
	}
	if encoder.ShardCount() != 9 || encoder.DataShardCount() != 6 || encoder.ParityShardCount() != 3 {
		t.Fatalf("unexpected shard counts: total=%d data=%d parity=%d",
			encoder.ShardCount(), encoder.DataShardCount(), encoder.ParityShardCount())
	}
	decoded, err := encoder.Decode(shards)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatal("decode did not reproduce original data")
	}
}

func TestDecodeRecoversFromEverySixShardCombination(t *testing.T) {
	encoder := newTestEncoder(t)
	data := make([]byte, 4097)
	rand.New(rand.NewSource(42)).Read(data)
	shards, err := encoder.Encode(data)
	if err != nil {
		t.Fatal(err)
	}

	for first := 0; first < TotalShards; first++ {
		for second := first + 1; second < TotalShards; second++ {
			for third := second + 1; third < TotalShards; third++ {
				missing := map[int]bool{first: true, second: true, third: true}
				available := append([][]byte(nil), shards...)
				for index := range available {
					if missing[index] {
						available[index] = nil
					}
				}
				decoded, err := encoder.Decode(available)
				if err != nil {
					t.Fatalf("missing shards %d,%d,%d: %v", first, second, third, err)
				}
				if !bytes.Equal(decoded, data) {
					t.Fatalf("wrong reconstruction with shards %d,%d,%d missing", first, second, third)
				}
			}
		}
	}
}

func TestDecodeAcceptsShuffledShardFrames(t *testing.T) {
	encoder := newTestEncoder(t)
	data := bytes.Repeat([]byte("shuffle"), 1000)
	shards, err := encoder.Encode(data)
	if err != nil {
		t.Fatal(err)
	}
	rand.New(rand.NewSource(17)).Shuffle(len(shards), func(i, j int) {
		shards[i], shards[j] = shards[j], shards[i]
	})
	decoded, err := encoder.Decode(shards)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatal("shuffled shards did not reconstruct original data")
	}
}

func TestDecodeDetectsCorruptionAndRecoversWithEnoughValidShards(t *testing.T) {
	encoder := newTestEncoder(t)
	data := bytes.Repeat([]byte("corruption test"), 500)
	shards, err := encoder.Encode(data)
	if err != nil {
		t.Fatal(err)
	}
	shards[2][ShardHeaderSize+3] ^= 0x80
	if _, err := ParseShard(shards[2]); err == nil {
		t.Fatal("ParseShard accepted a corrupted payload")
	}
	decoded, err := encoder.Decode(shards)
	if err != nil {
		t.Fatalf("Decode failed to recover one corrupted shard: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatal("recovery with one corrupted shard changed data")
	}

	shards, err = encoder.Encode(data)
	if err != nil {
		t.Fatal(err)
	}
	shards[1][ShardHeaderSize] ^= 1
	shards[7][ShardHeaderSize] ^= 1
	decoded, err = encoder.Decode(shards)
	if err != nil {
		t.Fatalf("Decode failed to recover with seven valid shards: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatal("recovery with two corrupted shards changed data")
	}
}

func TestDecodeRejectsInsufficientShardsAndInvalidInputs(t *testing.T) {
	encoder := newTestEncoder(t)
	shards, err := encoder.Encode([]byte("threshold"))
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 4; index++ {
		shards[index] = nil
	}
	if _, err := encoder.Decode(shards); !errors.Is(err, ErrInsufficientShards) {
		t.Fatalf("Decode error = %v, want ErrInsufficientShards", err)
	}
	if _, err := encoder.Decode(shards[:TotalShards-1]); !errors.Is(err, ErrInvalidShardCount) {
		t.Fatalf("Decode error = %v, want ErrInvalidShardCount", err)
	}
}

func TestDecodeRejectsDuplicateAndInconsistentShardMetadata(t *testing.T) {
	encoder := newTestEncoder(t)
	shards, err := encoder.Encode([]byte("metadata consistency"))
	if err != nil {
		t.Fatal(err)
	}

	duplicates := append([][]byte(nil), shards...)
	duplicates[8] = append([]byte(nil), shards[0]...)
	if _, err := encoder.Decode(duplicates); err == nil {
		t.Fatal("Decode accepted duplicate shard indexes")
	}

	inconsistent := append([][]byte(nil), shards...)
	inconsistent[8] = append([]byte(nil), shards[8]...)
	inconsistent[8][12] ^= 1
	if _, err := encoder.Decode(inconsistent); err == nil {
		t.Fatal("Decode accepted inconsistent chunk hashes")
	}
}

func TestEncodeDecodeBoundarySizes(t *testing.T) {
	encoder := newTestEncoder(t)
	for _, size := range []int{0, 1, 5, 6, 7, 1024, 256 * 1024} {
		data := make([]byte, size)
		rand.New(rand.NewSource(int64(size) + 1)).Read(data)
		shards, err := encoder.Encode(data)
		if err != nil {
			t.Fatalf("Encode size %d: %v", size, err)
		}
		for i, frame := range shards {
			parsed, err := ParseShard(frame)
			if err != nil {
				t.Fatalf("ParseShard size %d index %d: %v", size, i, err)
			}
			wantShardSize := size / DataShards
			if size%DataShards != 0 {
				wantShardSize++
			}
			if len(parsed.Payload) != wantShardSize {
				t.Fatalf("payload size for input %d = %d, want %d", size, len(parsed.Payload), wantShardSize)
			}
		}
		decoded, err := encoder.Decode(shards)
		if err != nil {
			t.Fatalf("Decode size %d: %v", size, err)
		}
		if !bytes.Equal(decoded, data) {
			t.Fatalf("roundtrip changed input of size %d", size)
		}
	}
}

func TestConcurrentEncodeDecode(t *testing.T) {
	encoder := newTestEncoder(t)
	data := bytes.Repeat([]byte("parallel"), 1024)
	const workers = 12
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			shards, err := encoder.Encode(data)
			if err != nil {
				errs <- err
				return
			}
			decoded, err := encoder.Decode(shards)
			if err != nil {
				errs <- err
				return
			}
			if !bytes.Equal(decoded, data) {
				errs <- errors.New("concurrent roundtrip changed data")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func FuzzDecodeCorruptionPatterns(f *testing.F) {
	f.Add([]byte("fuzz seed data"))
	f.Add(make([]byte, 0))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1024*1024 {
			t.Skip()
		}
		encoder, err := NewEncoder()
		if err != nil {
			t.Fatal(err)
		}
		shards, err := encoder.Encode(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(shards[0]) > ShardHeaderSize {
			shards[0][ShardHeaderSize] ^= 1
		}
		decoded, err := encoder.Decode(shards)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(decoded, data) {
			t.Fatal("decoded data differs from input")
		}
	})
}
