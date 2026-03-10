// Package middleware
// @Description: 加密插件：连接建立后客户端发送 cmd=0 act=0 携带 clientKey，服务端回复 serverKey，
//				 双方根据 serverKey+clientKey 生成对称密钥，之后对 Body 做按位异或加解密。

package middleware

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

// NewEncrypt
//
//	@Description: 创建服务端加密中间件，随机 serverKey，密钥交换 (0,0)。
//	@return *Encrypt
//	@return error
func NewEncrypt() (*Encrypt, error) {
	serverKey := make([]byte, 32)
	if _, err := rand.Read(serverKey); err != nil {
		return nil, err
	}
	return &Encrypt{serverKey: serverKey}, nil
}

// NewEncryptClient
//
//	@Description: 创建客户端加密中间件，收到 (0,0, serverKey) 后派生密钥。
//	@param clientKey
//	@return *Encrypt
func NewEncryptClient(clientKey []byte) *Encrypt {
	key := make([]byte, len(clientKey))
	copy(key, clientKey)
	return &Encrypt{clientKey: key}
}

// ServerKey
//
//	@Description: 返回本端 serverKey，供密钥交换回复使用。
//	@receiver e
//	@return []byte
func (e *Encrypt) ServerKey() []byte {
	return e.serverKey
}

// AfterDecode
//
//	@Description: 解码后：密钥交换(0,0)则派生密钥或回复；否则 XOR 解密 Body。
//	@receiver e
//	@param agent
//	@param msg
//	@return *protocol.Message
//	@return error
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

// BeforeEncode
//
//	@Description: 编码前：密钥交换(0,0)不加密，否则 XOR 加密 Body。
//	@receiver e
//	@param msg
//	@return *protocol.Message
//	@return error
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
