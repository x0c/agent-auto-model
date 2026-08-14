package ws

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Conn 已完成握手的 WebSocket 连接（服务端视角，写出帧不掩码）。
type Conn struct {
	nc net.Conn
	r  *bufio.Reader
}

// Handshake 在已接受的 Unix/TCP 连接上完成 HTTP Upgrade。
func Handshake(nc net.Conn) (*Conn, error) {
	r := bufio.NewReader(nc)
	var lines []string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 || !strings.Contains(strings.ToUpper(lines[0]), "GET ") {
		return nil, fmt.Errorf("websocket handshake: 非法请求行")
	}
	var key string
	for _, line := range lines[1:] {
		name, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "Sec-WebSocket-Key") {
			key = strings.TrimSpace(val)
		}
	}
	if key == "" {
		return nil, errors.New("websocket handshake: 缺少 Sec-WebSocket-Key")
	}
	sum := sha1.Sum([]byte(key + guid))
	accept := base64.StdEncoding.EncodeToString(sum[:])
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := nc.Write([]byte(resp)); err != nil {
		return nil, err
	}
	return &Conn{nc: nc, r: r}, nil
}

// ReadText 读取下一帧文本。ping 自动回复 pong；close 返回 io.EOF。
func (c *Conn) ReadText() (string, error) {
	for {
		opcode, payload, err := c.readFrame()
		if err != nil {
			return "", err
		}
		switch opcode {
		case 1, 2:
			return string(payload), nil
		case 8:
			_ = c.writeFrame(8, payload)
			return "", io.EOF
		case 9:
			_ = c.writeFrame(10, payload)
		case 10:
			// pong
		default:
			continue
		}
	}
}

// WriteText 发送文本帧。
func (c *Conn) WriteText(s string) error {
	return c.writeFrame(1, []byte(s))
}

// Close 发送 close 并关闭底层连接。
func (c *Conn) Close() error {
	_ = c.writeFrame(8, nil)
	return c.nc.Close()
}

func (c *Conn) readFrame() (byte, []byte, error) {
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(c.r, hdr); err != nil {
		return 0, nil, err
	}
	opcode := hdr[0] & 0x0f
	masked := hdr[1]&0x80 != 0
	ln := uint64(hdr[1] & 0x7f)
	switch ln {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(c.r, ext[:]); err != nil {
			return 0, nil, err
		}
		ln = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(c.r, ext[:]); err != nil {
			return 0, nil, err
		}
		ln = binary.BigEndian.Uint64(ext[:])
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(c.r, mask[:]); err != nil {
			return 0, nil, err
		}
	}
	if ln > 16<<20 {
		return 0, nil, fmt.Errorf("websocket 帧过大：%d", ln)
	}
	payload := make([]byte, ln)
	if ln > 0 {
		if _, err := io.ReadFull(c.r, payload); err != nil {
			return 0, nil, err
		}
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return opcode, payload, nil
}

func (c *Conn) writeFrame(opcode byte, payload []byte) error {
	n := len(payload)
	var hdr []byte
	hdr = append(hdr, 0x80|opcode)
	if n < 126 {
		hdr = append(hdr, byte(n))
	} else if n < 65536 {
		hdr = append(hdr, 126, byte(n>>8), byte(n))
	} else {
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(n))
		hdr = append(hdr, 127)
		hdr = append(hdr, ext[:]...)
	}
	if _, err := c.nc.Write(hdr); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	_, err := c.nc.Write(payload)
	return err
}
