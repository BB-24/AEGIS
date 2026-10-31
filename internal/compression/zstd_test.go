package compression

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"testing"
)

func TestZstdRoundTrip(t *testing.T) {
	compressor, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	input := bytes.Repeat([]byte("aegis backup metadata and repeated block data\n"), 4096)
	compressed, err := compressor.Compress(input)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := compressor.Decompress(compressed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatal("decompressed data does not match original")
	}
	if len(compressed)*2 > len(input) {
		t.Fatalf("compression ratio below 2:1: input=%d compressed=%d", len(input), len(compressed))
	}
}

func TestZstdEmptyRoundTrip(t *testing.T) {
	compressor, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	frame, err := compressor.Compress(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) == 0 {
		t.Fatal("Compress returned no zstd frame for empty input")
	}
	decoded, err := compressor.Decompress(frame)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 0 {
		t.Fatalf("decompressed empty input to %d bytes", len(decoded))
	}
	n, err := compressor.DecompressFromReader(bytes.NewReader(frame), nil)
	if err != nil || n != 0 {
		t.Fatalf("DecompressFromReader(empty) = (%d, %v), want (0, nil)", n, err)
	}
}

func TestZstdWriterAndReader(t *testing.T) {
	compressor, err := New(Config{Level: 3})
	if err != nil {
		t.Fatal(err)
	}
	input := bytes.Repeat([]byte("stream frame with content size\n"), 100)
	var frame bytes.Buffer
	if err := compressor.CompressToWriter(input, &frame); err != nil {
		t.Fatal(err)
	}
	output := make([]byte, len(input))
	n, err := compressor.DecompressFromReader(bytes.NewReader(frame.Bytes()), output)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(input) || !bytes.Equal(output, input) {
		t.Fatalf("decoded %d bytes; want %d matching bytes", n, len(input))
	}
	if _, err := compressor.DecompressFromReader(bytes.NewReader(frame.Bytes()), make([]byte, 1)); err == nil {
		t.Fatal("reader decompression truncated oversized output")
	}
}

func TestZstdDictionaryRoundTrip(t *testing.T) {
	samples := make([][]byte, 64)
	for i := range samples {
		var sample bytes.Buffer
		for record := 0; record < 128; record++ {
			_, _ = fmt.Fprintf(&sample, "snapshot=weekly namespace=production object=%04d node=storage-%d\n", i*128+record, record%8)
		}
		samples[i] = sample.Bytes()
	}
	dictionary, err := TrainDictionary(samples, 42, 8192)
	if err != nil {
		t.Fatal(err)
	}
	compressor, err := New(Config{Dictionary: dictionary})
	if err != nil {
		t.Fatal(err)
	}
	input := bytes.Repeat(samples[0], 128)
	compressed, err := compressor.Compress(input)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := compressor.Decompress(compressed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatal("dictionary roundtrip changed the input")
	}

	withoutDictionary, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	var baselineSize, dictionarySize int
	for i := 0; i < 128; i++ {
		record := []byte(fmt.Sprintf("snapshot=weekly namespace=production object=%04d node=storage-%d\n", 10000+i, i%8))
		baselineFrame, err := withoutDictionary.Compress(record)
		if err != nil {
			t.Fatal(err)
		}
		dictionaryFrame, err := compressor.Compress(record)
		if err != nil {
			t.Fatal(err)
		}
		baselineSize += len(baselineFrame)
		dictionarySize += len(dictionaryFrame)
	}
	if dictionarySize >= baselineSize {
		t.Fatalf("trained dictionary did not improve small-record compression: dictionary=%d baseline=%d", dictionarySize, baselineSize)
	}
}

func TestZstdRejectsInvalidConfiguration(t *testing.T) {
	if _, err := New(Config{Level: 20}); err == nil {
		t.Fatal("New accepted an out-of-range compression level")
	}
	if _, err := TrainDictionary(nil, 1, 4096); err == nil {
		t.Fatal("TrainDictionary accepted no samples")
	}
}

func TestZstdConcurrentRoundTrips(t *testing.T) {
	compressor, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 12
	var group sync.WaitGroup
	errors := make(chan error, workers)
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			input := bytes.Repeat([]byte{byte(worker + 1)}, 4096+worker)
			frame, err := compressor.Compress(input)
			if err != nil {
				errors <- err
				return
			}
			decoded, err := compressor.Decompress(frame)
			if err != nil {
				errors <- err
				return
			}
			if !bytes.Equal(decoded, input) {
				errors <- fmt.Errorf("worker %d got mismatched decompressed data", worker)
			}
		}(worker)
	}
	group.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func BenchmarkZstdCompress64KiB(b *testing.B) {
	compressor, err := New(Config{})
	if err != nil {
		b.Fatal(err)
	}
	input := benchmarkInput()
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := compressor.Compress(input); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkZstdCompress64KiBRandom(b *testing.B) {
	compressor, err := New(Config{})
	if err != nil {
		b.Fatal(err)
	}
	input := benchmarkRandomInput()
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := compressor.Compress(input); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkZstdDecompress64KiB(b *testing.B) {
	compressor, err := New(Config{})
	if err != nil {
		b.Fatal(err)
	}
	input := benchmarkInput()
	frame, err := compressor.Compress(input)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := compressor.Decompress(frame); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkZstdDecompress64KiBRandom(b *testing.B) {
	compressor, err := New(Config{})
	if err != nil {
		b.Fatal(err)
	}
	input := benchmarkRandomInput()
	frame, err := compressor.Compress(input)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := compressor.Decompress(frame); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkInput() []byte {
	payload := []byte("aegis backup payload pattern\n")
	input := bytes.Repeat(payload, (64*1024+len(payload)-1)/len(payload))
	return input[:64*1024]
}

func benchmarkRandomInput() []byte {
	input := make([]byte, 64*1024)
	random := rand.New(rand.NewSource(42))
	_, _ = random.Read(input)
	return input
}
