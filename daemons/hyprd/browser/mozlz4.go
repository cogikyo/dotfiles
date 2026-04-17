package browser

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/pierrec/lz4/v4"
)

// mozLz40 header layout:
//   [0:8]   magic bytes "mozLz40\0"
//   [8:12]  little-endian uint32 decompressed size
//   [12:]   raw LZ4 block (not framed) — use UncompressBlock/CompressBlock, NOT the streaming reader/writer.
var mozillaLZ4Magic = []byte("mozLz40\x00")

// decodeMozillaLZ4File reads a mozLz40-wrapped LZ4 block from path and returns the decompressed payload.
//
// Firefox uses this wrapping for sessionstore.jsonlz4, recovery.jsonlz4, and similar files.
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

// encodeMozillaLZ4File writes data to path wrapped in the mozLz40 header Firefox expects.
//
// Used to produce session files during exact restore.
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
