package codec

import (
	"bytes"
	"testing"

	"github.com/dzm2020/gas/internal/component/gate/protocol"
)

func TestEncode_Decode_RoundTrip(t *testing.T) {
	msg := protocol.New(10, 20, []byte("hello world"))

	encoded, err := Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded) < protocol.HeadLen {
		t.Errorf("encoded length want >= %d, got %d", protocol.HeadLen, len(encoded))
	}

	out, n, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if n != len(encoded) {
		t.Errorf("Decode consumed want %d, got %d", len(encoded), n)
	}
	if out == nil {
		t.Fatal("Decode should return non-nil *protocol.Message")
	}
	if out.GetCmd() != 10 || out.GetAct() != 20 {
		t.Errorf("Cmd/Act want 10/20, got %d/%d", out.GetCmd(), out.GetAct())
	}
	if !bytes.Equal(out.Data, []byte("hello world")) {
		t.Errorf("Data want 'hello world', got %q", out.Data)
	}
}

func TestDecode_ShortBuffer(t *testing.T) {
	short := make([]byte, protocol.HeadLen-1)
	msg, n, err := Decode(short)
	if msg != nil || n != 0 || err != nil {
		t.Errorf("short buffer: want (nil, 0, nil), got (%v, %d, %v)", msg, n, err)
	}
}

func TestDecode_PartialBody(t *testing.T) {
	buf := make([]byte, protocol.HeadLen+5)
	buf[0] = 0
	buf[1] = 0
	buf[2] = 0
	buf[3] = 100
	msg, n, err := Decode(buf)
	if msg != nil || n != 0 || err != nil {
		t.Errorf("partial body: want (nil, 0, nil), got (%v, %d, %v)", msg, n, err)
	}
}

func TestEncode_DoesNotMutateMsgLen(t *testing.T) {
	data := []byte("abc")
	msg := protocol.New(1, 2, data)
	msg.SetLen(999)

	encoded, err := Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if msg.GetLen() != 999 {
		t.Errorf("Encode must not mutate msg.Len: want 999, got %d", msg.GetLen())
	}
	out, _, _ := Decode(encoded)
	if out.GetLen() != uint32(len(data)) {
		t.Errorf("decoded Len want %d, got %d", len(data), out.GetLen())
	}
}
