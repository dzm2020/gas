package session

import (
	"bytes"
	"encoding/base64"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/pb"
	"github.com/dzm2020/gas/pkg/lib"
)

// ---------- mock IContext for transport (captures InvokerMessage) ----------

type captureContext struct {
	pid     *iface.Pid
	lastMsg *iface.ActorMessage
	mu      sync.Mutex
}

func (c *captureContext) ID() *iface.Pid { return c.pid }
func (c *captureContext) InvokerMessage(msg interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if m, ok := msg.(*iface.ActorMessage); ok {
		c.lastMsg = m
	}
	return nil
}
func (c *captureContext) Serializer() lib.ISerializer     { return nil }
func (c *captureContext) Message() *iface.ActorMessage   { return nil }
func (c *captureContext) Actor() iface.IActor             { return nil }
func (c *captureContext) Process() iface.IProcess         { return nil }
func (c *captureContext) System() iface.ISystem           { return nil }
func (c *captureContext) Named(string) error             { return nil }
func (c *captureContext) Unname() error                  { return nil }
func (c *captureContext) GetName() string                { return "" }
func (c *captureContext) SetCallTimeout(time.Duration)   {}
func (c *captureContext) Send(*iface.Pid, string, interface{}) error { return nil }
func (c *captureContext) Call(*iface.Pid, string, interface{}, interface{}) error { return nil }
func (c *captureContext) Forward(*iface.Pid, string) error { return nil }
func (c *captureContext) AfterFunc(time.Duration, iface.Task) *lib.Timer { return nil }
func (c *captureContext) Shutdown() error                { return nil }

func (c *captureContext) getLastMsg() *iface.ActorMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastMsg
}

// ---------- tests ----------

func TestSession_New_Values(t *testing.T) {
	pid := &iface.Pid{NodeId: 1, ActorId: 1}
	raw := &pb.Session{Id: 10, Agent: pid, Values: map[string]string{}}
	ctx := &captureContext{pid: pid}
	s := New(raw, ctx)
	if s.GetString("x") != "" {
		t.Error("GetString empty key should return empty")
	}
	s.SetString("k", "v")
	if s.GetString("k") != "v" {
		t.Errorf("GetString want v, got %q", s.GetString("k"))
	}
	s.SetUint64("u", 42)
	if s.GetUint64("u") != 42 {
		t.Errorf("GetUint64 want 42, got %d", s.GetUint64("u"))
	}
	s.SetInt64("i", -1)
	if s.GetInt64("i") != -1 {
		t.Errorf("GetInt64 want -1, got %d", s.GetInt64("i"))
	}
}

func TestSession_Response(t *testing.T) {
	pid := &iface.Pid{NodeId: 1, ActorId: 1}
	raw := &pb.Session{Id: 1, Agent: pid, Values: map[string]string{}}
	ctx := &captureContext{pid: pid}
	s := New(raw, ctx)
	s.SetMessage(protocol.New(0, 0, []byte("req")))
	err := s.Response([]byte("resp"))
	if err != nil {
		t.Fatalf("Response: %v", err)
	}
	msg := ctx.getLastMsg()
	if msg == nil || msg.GetMethod() != MethodPush {
		t.Fatalf("InvokerMessage should be called with Method Push, got %v", msg)
	}
	dec, _, _ := codec.Decode(msg.GetData())
	if dec == nil || !bytes.Equal(dec.Data, []byte("resp")) {
		t.Errorf("Response data want 'resp', got %q", dec.Data)
	}
}

func TestSession_ResponseErr(t *testing.T) {
	pid := &iface.Pid{NodeId: 1, ActorId: 1}
	raw := &pb.Session{Id: 1, Agent: pid, Values: map[string]string{}}
	ctx := &captureContext{pid: pid}
	s := New(raw, ctx)
	s.SetMessage(protocol.New(0, 0, nil))
	err := s.ResponseErr(500)
	if err != nil {
		t.Fatalf("ResponseErr: %v", err)
	}
	msg := ctx.getLastMsg()
	if msg == nil || msg.GetMethod() != MethodPush {
		t.Fatalf("want Method Push, got %v", msg)
	}
	dec, _, _ := codec.Decode(msg.GetData())
	if dec == nil || dec.Error != 500 {
		t.Errorf("ResponseErr code want 500, got %d", dec.Error)
	}
}

