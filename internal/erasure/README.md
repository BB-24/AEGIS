# Reed-Solomon erasure coding

The package encodes input into six data shards and three parity shards using
`github.com/klauspost/reedsolomon`. Any six valid shards can recover the
original input. `NewEncoder` returns the fixed `(6,3)` encoder; `Encode`
produces exactly nine self-describing shard frames, and `Decode` accepts the
nine slots in any order, with nil entries for missing shards.

## Shard frame format

The issue's proposed 32-byte header cannot contain its listed 32-byte SHA-256
hash plus the remaining metadata. AEGIS therefore uses a fixed 64-byte
big-endian header, followed by the shard payload:

| Offset | Size | Field |
|---:|---:|---|
| 0 | 4 | Magic (`AEGI`) |
| 4 | 1 | Format version (`1`) |
| 5 | 1 | Shard index (`0` through `8`) |
| 6 | 1 | Total shards (`9`) |
| 7 | 1 | Data shards (`6`) |
| 8 | 1 | Parity shards (`3`) |
| 9 | 3 | Reserved; must be zero |
| 12 | 32 | SHA-256 of the original encoded input |
| 44 | 4 | CRC32C (Castagnoli) of the payload |
| 48 | 8 | Original input length |
| 56 | 4 | Payload length |
| 60 | 4 | Reserved; must be zero |
| 64 | variable | Shard payload |

Original input length is necessary to remove padding after reconstruction.
`ParseShard` validates framing and CRC32C. During `Decode`, a missing or
checksum-invalid frame is treated as an unavailable shard; decode succeeds if
at least six valid shards remain. The reconstructed bytes are also checked
against the stored SHA-256 hash. CRC32C is for accidental corruption
detection, not authenticity; inputs should be encrypted/authenticated before
erasure encoding.

The API operates on byte slices rather than streams. It is intended for AEGIS
encrypted chunks (currently bounded by the chunker to 256 KiB), so each
operation holds only the input and nine comparatively small payloads in
memory. It is not a streaming codec for arbitrarily large objects.

Run package tests and benchmarks:

```sh
go test ./internal/erasure/...
go test -run=^$ -bench=ReedSolomon -benchmem ./internal/erasure
```

On an 11th Gen Intel Core i5-1155G7 with Go's Windows/amd64 toolchain, a
local 3-second benchmark run measured approximately 505 MB/s encode and
715 MB/s decode at 256 KiB. Results vary with CPU and power state; decode did
not reach the issue's 800 MB/s target on this machine.
