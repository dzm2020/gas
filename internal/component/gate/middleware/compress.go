package middleware

// 压缩插件：对 Body 做 gzip 压缩/解压，通过 Head.Tag 的 TagCompressed 位标记是否已压缩。
import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"

	"github.com/dzm2020/gas/internal/component/gate/gateiface"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
)

var _ gateiface.IMiddleware = (*Compress)(nil)

const TagCompressed = 1 << 0 // Head.Tag 的位：1 表示 Data 为 gzip 压缩后内容

var ErrDecompress = errors.New("middleware: decompress failed")

// Compress 压缩中间件：BeforeEncode 时对 Data 做 gzip 压缩并设置 Tag；AfterDecode 时若 Tag 带压缩位则解压并清除该位。
type Compress struct {
	minLen int // 仅当 len(Data) >= minLen 时压缩，0 表示始终压缩
}

// NewCompress 创建压缩中间件。minLen 为启用压缩的最小 Body 长度，0 表示一律压缩。
func NewCompress(minLen int) *Compress {
	return &Compress{minLen: minLen}
}

func (c *Compress) AfterDecode(_ gateiface.IAgent, msg *protocol.Message) (*protocol.Message, error) {
	if msg == nil || len(msg.Data) == 0 {
		return msg, nil
	}
	if msg.Tag&TagCompressed == 0 {
		return msg, nil
	}
	rd, err := gzip.NewReader(bytes.NewReader(msg.Data))
	if err != nil {
		return nil, ErrDecompress
	}
	defer rd.Close()
	data, err := io.ReadAll(rd)
	if err != nil {
		return nil, ErrDecompress
	}
	head := *msg.Head
	head.Tag &^= TagCompressed
	return &protocol.Message{Head: &head, Data: data}, nil
}

func (c *Compress) BeforeEncode(_ gateiface.IAgent, msg *protocol.Message) (*protocol.Message, error) {
	if msg == nil {
		return nil, nil
	}
	if len(msg.Data) == 0 {
		return msg, nil
	}
	if c.minLen > 0 && len(msg.Data) < c.minLen {
		return msg, nil
	}
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(msg.Data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	head := *msg.Head
	head.Tag |= TagCompressed
	return &protocol.Message{Head: &head, Data: buf.Bytes()}, nil
}
