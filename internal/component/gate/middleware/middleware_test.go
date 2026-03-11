package middleware

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/gateiface"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/component/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/network"
	"golang.org/x/time/rate"
)

func TestRateLimit_ByConnection(t *testing.T) {
	rl := NewRateLimitForConnection(rate.Limit(2), 2)
	msg := protocol.New(1, 2, []byte("a"))
	_, err := rl.AfterDecode(nil, msg)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err = rl.AfterDecode(nil, msg)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	_, err = rl.AfterDecode(nil, msg)
	if err == nil || !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("third should be rate limited: %v", err)
	}
}

func TestRateLimit_ByMessageID(t *testing.T) {
	targetID := protocol.CmdAct(1, 0)
	rl := NewRateLimitForMessageID(rate.Limit(1), 1, targetID)
	msgLimit := protocol.New(1, 0, []byte("a"))
	msgOther := protocol.New(2, 0, []byte("b"))
	_, err := rl.AfterDecode(nil, msgLimit)
	if err != nil {
		t.Fatalf("msgLimit first: %v", err)
	}
	_, err = rl.AfterDecode(nil, msgOther)
	if err != nil {
		t.Fatalf("msgOther should pass through: %v", err)
	}
	_, err = rl.AfterDecode(nil, msgLimit)
	if err == nil || !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("msgLimit second should be limited: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	_, err = rl.AfterDecode(nil, msgLimit)
	if err != nil {
		t.Fatalf("msgLimit after sleep: %v", err)
	}
}

func TestLog_Passthrough(t *testing.T) {
	l := NewLog()
	msg := protocol.New(1, 2, []byte("x"))
	out, err := l.AfterDecode(nil, msg)
	if err != nil || out != msg {
		t.Fatalf("AfterDecode: err=%v out=%p", err, out)
	}
	out, err = l.BeforeEncode(nil, msg)
	if err != nil || out != msg {
		t.Fatalf("BeforeEncode: err=%v", err)
	}
}

func TestCompress_RoundTrip(t *testing.T) {
	c := NewCompress(0)
	msg := protocol.New(0, 0, []byte("hello world"))
	enc, err := c.BeforeEncode(nil, msg)
	if err != nil {
		t.Fatalf("BeforeEncode: %v", err)
	}
	if enc.GetTag()&TagCompressed == 0 {
		t.Error("Tag should have TagCompressed set")
	}
	dec, err := c.AfterDecode(nil, enc)
	if err != nil {
		t.Fatalf("AfterDecode: %v", err)
	}
	if dec.GetTag()&TagCompressed != 0 {
		t.Error("Tag should clear TagCompressed after decode")
	}
	if string(dec.Data) != "hello world" {
		t.Errorf("data want 'hello world', got %q", dec.Data)
	}
}

func TestCompress_SkipSmall(t *testing.T) {
	c := NewCompress(100)
	msg := protocol.New(0, 0, []byte("hi"))
	out, err := c.BeforeEncode(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	if out != msg {
		t.Error("short data should not be compressed")
	}
	if out.GetTag()&TagCompressed != 0 {
		t.Error("Tag should not have TagCompressed")
	}
}

func TestEncrypt_KeyExchange_AutoReply(t *testing.T) {
	serverEnc, err := NewEncrypt()
	if err != nil {
		t.Fatalf("NewEncrypt: %v", err)
	}
	clientKey := make([]byte, 32)
	for i := range clientKey {
		clientKey[i] = byte(i)
	}
	clientKeyMsg := protocol.New(exchangeCmd, exchangeAct, clientKey)
	var pushed [][]byte
	agent := &mockAgentForEncrypt{push: func(msg *protocol.Message) error {
		bin, _ := codec.Encode(msg)
		pushed = append(pushed, bin)
		return nil
	}}
	out, err := serverEnc.AfterDecode(agent, clientKeyMsg)
	if err != nil {
		t.Fatalf("AfterDecode: %v", err)
	}
	if out != nil {
		t.Fatalf("server should return nil msg to skip handler, got %v", out)
	}
	if len(pushed) != 1 {
		t.Fatalf("agent.Push should be called once, got %d", len(pushed))
	}
	dec, _, _ := codec.Decode(pushed[0])
	if dec == nil || dec.GetCmd() != exchangeCmd || dec.GetAct() != exchangeAct || !bytes.Equal(dec.Data, serverEnc.ServerKey()) {
		t.Errorf("pushed message want (0,0, serverKey), got cmd=%d act=%d data=%v", dec.GetCmd(), dec.GetAct(), dec.Data)
	}
}

// mockAgentForEncrypt 实现 gateiface.IAgent，仅 Push 用于 Encrypt 测试。
type mockAgentForEncrypt struct {
	push func(*protocol.Message) error
}

func (m *mockAgentForEncrypt) Context() iface.IContext                   { return nil }
func (m *mockAgentForEncrypt) GetEntity() network.IConnection            { return nil }
func (m *mockAgentForEncrypt) GetSession() *session.Session              { return nil }
func (m *mockAgentForEncrypt) SetMiddleware([]gateiface.IMiddleware)     {}
func (m *mockAgentForEncrypt) AppendMiddleware(...gateiface.IMiddleware) {}
func (m *mockAgentForEncrypt) GetMiddleware() []gateiface.IMiddleware    { return nil }
func (m *mockAgentForEncrypt) Push(msg *protocol.Message) error          { return m.push(msg) }
func (m *mockAgentForEncrypt) SetValues(map[string]string) error         { return nil }
func (m *mockAgentForEncrypt) Shutdown() error                           { return nil }

func TestEncrypt_KeyExchangeAndXOR(t *testing.T) {
	serverEnc, err := NewEncrypt()
	if err != nil {
		t.Fatalf("NewEncrypt: %v", err)
	}
	clientKey := make([]byte, 32)
	for i := range clientKey {
		clientKey[i] = byte(i + 10)
	}
	clientEnc := NewEncryptClient(clientKey)
	noOpAgent := &mockAgentForEncrypt{push: func(*protocol.Message) error { return nil }}

	// 1. 客户端发 (0,0, clientKey)
	clientKeyMsg := protocol.New(exchangeCmd, exchangeAct, clientKey)
	out, err := clientEnc.BeforeEncode(noOpAgent, clientKeyMsg)
	if err != nil || out != clientKeyMsg {
		t.Fatalf("client send key: %v", err)
	}
	// 2. 服务端收 clientKey，派生密钥并透过 agent.Push 回复 serverKey，返回 (nil,nil) 表示已处理、跳过业务
	in, err := serverEnc.AfterDecode(noOpAgent, clientKeyMsg)
	if err != nil {
		t.Fatalf("server recv clientKey: %v", err)
	}
	if in != nil {
		t.Fatalf("server key exchange should return nil msg to skip handler, got %v", in)
	}
	serverKey := serverEnc.ServerKey()
	if len(serverKey) != 32 {
		t.Fatalf("serverKey len want 32, got %d", len(serverKey))
	}
	// 3. 服务端回 (0,0, serverKey)
	serverKeyMsg := protocol.New(exchangeCmd, exchangeAct, serverKey)
	out, err = serverEnc.BeforeEncode(noOpAgent, serverKeyMsg)
	if err != nil || out != serverKeyMsg {
		t.Fatalf("server send key: %v", err)
	}
	// 4. 客户端收 serverKey，派生密钥
	in, err = clientEnc.AfterDecode(noOpAgent, serverKeyMsg)
	if err != nil || in != serverKeyMsg {
		t.Fatalf("client recv serverKey: %v", err)
	}
	// 5. 双方用派生密钥加解密
	plain := []byte("secret body")
	msg := protocol.New(1, 2, plain)
	encrypted, err := serverEnc.BeforeEncode(noOpAgent, msg)
	if err != nil {
		t.Fatalf("server encode: %v", err)
	}
	if bytes.Equal(encrypted.Data, plain) {
		t.Error("data should be XOR encrypted")
	}
	decrypted, err := clientEnc.AfterDecode(noOpAgent, encrypted)
	if err != nil {
		t.Fatalf("client decode: %v", err)
	}
	if string(decrypted.Data) != string(plain) {
		t.Errorf("decrypt want %q, got %q", plain, decrypted.Data)
	}
	// 反向：客户端加密，服务端解密
	encrypted2, _ := clientEnc.BeforeEncode(noOpAgent, msg)
	decrypted2, _ := serverEnc.AfterDecode(noOpAgent, encrypted2)
	if string(decrypted2.Data) != string(plain) {
		t.Errorf("reverse decrypt want %q, got %q", plain, decrypted2.Data)
	}
}