func TestSession_Push(t *testing.T) {
	pid := &iface.Pid{NodeId: 1, ActorId: 1}
	raw := &pb.Session{Id: 1, Agent: pid, Values: map[string]string{}}
	ctx := &captureContext{pid: pid}
	s := New(raw, ctx)
	err := s.Push(1, 2, []byte("body"))
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	msg := ctx.getLastMsg()
	if msg == nil || msg.GetMethod() != MethodPush {
		t.Fatalf("want Method Push, got %v", msg)
	}
	dec, _, _ := codec.Decode(msg.GetData())
	if dec == nil || dec.Cmd != 1 || dec.Act != 2 || !bytes.Equal(dec.Data, []byte("body")) {
		t.Errorf("Push Cmd/Act/Data want 1/2/body, got %d/%d/%q", dec.Cmd, dec.Act, dec.Data)
	}
}

func TestSession_Close(t *testing.T) {
	pid := &iface.Pid{NodeId: 1, ActorId: 1}
	raw := &pb.Session{Id: 1, Agent: pid, Values: map[string]string{}}
	ctx := &captureContext{pid: pid}
	s := New(raw, ctx)
	err := s.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	msg := ctx.getLastMsg()
	if msg == nil || msg.GetMethod() != MethodShutDown {
		t.Fatalf("want Method Shutdown, got %v", msg)
	}
}

func TestSession_SyncValues(t *testing.T) {
	pid := &iface.Pid{NodeId: 1, ActorId: 1}
	raw := &pb.Session{Id: 1, Agent: pid, Values: map[string]string{}}
	ctx := &captureContext{pid: pid}
	s := New(raw, ctx)
	s.SetString("a", "1")
	err := s.SyncValues()
	if err != nil {
		t.Fatalf("SyncValues: %v", err)
	}
	msg := ctx.getLastMsg()
	if msg == nil || msg.GetMethod() != MethodSetValue {
		t.Fatalf("want Method SetValue, got %v", msg)
	}
	// Data is JSON map
	if len(msg.GetData()) == 0 {
		t.Error("SetValue data should be non-empty JSON")
	}
}

func TestSession_GetMessage_SetMessage_Raw(t *testing.T) {
	pid := &iface.Pid{NodeId: 1, ActorId: 1}
	raw := &pb.Session{Id: 1, Agent: pid, Values: map[string]string{}}
	ctx := &captureContext{pid: pid}
	s := New(raw, ctx)
	if s.GetMessage() != nil {
		t.Error("GetMessage before SetMessage should be nil when Values empty")
	}
	msg := protocol.New(1, 2, []byte("d"))
	s.SetMessage(msg)
	got := s.GetMessage()
	if got == nil || got.Cmd != 1 || got.Act != 2 || !bytes.Equal(got.Data, []byte("d")) {
		t.Errorf("GetMessage want Cmd=1 Act=2 Data=d, got %v", got)
	}
	rawOut := s.Raw()
	if rawOut.GetId() != 1 {
		t.Errorf("Raw Id want 1, got %d", rawOut.GetId())
	}
	val := rawOut.GetValues()[KeyMessage]
	if val == "" {
		t.Error("Raw Values should contain KeyMessage (base64 encoded)")
	}
	decoded, _ := base64.StdEncoding.DecodeString(val)
	dec, _, _ := codec.Decode(decoded)
	if dec == nil || dec.Cmd != 1 {
		t.Errorf("KeyMessage decode want Cmd=1, got %v", dec)
	}
}

func TestSession_Close_NoTransport(t *testing.T) {
	s := &Session{Session: &pb.Session{Id: 1, Values: map[string]string{}}, transport: nil}
	err := s.Close()
	if !errors.Is(err, errTransportIsNil) {
		t.Errorf("Close with nil transport want errTransportIsNil, got %v", err)
	}
}
