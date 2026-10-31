package chunker

import (
	"bytes"
	"math/rand"
	"os"
	"testing"
)

func BenchmarkChunkBytes(b *testing.B) {
	data := benchmarkData()
	chunker, err := New(DefaultConfig())
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunker.ChunkBytes(data)
	}
}

func BenchmarkChunkReader(b *testing.B) {
	data := benchmarkData()
	chunker, err := New(DefaultConfig())
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := chunker.Chunk(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkChunkBytes1GiB(b *testing.B) {
	if os.Getenv("AEGIS_CHUNKER_BENCH_1G") != "1" {
		b.Skip("set AEGIS_CHUNKER_BENCH_1G=1 to allocate the 1 GiB benchmark input")
	}

	data := benchmarkInput(1 << 30)
	chunker, err := New(DefaultConfig())
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunker.ChunkBytes(data)
	}
}

func benchmarkData() []byte {
	return benchmarkInput(8 * 1024 * 1024)
}

func benchmarkInput(size int) []byte {
	data := make([]byte, size)
	_, _ = rand.New(rand.NewSource(42)).Read(data)
	return data
}
