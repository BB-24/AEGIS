package chunker

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/bits"
)

const (
	DefaultMinSize = 16 * 1024
	DefaultAvgSize = 64 * 1024
	DefaultMaxSize = 256 * 1024
)

// Chunker divides a byte stream into content-defined chunks.
type Chunker interface {
	Chunk(reader io.Reader) ([]Chunk, error)
	ChunkBytes(data []byte) []Chunk
}

// Chunk contains chunk data, its position in the input, and content fingerprints.
// ChunkBytes returns Data as a view of its input; Chunk returns owned data.
type Chunk struct {
	Data        []byte
	Offset      int64
	Length      int
	Hash        [32]byte
	Fingerprint uint64
}

// Config controls the chunk-size bounds and boundary hash.
type Config struct {
	MinSize int
	AvgSize int
	MaxSize int
	Mask    uint64
}

// DefaultConfig returns FastCDC defaults with an average target of 64 KiB.
func DefaultConfig() Config {
	return Config{
		MinSize: DefaultMinSize,
		AvgSize: DefaultAvgSize,
		MaxSize: DefaultMaxSize,
	}
}

// FastCDC implements normalized FastCDC using a deterministic 48-bit gear hash.
type FastCDC struct {
	config    Config
	smallMask uint64
	largeMask uint64
}

var _ Chunker = (*FastCDC)(nil)

// New creates a FastCDC chunker. A zero Mask selects normalized FastCDC masks
// derived from AvgSize; a non-zero Mask is used in both normalization regions.
func New(config Config) (*FastCDC, error) {
	if config.MinSize <= 0 {
		return nil, fmt.Errorf("minimum chunk size must be positive: %d", config.MinSize)
	}
	if config.AvgSize < config.MinSize {
		return nil, fmt.Errorf("average chunk size %d is smaller than minimum %d", config.AvgSize, config.MinSize)
	}
	if config.MaxSize < config.AvgSize {
		return nil, fmt.Errorf("maximum chunk size %d is smaller than average %d", config.MaxSize, config.AvgSize)
	}
	if config.MaxSize-config.MinSize < config.MinSize {
		return nil, fmt.Errorf("maximum chunk size %d must be at least twice the minimum %d", config.MaxSize, config.MinSize)
	}
	if config.Mask&^hashMask != 0 {
		return nil, fmt.Errorf("boundary mask %#x exceeds the 48-bit gear hash", config.Mask)
	}

	smallMask, largeMask := normalizedMasks(config.AvgSize)
	if config.Mask != 0 {
		smallMask = config.Mask & hashMask
		largeMask = smallMask
	}

	return &FastCDC{
		config:    config,
		smallMask: smallMask,
		largeMask: largeMask,
	}, nil
}

// Chunk reads a stream and returns its chunks. At most MaxSize bytes are
// buffered for boundary detection at a time; returned chunk data is retained.
func (c *FastCDC) Chunk(reader io.Reader) ([]Chunk, error) {
	if reader == nil {
		return nil, errors.New("chunk reader is nil")
	}

	buffered := bufio.NewReaderSize(reader, c.config.MaxSize)
	chunks := make([]Chunk, 0)
	var offset int64

	for {
		data, err := buffered.Peek(c.config.MaxSize)
		if len(data) == 0 {
			if errors.Is(err, io.EOF) {
				return c.normalizeTail(chunks), nil
			}
			if err != nil {
				return chunks, err
			}
			continue
		}

		length, fingerprint := c.boundary(data)
		chunks = append(chunks, makeChunk(data[:length], offset, fingerprint))
		offset += int64(length)
		if _, discardErr := buffered.Discard(length); discardErr != nil {
			return chunks, discardErr
		}
	}
}

// ChunkBytes chunks data without an intermediate stream buffer.
func (c *FastCDC) ChunkBytes(data []byte) []Chunk {
	if len(data) == 0 {
		return nil
	}

	chunks := make([]Chunk, 0, 1+(len(data)-1)/c.config.AvgSize)
	var offset int64
	for len(data) > 0 {
		length, fingerprint := c.boundary(data)
		chunks = append(chunks, makeChunkView(data[:length], offset, fingerprint))
		offset += int64(length)
		data = data[length:]
	}
	return c.normalizeTail(chunks)
}

