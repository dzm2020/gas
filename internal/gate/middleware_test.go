package gate

import (
	"errors"
	"testing"

	"github.com/dzm2020/gas/internal/gate/protocol"
)

type noopMiddleware struct{}

func (noopMiddleware) AfterDecode(msg *protocol.Message) (*protocol.Message, error) { return msg, nil }
func (noopMiddleware) BeforeEncode(msg *protocol.Message) (*protocol.Message, error) { return msg, nil }

type errMiddleware struct{ err error }

func (e errMiddleware) AfterDecode(*protocol.Message) (*protocol.Message, error)  { return nil, e.err }
func (e errMiddleware) BeforeEncode(*protocol.Message) (*protocol.Message, error) { return nil, e.err }

type dropMiddleware struct{}

func (dropMiddleware) AfterDecode(*protocol.Message) (*protocol.Message, error)  { return nil, nil }
func (dropMiddleware) BeforeEncode(*protocol.Message) (*protocol.Message, error) { return nil, nil }

type modifyMiddleware struct{ suffix []byte }

func (m modifyMiddleware) AfterDecode(msg *protocol.Message) (*protocol.Message, error) {
	if msg == nil {
		return nil, nil
	}
	out := *msg
	out.Data = append([]byte(nil), msg.Data...)
	out.Data = append(out.Data, m.suffix...)
	return &out, nil
}
func (m modifyMiddleware) BeforeEncode(msg *protocol.Message) (*protocol.Message, error) {
	if msg == nil {
		return nil, nil
	}
	out := *msg
	out.Data = append([]byte(nil), msg.Data...)
	out.Data = append(out.Data, m.suffix...)
	return &out, nil
}

func TestRunAfterDecode_EmptyChain(t *testing.T) {
	msg := protocol.New(1, 2, []byte("hi"))
	out, err := RunAfterDecode(nil, msg)
	if err != nil {
		t.Fatalf("RunAfterDecode(nil, msg): err=%v", err)
	}
	if out != msg {
		t.Errorf("RunAfterDecode(nil, msg): want same msg, got %p", out)
	}
	out, err = RunAfterDecode([]Middleware{}, msg)
	if err != nil || out != msg {
		t.Errorf("RunAfterDecode([]Middleware{}, msg): out=%p err=%v", out, err)
	}
}

func TestRunAfterDecode_Noop(t *testing.T) {
	msg := protocol.New(1, 2, []byte("hi"))
	chain := []Middleware{noopMiddleware{}}
	out, err := RunAfterDecode(chain, msg)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if out != msg {
		t.Errorf("want same msg")
	}
}

func TestRunAfterDecode_Error(t *testing.T) {
	msg := protocol.New(1, 2, nil)
	wantErr := errors.New("after decode error")
	chain := []Middleware{noopMiddleware{}, errMiddleware{wantErr}}
	out, err := RunAfterDecode(chain, msg)
	if err != wantErr {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
	if out != nil {
		t.Errorf("out want nil, got %v", out)
	}
}

func TestRunAfterDecode_NilMessage(t *testing.T) {
	chain := []Middleware{dropMiddleware{}}
	out, err := RunAfterDecode(chain, protocol.New(0, 0, nil))
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if out != nil {
		t.Errorf("out want nil (dropped), got %v", out)
	}
}

func TestRunAfterDecode_ChainOrder(t *testing.T) {
	msg := protocol.New(0, 0, []byte("a"))
	chain := []Middleware{
		modifyMiddleware{suffix: []byte("b")},
		modifyMiddleware{suffix: []byte("c")},
	}
	out, err := RunAfterDecode(chain, msg)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if string(out.Data) != "abc" {
		t.Errorf("Data=%q, want \"abc\"", out.Data)
	}
}

func TestRunAfterDecode_SkipsNilMiddleware(t *testing.T) {
	msg := protocol.New(1, 2, []byte("x"))
	chain := []Middleware{nil, noopMiddleware{}, nil}
	out, err := RunAfterDecode(chain, msg)
	if err != nil || out != msg {
		t.Errorf("out=%p err=%v", out, err)
	}
}

func TestRunBeforeEncode_EmptyChain(t *testing.T) {
	msg := protocol.New(1, 2, []byte("hi"))
	out, err := RunBeforeEncode(nil, msg)
	if err != nil || out != msg {
		t.Errorf("out=%p err=%v", out, err)
	}
}

func TestRunBeforeEncode_Error(t *testing.T) {
	msg := protocol.New(1, 2, nil)
	wantErr := errors.New("before encode error")
	chain := []Middleware{errMiddleware{wantErr}}
	out, err := RunBeforeEncode(chain, msg)
	if err != wantErr || out != nil {
		t.Errorf("err=%v out=%v", err, out)
	}
}

func TestRunBeforeEncode_ChainOrder(t *testing.T) {
	msg := protocol.New(0, 0, []byte("1"))
	chain := []Middleware{
		modifyMiddleware{suffix: []byte("2")},
		modifyMiddleware{suffix: []byte("3")},
	}
	out, err := RunBeforeEncode(chain, msg)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if string(out.Data) != "123" {
		t.Errorf("Data=%q, want \"123\"", out.Data)
	}
}
