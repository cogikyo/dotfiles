package secureboot

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLog(t *testing.T) {
	for name, want := range map[string]int{"t480s_eventlog": 0, "t14_eventlog": 7, "t14s_eventlog": 11} {
		data, err := os.ReadFile(filepath.Join("testdata", "eventlog", name))
		if err != nil {
			t.Fatal(err)
		}
		log, err := parseLog(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if log.oproms != want {
			t.Errorf("%s: %d option ROMs, want %d as sbctl counts them", name, log.oproms, want)
		}
		if !log.pk {
			t.Errorf("%s: PK measurement not found", name)
		}
		if _, err := parseLog(data[:len(data)-3]); err == nil {
			t.Errorf("%s: truncated log parsed", name)
		}
	}
	if _, err := parseLog(make([]byte, 64)); err == nil {
		t.Error("a log without a Spec ID event parsed")
	}
}

func synthetic(oproms int, pk bool) []byte {
	var b bytes.Buffer
	put := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	spec := append(append([]byte{}, specID...), make([]byte, 8)...)
	spec = binary.LittleEndian.AppendUint32(spec, 1)
	spec = binary.LittleEndian.AppendUint16(spec, algSHA256)
	spec = binary.LittleEndian.AppendUint16(spec, 32)
	spec = append(spec, 0)
	put(uint32(0))
	put(uint32(evNoAction))
	b.Write(make([]byte, 20))
	put(uint32(len(spec)))
	b.Write(spec)
	event := func(kind uint32, sum byte, data []byte) {
		put(uint32(7))
		put(kind)
		put(uint32(1))
		put(uint16(algSHA256))
		b.Write(bytes.Repeat([]byte{sum}, 32))
		put(uint32(len(data)))
		b.Write(data)
	}
	size := uint64(0)
	if pk {
		size = 4
	}
	v := append([]byte{}, globalGUID...)
	v = binary.LittleEndian.AppendUint64(v, 2)
	v = binary.LittleEndian.AppendUint64(v, size)
	v = append(v, 'P', 0, 'K', 0)
	event(evVariableConf, 0, append(v, make([]byte, size)...))
	for i := range oproms {
		event(evDriver, byte(i+1), nil)
		event(evDriver, byte(i+1), nil)
	}
	return b.Bytes()
}

func TestSyntheticLog(t *testing.T) {
	for _, tc := range []struct {
		oproms int
		pk     bool
	}{{0, false}, {3, true}} {
		log, err := parseLog(synthetic(tc.oproms, tc.pk))
		if err != nil || log.oproms != tc.oproms || log.pk != tc.pk {
			t.Errorf("synthetic(%d, %v) = %+v, %v", tc.oproms, tc.pk, log, err)
		}
	}
}
