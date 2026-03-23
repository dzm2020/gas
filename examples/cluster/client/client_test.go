//go:build integration

// Package client 的 TestClient 需本机已启动 Gate（如 127.0.0.1:9002）。
// 默认 go test 不包含本包；集成测试：go test -tags=integration ./examples/cluster/client/...

package client

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/dzm2020/gas/examples/cluster/common"
	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/pkg/lib/serializer"
)

func readLoop(con net.Conn) {
	// 解码消息
	for {
		buf := make([]byte, 1024)
		n, _ := con.Read(buf)
		if n == 0 {
			break
		}
		for {
			response, decodeN, err := codec.Decode(buf)
			if decodeN == 0 {
				break
			}
			fmt.Printf("client recv head:%+v data:%v  len:%d\n", response.Head, string(response.Data), n)
			buf = buf[decodeN:]
			n -= decodeN
			if n <= 0 || err != nil {
				break
			}

		}
	}
}

func TestClient(t *testing.T) {

	// 模拟客户端连接
	conn, err := net.DialTimeout("tcp", "127.0.0.1:9002", 5*time.Second)
	if err != nil {
		t.Fatalf("connect to gate failed: %v", err)
	}
	//defer conn.Close()

	// 创建测试消息
	request := common.LoginRequest{
		//Username: "username",
		//Password: "password",
		//Uid:      1,
	}
	bin, _ := serializer.Json.Marshal(&request)
	msg := protocol.New(1, 1, bin)
	msg.SetIndex(222)
	// 编码消息
	encoded, _ := codec.Encode(msg)

	// 发送消息
	if _, err := conn.Write(encoded); err != nil {
		t.Fatalf("send message failed: %v", err)
	}
	fmt.Printf("Client sent message: bin:%v cmd=%d, act=%d, data=%s   \n ", encoded, msg.GetCmd(), msg.GetAct(), string(msg.Data))

	go readLoop(conn)

	time.Sleep(2 * time.Second)
	_ = conn.Close()
}
