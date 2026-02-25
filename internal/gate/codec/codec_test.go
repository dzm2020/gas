package codec

import (
	"bytes"
	"testing"

	"github.com/dzm2020/gas/internal/gate/protocol"
)

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() should not return nil")
	}
}

func TestCodec_Encode_Decode_RoundTrip(t *testing.T) {
	c := New()
	msg := protocol.New(10, 20, []byte("hello world"))

	encoded, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded) < protocol.HeadLen {
		t.Errorf("encoded length want >= %d, got %d", protocol.HeadLen, len(encoded))
	}

	out, n, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if n != len(encoded) {
		t.Errorf("Decode consumed want %d, got %d", len(encoded), n)
	}
	if out == nil {
		t.Fatal("Decode should return non-nil *protocol.Message")
	}
	if out.Cmd != 10 || out.Act != 20 {
		t.Errorf("Cmd/Act want 10/20, got %d/%d", out.Cmd, out.Act)
	}
	if !bytes.Equal(out.Data, []byte("hello world")) {
		t.Errorf("Data want 'hello world', got %q", out.Data)
	}
}


func TestCodec_Encode_InvalidType(t *testing.T) {
	c := New()
	_, err := c.Encode("not a message")
	if err != ErrInvalidCodecMessageType {
		t.Errorf("Encode invalid type want ErrInvalidCodecMessageType, got %v", err)
	}
}

func TestCodec_Decode_ShortBuffer(t *testing.T) {
	c := New()
	// 少于 HeadLen 的缓冲应返回 (nil, 0, nil) 表示数据不完整
	short := make([]byte, protocol.HeadLen-1)
	msg, n, err := c.Decode(short)
	if msg != nil || n != 0 || err != nil {
		t.Errorf("short buffer: want (nil, 0, nil), got (%v, %d, %v)", msg, n, err)
	}
}

func TestCodec_Decode_PartialBody(t *testing.T) {
	c := New()
	// 头部完整但 body 长度不足
	buf := make([]byte, protocol.HeadLen+5)
	// Len = 100，但 buf 只有 5 字节 body
	buf[0] = 0
	buf[1] = 0
	buf[2] = 0
	buf[3] = 100
	msg, n, err := c.Decode(buf)
	if msg != nil || n != 0 || err != nil {
		t.Errorf("partial body: want (nil, 0, nil), got (%v, %d, %v)", msg, n, err)
	}
}

func TestCodec_Encode_SetsLen(t *testing.T) {
	c := New()
	data := []byte("abc")
	msg := protocol.New(1, 2, data)
	msg.Len = 999 // 会被 Encode 覆盖

	encoded, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// 前 4 字节为 BigEndian Len
	out, _, _ := c.Decode(encoded)
	if out.Len != uint32(len(data)) {
		t.Errorf("Len want %d, got %d", len(data), out.Len)
	}
}
