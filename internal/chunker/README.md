# FastCDC chunker

This package implements normalized FastCDC with a 48-bit gear hash and SHA-256
content digests. The gear table is generated from a fixed seed using SplitMix64,
so chunk boundaries are repeatable across runs and platforms.

## Defaults

| Setting | Default |
| --- | ---: |
| Minimum chunk size | 16 KiB |
| Average target | 64 KiB |
| Maximum chunk size | 256 KiB |

```go
chunker, err := chunker.New(chunker.DefaultConfig())
if err != nil {
    return err
}

chunks, err := chunker.Chunk(reader)
if err != nil {
    return err
}
```

Set `AvgSize` to tune the target (for example, 32 KiB or 128 KiB). Set `Mask`
to use an explicit boundary mask in both normalization regions; leave it zero
to derive the two FastCDC masks from `AvgSize`.

The reader API buffers at most one maximum-sized window for boundary detection.
It returns chunk data, so callers should still account for memory proportional
to the total input size. Final chunks shorter than `MinSize` are merged or
adjusted against the preceding boundary when possible. Inputs smaller than
`MinSize` form one short chunk. `ChunkBytes` returns zero-copy slices backed by
the input; keep that input unchanged while using the returned chunk data.

## Testing and benchmarks

Run package tests with:

```sh
go test -race ./internal/chunker
```

Benchmark byte-slice and reader APIs with:

```sh
go test -bench=. -benchmem ./internal/chunker
```

Benchmarks use an 8 MiB seeded pseudo-random stream and report throughput with
Go's standard `MB/s` benchmark metric. For a throughput target of 500 MB/s,
compare the reported `MB/s` on the intended CPU and Go version; performance
varies with hardware and hash implementation.

An opt-in 1 GiB `ChunkBytes` benchmark is available for machines with enough
memory:

```sh
AEGIS_CHUNKER_BENCH_1G=1 go test -run '^$' -bench '^BenchmarkChunkBytes1GiB$' -benchtime=1x ./internal/chunker
```

On the development machine (Windows/amd64, Go 1.27.3, Intel Core i5-1155G7),
a 3-second run measured 168 MB/s for `ChunkBytes` and 134 MB/s for `Chunk`.
These figures are below the 500 MB/s target and are hardware- and load-dependent;
the target remains to be verified on the intended production CPU.
