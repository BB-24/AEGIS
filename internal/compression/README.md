# Zstandard compression

`internal/compression` wraps `github.com/klauspost/compress/zstd`. The default
compression level is 3; levels 1 through 19 are supported. A trained zstd
dictionary may be supplied in `Config.Dictionary` and is used for both encoding
and decoding.

`TrainDictionary(samples, id, capacity)` creates a zstd-format dictionary from
representative backup samples. Dictionary IDs must be non-zero, and every
reader must be configured with the dictionary used to produce its frame.
`Compress` and `CompressToWriter` emit frames with the known content size.
`DecompressFromReader` returns an error instead of truncating when the supplied
destination buffer is too small.

Run correctness and throughput benchmarks with:

```sh
go test ./internal/compression
go test -bench='Zstd' -benchmem ./internal/compression
```
