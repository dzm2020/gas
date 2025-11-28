package lib

import (
	"errors"
	"io"
)

type IBuffer interface {
	Len() int
	Cap() int
	Available() int
	Reset()
	Bytes() []byte
	Readable() []byte
	Write(data []byte) (int, error)
	WriteReader(reader io.Reader) (int, error)
	WriteByte(c byte) error
	Read(p []byte) (int, error)
	ReadByte() (byte, error)
	Peek(n int) ([]byte, error)
	Skip(n int) error
	String() string
}

// Buffer 网络缓冲区结构体
type Buffer struct {
	buf   []byte // 实际存储数据的字节切片
	r     int    // 读指针（下一个要读取的位置）
	w     int    // 写指针（下一个要写入的位置）
	cap   int    // 缓冲区总容量
	limit int    // 限制可写入的最大位置（通常等于cap）
}

// New 创建一个指定初始容量的缓冲区
func New(initialCap int) *Buffer {
	if initialCap <= 0 {
		initialCap = 4096 // 默认初始容量4KB
	}
	return &Buffer{
		buf:   make([]byte, initialCap),
		cap:   initialCap,
		limit: initialCap * 10,
	}
}

// Len 返回当前已写入数据的长度（可读取的字节数）
func (b *Buffer) Len() int {
	return b.w - b.r
}

// Cap 返回缓冲区总容量
func (b *Buffer) Cap() int {
	return b.cap
}

// Available 返回剩余可写入的空间
func (b *Buffer) Available() int {
	return b.limit - b.w
}

// Reset 重置缓冲区（清空数据，指针归零）
func (b *Buffer) Reset() {
	b.r = 0
	b.w = 0
}

// Bytes 返回当前可读取的数据切片（不复制）
func (b *Buffer) Bytes() []byte {
	return b.buf[b.r:b.w]
}

// Readable 返回可读取的数据切片（复制，避免外部修改内部缓冲区）
func (b *Buffer) Readable() []byte {
	data := make([]byte, b.Len())
	copy(data, b.buf[b.r:b.w])
	return data
}

// 确保有足够的写入空间，不够则扩容
func (b *Buffer) ensureSpace(n int) {
	if b.Available() >= n {
		return
	}

	// 如果已读数据较多，先压缩缓冲区（将未读数据移到开头）
	if b.r > 0 {
		copy(b.buf, b.buf[b.r:b.w])
		b.w -= b.r
		b.r = 0
	}

	// 压缩后仍不够，进行扩容
	if b.Available() < n {
		newCap := b.cap
		for newCap-b.w < n {
			newCap *= 2                // 每次翻倍扩容
			if newCap > 1024*1024*10 { // 最大容量限制（10MB）
				newCap = 1024*1024*10 + n
			}
		}
		newBuf := make([]byte, newCap)
		copy(newBuf, b.buf[:b.w])
		b.buf = newBuf
		b.cap = newCap
		b.limit = newCap
	}
}

// Write 写入字节切片到缓冲区
func (b *Buffer) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	b.ensureSpace(len(data))
	n := copy(b.buf[b.w:], data)
	b.w += n
	return n, nil
}

// WriteReader 从io.Reader读取数据并写入缓冲区
// 返回读取的字节数和可能的错误（io.EOF表示读取结束但成功写入已读数据）
func (b *Buffer) WriteReader(reader io.Reader) (int, error) {
	total := 0
	// 每次读取时使用当前可用空间，减少扩容次数
	for {
		// 确保至少有4KB的可用空间（可根据实际场景调整）
		need := 4096
		if b.Available() < need {
			b.ensureSpace(need)
		}

		// 从reader读取数据到缓冲区的可用空间
		n, err := reader.Read(b.buf[b.w:])
		if n > 0 {
			b.w += n
			total += n
		}

		// 处理读取错误
		if err != nil {
			if err == io.EOF {
				// 读取结束但已成功读取部分数据，返回已读数量和nil
				return total, nil
			}
			// 其他错误（如连接中断），返回已读数量和错误
			return total, err
		}

		// 如果读取到0字节且无错误，说明reader暂时无数据，退出循环
		if n == 0 {
			break
		}
	}
	return total, nil
}

// WriteByte 写入单个字节
func (b *Buffer) WriteByte(c byte) error {
	b.ensureSpace(1)
	b.buf[b.w] = c
	b.w++
	return nil
}

// Read 从缓冲区读取数据到p
func (b *Buffer) Read(p []byte) (int, error) {
	if b.Len() == 0 {
		return 0, io.EOF
	}

	n := copy(p, b.buf[b.r:b.w])
	b.r += n
	return n, nil
}

// ReadByte 读取单个字节
func (b *Buffer) ReadByte() (byte, error) {
	if b.Len() == 0 {
		return 0, io.EOF
	}
	c := b.buf[b.r]
	b.r++
	return c, nil
}

// Peek 查看缓冲区前n个字节（不移动读指针）
func (b *Buffer) Peek(n int) ([]byte, error) {
	if n <= 0 {
		return []byte{}, nil
	}
	if b.Len() < n {
		return nil, errors.New("insufficient data")
	}
	return b.buf[b.r : b.r+n], nil
}

// Skip 跳过n个字节（移动读指针）
func (b *Buffer) Skip(n int) error {
	if n <= 0 {
		return nil
	}
	if b.Len() < n {
		return errors.New("insufficient data to skip")
	}
	b.r += n
	return nil
}

// String 转换为字符串
func (b *Buffer) String() string {
	return string(b.buf[b.r:b.w])
}
