package chunker

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"testing"
)

func newDefaultChunker(t *testing.T) *FastCDC {
	t.Helper()
	chunker, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	return chunker
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{name: "zero minimum", config: Config{AvgSize: 64, MaxSize: 128}},
		{name: "average below minimum", config: Config{MinSize: 64, AvgSize: 32, MaxSize: 128}},
		{name: "maximum below average", config: Config{MinSize: 16, AvgSize: 64, MaxSize: 32}},
		{name: "maximum below twice minimum", config: Config{MinSize: 16, AvgSize: 20, MaxSize: 31}},
		{name: "mask exceeds gear hash", config: Config{MinSize: 16, AvgSize: 20, MaxSize: 32, Mask: 1 << hashBits}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.config); err == nil {
				t.Fatal("New() error = nil, want invalid config error")
			}
		})
	}
}

func TestNewAcceptsConfigurableAverageSizes(t *testing.T) {
	for _, avgSize := range []int{32 * 1024, 64 * 1024, 128 * 1024} {
		t.Run(fmt.Sprintf("%dKiB", avgSize/1024), func(t *testing.T) {
			config := Config{MinSize: DefaultMinSize, AvgSize: avgSize, MaxSize: DefaultMaxSize}
			if _, err := New(config); err != nil {
				t.Fatalf("New(%+v) error = %v", config, err)
			}
		})
	}
}

func TestChunkBytesEdgeCases(t *testing.T) {
	chunker := newDefaultChunker(t)
	tests := []struct {
		name string
		size int
		want int
	}{
		{name: "empty", size: 0, want: 0},
		{name: "tiny", size: 5, want: 1},
		{name: "one byte below minimum", size: DefaultMinSize - 1, want: 1},
		{name: "exact minimum", size: DefaultMinSize, want: 1},
		{name: "exact maximum", size: DefaultMaxSize, want: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := chunker.ChunkBytes(make([]byte, tt.size))
			if tt.want >= 0 && len(chunks) != tt.want {
				t.Fatalf("got %d chunks, want %d", len(chunks), tt.want)
			}
			if tt.size > 0 {
				assertChunks(t, make([]byte, tt.size), chunks, chunker.config.MinSize, chunker.config.MaxSize)
			}
		})
	}
}