func (c *FastCDC) boundary(data []byte) (int, uint64) {
	limit := len(data)
	if limit > c.config.MaxSize {
		limit = c.config.MaxSize
	}

	var hash uint64
	for i := 0; i < limit; i++ {
		hash = (hash << 1) + defaultGear[data[i]]
		length := i + 1
		if length < c.config.MinSize {
			continue
		}

		mask := c.largeMask
		if length < c.config.AvgSize {
			mask = c.smallMask
		}
		if hash&mask == 0 {
			return length, hash & hashMask
		}
	}
	return limit, hash & hashMask
}

func normalizedMasks(avgSize int) (small, large uint64) {
	if avgSize <= 1 {
		return (uint64(1) << 1) - 1, (uint64(1) << 1) - 1
	}
	bitsForAverage := bits.Len(uint(avgSize - 1)) // #nosec G115 -- avgSize is positive after the guard above.
	if bitsForAverage < 1 {
		bitsForAverage = 1
	}
	if bitsForAverage > hashBits-1 {
		bitsForAverage = hashBits - 1
	}
	smallBits := bitsForAverage
	if avgSize < DefaultAvgSize {
		smallBits--
	} else if avgSize >= 2*DefaultAvgSize {
		smallBits++
	}
	if smallBits < 1 {
		smallBits = 1
	}

	largeBits := bitsForAverage - 1
	maximumLargeBits := bits.Len(uint(DefaultAvgSize-1)) - 1
	if largeBits > maximumLargeBits {
		largeBits = maximumLargeBits
	}
	small = (uint64(1) << smallBits) - 1
	large = (uint64(1) << largeBits) - 1
	return small, large
}

func makeChunkView(data []byte, offset int64, fingerprint uint64) Chunk {
	return Chunk{
		Data:        data,
		Offset:      offset,
		Length:      len(data),
		Hash:        sha256.Sum256(data),
		Fingerprint: fingerprint,
	}
}

func makeChunk(data []byte, offset int64, fingerprint uint64) Chunk {
	chunkData := append([]byte(nil), data...)
	return Chunk{
		Data:        chunkData,
		Offset:      offset,
		Length:      len(chunkData),
		Hash:        sha256.Sum256(chunkData),
		Fingerprint: fingerprint,
	}
}

func (c *FastCDC) normalizeTail(chunks []Chunk) []Chunk {
	if len(chunks) < 2 || chunks[len(chunks)-1].Length >= c.config.MinSize {
		return chunks
	}

	lastIndex := len(chunks) - 1
	previous := &chunks[lastIndex-1]
	last := &chunks[lastIndex]
	combinedLength := previous.Length + last.Length

	if combinedLength <= c.config.MaxSize {
		previous.Data = append(previous.Data, last.Data...)
		previous.Length = len(previous.Data)
		previous.Hash = sha256.Sum256(previous.Data)
		previous.Fingerprint = c.fingerprint(previous.Data)
		return chunks[:lastIndex]
	}

	transfer := c.config.MinSize - last.Length
	if previous.Length-transfer < c.config.MinSize {
		return chunks
	}

	previousData := previous.Data
	lastData := last.Data
	previous.Data = append([]byte(nil), previous.Data[:previous.Length-transfer]...)
	previous.Length = len(previous.Data)
	previous.Hash = sha256.Sum256(previous.Data)
	previous.Fingerprint = c.fingerprint(previous.Data)
	last.Data = append(append([]byte(nil), previousData[previous.Length:]...), lastData...)
	last.Offset = previous.Offset + int64(previous.Length)
	last.Length = len(last.Data)
	last.Hash = sha256.Sum256(last.Data)
	last.Fingerprint = c.fingerprint(last.Data)
	return chunks
}

func (c *FastCDC) fingerprint(data []byte) uint64 {
	var hash uint64
	for _, value := range data {
		hash = (hash << 1) + defaultGear[value]
	}
	return hash & hashMask
}
