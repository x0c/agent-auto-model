package ws

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestHandshakeAndTextRoundTrip(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	errCh := make(chan error, 1)
	connCh := make(chan *Conn, 1)
	go func() {
		nc, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		c, err := Handshake(nc)
		if err != nil {
			_ = nc.Close()
			errCh <- err
			return
		}
		connCh <- c
	}()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	key := "dGhlIHNhbXBsZSBub25jZQ=="
	req := "GET /rpc HTTP/1.1\r\n" +
		"Host: localhost\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := client.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(client)
	status, err := br.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("status=%q", status)
	}
	sum := sha1.Sum([]byte(key + guid))
	wantAccept := base64.StdEncoding.EncodeToString(sum[:])
	gotAccept := ""
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, val, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(name), "Sec-WebSocket-Accept") {
			gotAccept = strings.TrimSpace(val)
		}
	}
	if gotAccept != wantAccept {
		t.Fatalf("accept=%q want=%q", gotAccept, wantAccept)
	}

	var conn *Conn
	select {
	case err := <-errCh:
		t.Fatal(err)
	case conn = <-connCh:
	case <-time.After(3 * time.Second):
		t.Fatal("handshake timed out")
	}
	defer conn.Close()

	if err := writeMaskedText(client, `{"method":"ping"}`); err != nil {
		t.Fatal(err)
	}
	got, err := conn.ReadText()
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"method":"ping"}` {
		t.Fatalf("got=%q", got)
	}
	if err := conn.WriteText(`{"ok":true}`); err != nil {
		t.Fatal(err)
	}
	opcode, payload, err := readUnmaskedFrame(br)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != 1 || string(payload) != `{"ok":true}` {
		t.Fatalf("opcode=%d payload=%q", opcode, payload)
	}
}

func writeMaskedText(w io.Writer, s string) error {
	payload := []byte(s)
	n := len(payload)
	if n >= 126 {
		return io.ErrShortBuffer
	}
	hdr := []byte{0x81, 0x80 | byte(n)}
	mask := [4]byte{1, 2, 3, 4}
	hdr = append(hdr, mask[:]...)
	masked := make([]byte, n)
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	_, err := w.Write(masked)
	return err
}

func readUnmaskedFrame(r io.Reader) (byte, []byte, error) {
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return 0, nil, err
	}
	opcode := hdr[0] & 0x0f
	ln := uint64(hdr[1] & 0x7f)
	if ln == 126 {
		var ext [2]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return 0, nil, err
		}
		ln = uint64(binary.BigEndian.Uint16(ext[:]))
	}
	payload := make([]byte, ln)
	if ln > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, err
		}
	}
	return opcode, payload, nil
}
