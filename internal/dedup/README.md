# Chunk deduplication index

`internal/dedup` provides a thread-safe content-addressed metadata index. Use
`Open(path)` for a BoltDB-backed index (the containing directory must exist) or
`NewMemoryIndex()` for tests and ephemeral workloads.

`Add` registers a chunk with one reference. `IncRef` and `DecRef` persist each
update; `DecRef` marks an entry `Deleted` and returns `true` when the last
reference is removed. Deleted entries are retained as tombstones and can be
replaced by a later `Add`. `Lookup` returns a defensive copy, including for
tombstones. `Stats` counts active chunks only.

Run package tests, including the race detector, with:

```sh
go test -race ./internal/dedup
```
