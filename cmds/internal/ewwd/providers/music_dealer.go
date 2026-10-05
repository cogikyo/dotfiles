package providers

// music_dealer.go is a minimal RFC 6455 client for Spotify's dealer push socket.
import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
)

const (
	dealerMaxMessage = 16 << 20
	opText           = 0x1
	opClose          = 0x8
	opPing           = 0x9
	opPong           = 0xa
)

type dealerSocket struct {
	conn   io.ReadWriteCloser
	reader *bufio.Reader
	write  sync.Mutex
	close  sync.Once
}

func dialDealer(ctx context.Context, rawURL string) (*dealerSocket, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", connectUserAgent)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	var key [16]byte
	_, _ = rand.Read(key[:])
	req.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString(key[:]))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if urlErr, ok := errors.AsType[*url.Error](err); ok {
			err = urlErr.Err
		}
		return nil, fmt.Errorf("dealer connect: %w", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		defer resp.Body.Close()
		return nil, newSpotifyStatusError("dealer connect", resp)
	}
	conn, ok := resp.Body.(io.ReadWriteCloser)
	if !ok {
		resp.Body.Close()
		return nil, errors.New("dealer connect: upgraded body is not writable")
	}
	return &dealerSocket{conn: conn, reader: bufio.NewReader(conn)}, nil
}

func (s *dealerSocket) Close() {
	s.close.Do(func() {
		_ = s.send(opClose, binary.BigEndian.AppendUint16(nil, 1000))
		_ = s.conn.Close()
	})
}

func (s *dealerSocket) WriteText(data []byte) error { return s.send(opText, data) }

func (s *dealerSocket) send(opcode byte, payload []byte) error {
	frame := []byte{0x80 | opcode}
	switch size := len(payload); {
	case size < 126:
		frame = append(frame, 0x80|byte(size))
	case size <= 0xffff:
		frame = binary.BigEndian.AppendUint16(append(frame, 0x80|126), uint16(size))
	default:
		frame = binary.BigEndian.AppendUint64(append(frame, 0x80|127), uint64(size))
	}
	var mask [4]byte
	_, _ = rand.Read(mask[:])
	frame = append(frame, mask[:]...)
	for i, b := range payload {
		frame = append(frame, b^mask[i%4])
	}
	s.write.Lock()
	defer s.write.Unlock()
	_, err := s.conn.Write(frame)
	return err
}

func (s *dealerSocket) ReadMessage() ([]byte, error) {
	var message []byte
	for {
		var head [2]byte
		if _, err := io.ReadFull(s.reader, head[:]); err != nil {
			return nil, err
		}
		final, opcode, size := head[0]&0x80 != 0, head[0]&0x0f, uint64(head[1]&0x7f)
		switch size {
		case 126:
			var ext [2]byte
			if _, err := io.ReadFull(s.reader, ext[:]); err != nil {
				return nil, err
			}
			size = uint64(binary.BigEndian.Uint16(ext[:]))
		case 127:
			var ext [8]byte
			if _, err := io.ReadFull(s.reader, ext[:]); err != nil {
				return nil, err
			}
			size = binary.BigEndian.Uint64(ext[:])
		}
		var mask []byte
		if head[1]&0x80 != 0 {
			mask = make([]byte, 4)
			if _, err := io.ReadFull(s.reader, mask); err != nil {
				return nil, err
			}
		}
		if size > dealerMaxMessage-uint64(len(message)) {
			return nil, errors.New("dealer message too large")
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(s.reader, payload); err != nil {
			return nil, err
		}
		if mask != nil {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
		switch opcode {
		case opClose:
			return nil, io.EOF
		case opPing:
			if err := s.send(opPong, payload); err != nil {
				return nil, err
			}
			continue
		case opPong:
			continue
		}
		message = append(message, payload...)
		if final {
			return message, nil
		}
	}
}
