package browser

// mozlz4.go encodes and decodes Firefox's mozLz40-wrapped jsonlz4 session files.

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/pierrec/lz4/v4"
)

var mozillaLZ4Magic = []byte("mozLz40\x00")

func decodeMozillaLZ4File(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) < 12 || string(raw[:8]) != string(mozillaLZ4Magic) {
		return nil, fmt.Errorf("%s: not a mozilla jsonlz4 file", path)
	}

	size := int(binary.LittleEndian.Uint32(raw[8:12]))
	out := make([]byte, size)
	n, err := lz4.UncompressBlock(raw[12:], out)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return out[:n], nil
}

func encodeMozillaLZ4File(path string, data []byte) error {
	dst := make([]byte, lz4.CompressBlockBound(len(data)))
	n, err := lz4.CompressBlock(data, dst, nil)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if n <= 0 {
		return fmt.Errorf("encode %s: lz4 returned no compressed data", path)
	}

	out := make([]byte, 12+n)
	copy(out[:8], mozillaLZ4Magic)
	binary.LittleEndian.PutUint32(out[8:12], uint32(len(data)))
	copy(out[12:], dst[:n])
	return os.WriteFile(path, out, 0o644)
}
