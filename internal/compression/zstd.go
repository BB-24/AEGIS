package compression

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	zstdlib "github.com/klauspost/compress/zstd"
)

const DefaultLevel = 3

// Config controls zstd compression and an optional trained zstd dictionary.
type Config struct {
	Level      int
	Dictionary []byte
}

// Compressor compresses and decompresses chunk data.
type Compressor interface {
	Compress(src []byte) ([]byte, error)
	Decompress(src []byte) ([]byte, error)
	CompressToWriter(src []byte, dst io.Writer) error
	DecompressFromReader(src io.Reader, dst []byte) (int, error)
}

// Zstd is a zstd compressor configured for chunk-sized payloads.
type Zstd struct {
	level      zstdlib.EncoderLevel
	dictionary []byte
	encoder    *zstdlib.Encoder
	decoder    *zstdlib.Decoder
}

var _ Compressor = (*Zstd)(nil)

// New creates a zstd compressor at level 3 by default. Supported levels are 1-19.
func New(config Config) (*Zstd, error) {
	level := config.Level
	if level == 0 {
		level = DefaultLevel
	}
	if level < 1 || level > 19 {
		return nil, fmt.Errorf("zstd level must be between 1 and 19: %d", level)
	}

	compressor := &Zstd{
		level:      zstdlib.EncoderLevelFromZstd(level),
		dictionary: append([]byte(nil), config.Dictionary...),
	}
	encoder, err := zstdlib.NewWriter(nil, compressor.encoderOptions()...)
	if err != nil {
		return nil, fmt.Errorf("configure zstd encoder: %w", err)
	}
	decoder, err := zstdlib.NewReader(nil, compressor.decoderOptions()...)
	if err != nil {
		_ = encoder.Close()
		return nil, fmt.Errorf("configure zstd decoder: %w", err)
	}
	compressor.encoder = encoder
	compressor.decoder = decoder
	return compressor, nil
}

// Compress returns a zstd frame containing the source content size.
func (z *Zstd) Compress(src []byte) ([]byte, error) {
	return z.encoder.EncodeAll(src, nil), nil
}

// Decompress decodes a zstd frame, including frames that use the configured dictionary.
func (z *Zstd) Decompress(src []byte) ([]byte, error) {
	result, err := z.decoder.DecodeAll(src, nil)
	if err != nil {
		return nil, fmt.Errorf("decompress zstd frame: %w", err)
	}
	return result, nil
}

// CompressToWriter writes a complete zstd frame and records the known content size.
func (z *Zstd) CompressToWriter(src []byte, dst io.Writer) error {
	if dst == nil {
		return errors.New("zstd destination writer is nil")
	}
	encoder, err := zstdlib.NewWriter(dst, z.encoderOptions()...)
	if err != nil {
		return fmt.Errorf("create zstd encoder: %w", err)
	}
	encoder.ResetContentSize(dst, int64(len(src)))
	if _, err := encoder.Write(src); err != nil {
		_ = encoder.Close()
		return fmt.Errorf("write zstd frame: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("close zstd frame: %w", err)
	}
	return nil
}

// DecompressFromReader reads one zstd frame into dst. It errors rather than
// truncating when the decompressed content is larger than dst.
func (z *Zstd) DecompressFromReader(src io.Reader, dst []byte) (int, error) {
	if src == nil {
		return 0, errors.New("zstd source reader is nil")
	}
	decoder, err := zstdlib.NewReader(src, z.decoderOptions()...)
	if err != nil {
		return 0, fmt.Errorf("create zstd decoder: %w", err)
	}
	defer decoder.Close()

	decoded, err := io.ReadAll(io.LimitReader(decoder, int64(len(dst))+1))
	if err != nil {
		return 0, fmt.Errorf("read zstd frame: %w", err)
	}
	if len(decoded) > len(dst) {
		return 0, fmt.Errorf("decompressed content exceeds destination size %d", len(dst))
	}
	return copy(dst, decoded), nil
}

// TrainDictionary builds a zstd-format dictionary from representative samples.
func TrainDictionary(samples [][]byte, dictionaryID uint32, capacity int) (_ []byte, err error) {
	if len(samples) == 0 {
		return nil, errors.New("zstd dictionary training requires at least one sample")
	}
	if dictionaryID == 0 {
		return nil, errors.New("zstd dictionary ID must be non-zero")
	}
	if capacity < 256 {
		return nil, fmt.Errorf("zstd dictionary capacity must be at least 256 bytes: %d", capacity)
	}

	var history bytes.Buffer
	for _, sample := range samples {
		if len(sample) == 0 {
			continue
		}
		remaining := capacity - history.Len()
		if remaining <= 0 {
			break
		}
		if len(sample) > remaining {
			sample = sample[:remaining]
		}
		_, _ = history.Write(sample)
	}
	if history.Len() < 8 {
		return nil, errors.New("zstd dictionary training samples contain fewer than 8 bytes")
	}
	dictionary, err := buildDictionary(zstdlib.BuildDictOptions{
		ID:       dictionaryID,
		Contents: samples,
		History:  history.Bytes(),
		Offsets:  [3]int{1, 4, 8},
		Level:    zstdlib.EncoderLevelFromZstd(DefaultLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("train zstd dictionary: %w", err)
	}
	return dictionary, nil
}

func buildDictionary(options zstdlib.BuildDictOptions) (dictionary []byte, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			dictionary = nil
			err = fmt.Errorf("zstd dictionary trainer failed on the supplied samples: %v", recovered)
		}
	}()
	return zstdlib.BuildDict(options)
}

func (z *Zstd) encoderOptions() []zstdlib.EOption {
	options := []zstdlib.EOption{
		zstdlib.WithEncoderLevel(z.level),
		zstdlib.WithEncoderConcurrency(1),
		zstdlib.WithZeroFrames(true),
	}
	if len(z.dictionary) > 0 {
		options = append(options, zstdlib.WithEncoderDict(z.dictionary))
	}
	return options
}

func (z *Zstd) decoderOptions() []zstdlib.DOption {
	options := []zstdlib.DOption{zstdlib.WithDecoderConcurrency(1)}
	if len(z.dictionary) > 0 {
		options = append(options, zstdlib.WithDecoderDicts(z.dictionary))
	}
	return options
}
