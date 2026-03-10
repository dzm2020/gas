package middleware

// 加密插件：连接建立后客户端发送 cmd=0 act=0 携带 clientKey，服务端回复 serverKey，
// 双方根据 serverKey+clientKey 生成对称密钥，之后对 Body 做按位异或加解密。
import (
	"crypto/rand"

	"github.com/dzm2020/gas/internal/component/gate/gateiface"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
)

var _ gateiface.IMiddleware = (*Encrypt)(nil)

// 约定：cmd=0 act=0 为密钥交换消息（客户端发 clientKey，服务端回 serverKey）。
const KeyExchangeCmd, KeyExchangeAct = 0, 0

// TagEncrypted Head.Tag 的位：1 表示 Data 为 XOR 加密后内容（与 Compress 的 TagCompressed 可组合使用不同位）。
const TagEncrypted = 1 << 1

// deriveKey 根据 serverKey 与 clientKey 生成对称密钥（简单拼接，用于 XOR）。
func deriveKey(serverKey, clientKey []byte) []byte {
	return append(append([]byte(nil), serverKey...), clientKey...)
}

func xor(dst, src, key []byte) {
	for i := range src {
		dst[i] = src[i] ^ key[i%len(key)]
	}
}

// Encrypt 加密中间件：每连接一个实例；服务端用 NewEncrypt，客户端用 NewEncryptClient；密钥交换后 Body 与派生密钥按位异或。
type Encrypt struct {
	serverKey  []byte
	clientKey  []byte
	derivedKey []byte
}

// NewEncrypt 创建服务端加密中间件。随机生成 serverKey；业务在收到 (0,0, clientKey) 后回复 (0,0, ServerKey())。
func NewEncrypt() (*Encrypt, error) {
	serverKey := make([]byte, 32)
	if _, err := rand.Read(serverKey); err != nil {
		return nil, err
	}
	return &Encrypt{serverKey: serverKey}, nil
}

// NewEncryptClient 创建客户端加密中间件，传入本地 clientKey；收到服务端 (0,0, serverKey) 后派生密钥。
func NewEncryptClient(clientKey []byte) *Encrypt {
	key := make([]byte, len(clientKey))
	copy(key, clientKey)
	return &Encrypt{clientKey: key}
}

// ServerKey 返回本端 serverKey，供服务端在密钥交换时填入回复包（cmd=0 act=0, data=ServerKey()）。
func (e *Encrypt) ServerKey() []byte {
	return e.serverKey
}

// AfterDecode 解码后：若为密钥交换（cmd=0 act=0），服务端将 data 视为 clientKey 并派生密钥，客户端将 data 视为 serverKey 并派生密钥；否则用派生密钥对 Body 异或解密。
func (e *Encrypt) AfterDecode(agent gateiface.IAgent, msg *protocol.Message) (*protocol.Message, error) {
	if msg == nil {
		return nil, nil
	}
	if msg.Cmd != KeyExchangeCmd || msg.Act != KeyExchangeAct {
		key := e.derivedKey
		if len(key) == 0 {
			return msg, nil
		}
		if msg.Tag&TagEncrypted == 0 {
			return msg, nil
		}
		if len(msg.Data) == 0 {
			return msg, nil
		}
		plain := make([]byte, len(msg.Data))
		xor(plain, msg.Data, key)
		head := *msg.Head
		head.Tag &^= TagEncrypted
		return &protocol.Message{Head: &head, Data: plain}, nil
	}

	if len(msg.Data) == 0 {
		return msg, nil
	}
	if e.derivedKey != nil {
		return msg, nil
	}
	if e.serverKey != nil {
		// 服务端：收到客户端 clientKey，派生密钥后直接通过 agent.Push 回复 serverKey 并跳过业务处理
		e.derivedKey = deriveKey(e.serverKey, msg.Data)
		reply := protocol.New(KeyExchangeCmd, KeyExchangeAct, e.ServerKey())
		if err := agent.Push(reply); err == nil {
			return nil, nil
		}
		return msg, nil
	}
	// 客户端：收到服务端 serverKey，派生密钥并交给业务（返回 msg）
	e.derivedKey = deriveKey(msg.Data, e.clientKey)
	return msg, nil

}

// BeforeEncode 编码前：若为密钥交换（cmd=0 act=0 且 data 为本端 serverKey 或 clientKey），则不加密；否则用派生密钥对 Body 异或加密。
func (e *Encrypt) BeforeEncode(_ gateiface.IAgent, msg *protocol.Message) (*protocol.Message, error) {
	if msg == nil {
		return nil, nil
	}
	if msg.Cmd == KeyExchangeCmd && msg.Act == KeyExchangeAct {
		return msg, nil
	} else {
		key := e.derivedKey
		if len(key) == 0 {
			return msg, nil
		}
		if len(msg.Data) == 0 {
			return msg, nil
		}
		cipher := make([]byte, len(msg.Data))
		xor(cipher, msg.Data, key)
		head := *msg.Head
		head.Tag |= TagEncrypted
		return &protocol.Message{Head: &head, Data: cipher}, nil
	}
}
