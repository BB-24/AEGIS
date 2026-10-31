package erasure

import (
	"bytes"
	"testing"
)

func BenchmarkReedSolomon(b *testing.B) {
	for _, size := range []int{64 * 1024, 256 * 1024} {
		data := bytes.Repeat([]byte("AEGIS encrypted chunk benchmark "), (size+31)/32)[:size]
		b.Run(benchmarkSizeName(size), func(b *testing.B) {
			encoder, err := NewEncoder()
			if err != nil {
				b.Fatal(err)
			}
			b.Run("encode", func(b *testing.B) {
				b.SetBytes(int64(len(data)))
				b.ReportAllocs()
				for range b.N {
					if _, err := encoder.Encode(data); err != nil {
						b.Fatal(err)
					}
				}
			})
			shards, err := encoder.Encode(data)
			if err != nil {
				b.Fatal(err)
			}
			b.Run("decode", func(b *testing.B) {
				b.SetBytes(int64(len(data)))
				for range b.N {
					if _, err := encoder.Decode(shards); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

func benchmarkSizeName(size int) string {
	switch size {
	case 64 * 1024:
		return "64KiB"
	case 256 * 1024:
		return "256KiB"
	default:
		return "custom"
	}
}
