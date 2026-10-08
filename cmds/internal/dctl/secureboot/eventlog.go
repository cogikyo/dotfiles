package secureboot

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	evNoAction     = 0x3
	evVariableConf = 0x80000001
	evDriver       = 0x80000004
	algSHA256      = 0x000b
)

var (
	errTruncated = errors.New("TPM event log is truncated")
	specID       = []byte("Spec ID Event03\x00")
	globalGUID   = []byte{0x61, 0xdf, 0xe4, 0x8b, 0xca, 0x93, 0xd2, 0x11, 0xaa, 0x0d, 0x00, 0xe0, 0x98, 0x03, 0x2b, 0x8c}
)

type eventlog struct {
	oproms int
	pk     bool
}

type cursor struct {
	b   []byte
	err error
}

func (c *cursor) next(n int) []byte {
	if c.err != nil || n < 0 || n > len(c.b) {
		c.err = errTruncated
		return nil
	}
	out := c.b[:n:n]
	c.b = c.b[n:]
	return out
}

func (c *cursor) u16() uint16 {
	if b := c.next(2); b != nil {
		return binary.LittleEndian.Uint16(b)
	}
	return 0
}

func (c *cursor) u32() uint32 {
	if b := c.next(4); b != nil {
		return binary.LittleEndian.Uint32(b)
	}
	return 0
}

func parseLog(b []byte) (eventlog, error) {
	var log eventlog
	c := cursor{b: b}
	c.next(4)
	kind := c.u32()
	c.next(20)
	spec := c.next(int(c.u32()))
	if c.err != nil || kind != evNoAction || !bytes.HasPrefix(spec, specID) {
		return log, errors.New("TPM event log is not in the crypto-agile format")
	}
	s := cursor{b: spec[len(specID):]}
	s.next(8)
	sizes := map[uint16]int{}
	for n := s.u32(); n > 0 && s.err == nil; n-- {
		alg := s.u16()
		sizes[alg] = int(s.u16())
	}
	if s.err != nil {
		return log, fmt.Errorf("TPM event log Spec ID event: %w", s.err)
	}
	seen := map[string]bool{}
	for len(c.b) > 0 {
		c.next(4)
		kind := c.u32()
		var sum []byte
		for n := c.u32(); n > 0 && c.err == nil; n-- {
			alg := c.u16()
			size, ok := sizes[alg]
			if !ok {
				return log, fmt.Errorf("TPM event log: digest algorithm %#04x is not in the Spec ID event", alg)
			}
			if d := c.next(size); alg == algSHA256 {
				sum = d
			}
		}
		data := c.next(int(c.u32()))
		if c.err != nil {
			return log, c.err
		}
		switch kind {
		case evDriver:
			if !seen[string(sum)] {
				seen[string(sum)] = true
				log.oproms++
			}
		case evVariableConf:
			log.pk = log.pk || platformKey(data)
		}
	}
	return log, nil
}

func platformKey(data []byte) bool {
	if len(data) < 36 || !bytes.Equal(data[:16], globalGUID) {
		return false
	}
	return binary.LittleEndian.Uint64(data[16:]) == 2 && binary.LittleEndian.Uint64(data[24:]) > 0 && string(data[32:36]) == "P\x00K\x00"
}