func TestChunkBytesDeterministic(t *testing.T) {
	chunker := newDefaultChunker(t)
	data := randomData(4*DefaultMaxSize, 1234)

	first := chunker.ChunkBytes(data)
	second := chunker.ChunkBytes(data)
	if len(first) != len(second) {
		t.Fatalf("chunk counts differ: %d != %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Offset != second[i].Offset ||
			first[i].Length != second[i].Length ||
			first[i].Fingerprint != second[i].Fingerprint ||
			first[i].Hash != second[i].Hash ||
			!bytes.Equal(first[i].Data, second[i].Data) {
			t.Fatalf("chunk %d differs across runs", i)
		}
	}
}

func TestChunkBytesSizeBoundsAndAverage(t *testing.T) {
	chunker := newDefaultChunker(t)
	data := randomData(32*1024*1024, 42)
	chunks := chunker.ChunkBytes(data)
	assertChunks(t, data, chunks, DefaultMinSize, DefaultMaxSize)

	var total int
	for _, chunk := range chunks {
		total += chunk.Length
	}
	average := float64(total) / float64(len(chunks))
	if average < DefaultAvgSize*0.9 || average > DefaultAvgSize*1.1 {
		t.Fatalf("average chunk size %.0f, want within 10%% of %d", average, DefaultAvgSize)
	}
}

func TestChunkBytesConfigurableAverageTargets(t *testing.T) {
	data := randomData(16*1024*1024, 43)
	for _, avgSize := range []int{32 * 1024, 48 * 1024, 64 * 1024, 96 * 1024, 128 * 1024} {
		t.Run(fmt.Sprintf("%dKiB", avgSize/1024), func(t *testing.T) {
			config := Config{MinSize: DefaultMinSize, AvgSize: avgSize, MaxSize: DefaultMaxSize}
			chunker, err := New(config)
			if err != nil {
				t.Fatal(err)
			}
			chunks := chunker.ChunkBytes(data)
			assertChunks(t, data, chunks, config.MinSize, config.MaxSize)

			average := float64(len(data)) / float64(len(chunks))
			if average < float64(avgSize)*0.9 || average > float64(avgSize)*1.1 {
				t.Fatalf("average chunk size %.0f, want within 10%% of %d", average, avgSize)
			}
		})
	}
}

func TestChunkBytesHashesAndOffsets(t *testing.T) {
	chunker := newDefaultChunker(t)
	data := randomData(1024*1024, 99)
	chunks := chunker.ChunkBytes(data)
	assertChunks(t, data, chunks, DefaultMinSize, DefaultMaxSize)
	for _, chunk := range chunks {
		want := sha256.Sum256(chunk.Data)
		if chunk.Hash != want {
			t.Fatalf("hash at offset %d does not match SHA-256", chunk.Offset)
		}
		if chunk.Fingerprint > hashMask {
			t.Fatalf("fingerprint %#x exceeds the 48-bit hash mask", chunk.Fingerprint)
		}
		if got := chunker.fingerprint(chunk.Data); chunk.Fingerprint != got {
			t.Fatalf("fingerprint at offset %d = %#x, want %#x", chunk.Offset, chunk.Fingerprint, got)
		}
	}
}

func TestChunkBytesCustomMask(t *testing.T) {
	config := Config{MinSize: 16, AvgSize: 32, MaxSize: 64, Mask: hashMask}
	chunker, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	data := randomData(130, 77)
	chunks := chunker.ChunkBytes(data)
	assertChunks(t, data, chunks, config.MinSize, config.MaxSize)
	if len(chunks) != 3 {
		t.Fatalf("got %d chunks with boundary mask disabled, want 3", len(chunks))
	}
}

func TestNormalizeTailAdjustsShortRemainder(t *testing.T) {
	chunker, err := New(Config{MinSize: 16, AvgSize: 24, MaxSize: 32})
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMN")
	chunks := []Chunk{
		makeChunkView(data[:30], 0, 0),
		makeChunkView(data[30:], 30, 0),
	}
	got := chunker.normalizeTail(chunks)
	if len(got) != 2 || got[0].Length != 24 || got[1].Length != 16 {
		t.Fatalf("normalized chunk lengths = %v, want [24 16]", chunkLengths(got))
	}
	assertChunks(t, data, got, 16, 32)
}

func TestNormalizeTailMergesWhenWithinMaximum(t *testing.T) {
	chunker, err := New(Config{MinSize: 16, AvgSize: 24, MaxSize: 64})
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("abcdefghijklmnopqrs")
	chunks := []Chunk{
		makeChunkView(data[:16], 0, 0),
		makeChunkView(data[16:], 16, 0),
	}
	got := chunker.normalizeTail(chunks)
	if len(got) != 1 || got[0].Length != len(data) {
		t.Fatalf("normalized chunks = %v, want one chunk of length %d", chunkLengths(got), len(data))
	}
	assertChunks(t, data, got, 16, 64)
}

func TestChunkReaderMatchesChunkBytes(t *testing.T) {
	chunker := newDefaultChunker(t)
	data := randomData(3*DefaultMaxSize+1723, 7)
	want := chunker.ChunkBytes(data)

	got, err := chunker.Chunk(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	assertSameChunks(t, got, want)
}

func TestChunkReaderEmpty(t *testing.T) {
	chunker := newDefaultChunker(t)
	chunks, err := chunker.Chunk(bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 0 {
		t.Fatalf("got %d chunks for empty input, want none", len(chunks))
	}
}

func TestChunkReaderHandlesShortReads(t *testing.T) {
	chunker := newDefaultChunker(t)
	data := randomData(2*DefaultMaxSize+117, 8)
	want := chunker.ChunkBytes(data)

	got, err := chunker.Chunk(&shortReader{reader: bytes.NewReader(data), maxRead: 37})
	if err != nil {
		t.Fatal(err)
	}
	assertSameChunks(t, got, want)
}

func TestChunkReaderReportsReadErrors(t *testing.T) {
	chunker := newDefaultChunker(t)
	sentinel := errors.New("read failed")
	got, err := chunker.Chunk(errorReader{err: sentinel})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want %v", err, sentinel)
	}
	if len(got) != 0 {
		t.Fatalf("got %d chunks, want none", len(got))
	}
}

func TestChunkReaderRejectsNilReader(t *testing.T) {
	chunker := newDefaultChunker(t)
	if _, err := chunker.Chunk(nil); err == nil {
		t.Fatal("Chunk(nil) error = nil, want error")
	}
}

func TestGearTableDeterministicAnd48Bit(t *testing.T) {
	first := generateGearTable(123)
	second := generateGearTable(123)
	if first != second {
		t.Fatal("gear table generation is not deterministic")
	}
	for i, value := range first {
		if value > hashMask {
			t.Fatalf("entry %d exceeds the 48-bit hash mask: %#x", i, value)
		}
	}
}

func assertChunks(t *testing.T, data []byte, chunks []Chunk, minSize, maxSize int) {
	t.Helper()
	var offset int64
	for i, chunk := range chunks {
		if chunk.Offset != offset {
			t.Fatalf("chunk %d offset = %d, want %d", i, chunk.Offset, offset)
		}
		if chunk.Length != len(chunk.Data) {
			t.Fatalf("chunk %d length = %d, data length = %d", i, chunk.Length, len(chunk.Data))
		}
		if chunk.Length > maxSize {
			t.Fatalf("chunk %d length %d exceeds maximum %d", i, chunk.Length, maxSize)
		}
		if len(data) >= minSize && chunk.Length < minSize {
			t.Fatalf("chunk %d length %d is below minimum %d", i, chunk.Length, minSize)
		}
		if !bytes.Equal(chunk.Data, data[offset:offset+int64(chunk.Length)]) {
			t.Fatalf("chunk %d data does not match input at offset %d", i, offset)
		}
		offset += int64(chunk.Length)
	}
	if offset != int64(len(data)) {
		t.Fatalf("chunks cover %d bytes, input has %d", offset, len(data))
	}
}

func assertSameChunks(t *testing.T, got, want []Chunk) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("chunk counts differ: %d != %d", len(got), len(want))
	}
	for i := range got {
		if got[i].Offset != want[i].Offset ||
			got[i].Length != want[i].Length ||
			got[i].Fingerprint != want[i].Fingerprint ||
			got[i].Hash != want[i].Hash ||
			!bytes.Equal(got[i].Data, want[i].Data) {
			t.Fatalf("chunk %d differs between reader and byte APIs", i)
		}
	}
}

func randomData(size int, seed int64) []byte {
	data := make([]byte, size)
	_, _ = rand.New(rand.NewSource(seed)).Read(data)
	return data
}

func chunkLengths(chunks []Chunk) []int {
	lengths := make([]int, len(chunks))
	for i, chunk := range chunks {
		lengths[i] = chunk.Length
	}
	return lengths
}

type shortReader struct {
	reader  io.Reader
	maxRead int
}

func (r *shortReader) Read(p []byte) (int, error) {
	if len(p) > r.maxRead {
		p = p[:r.maxRead]
	}
	return r.reader.Read(p)
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}
