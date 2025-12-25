package network

import (
	"context"
	"crypto/tls"
	"fmt"
	"gas/pkg/glog"
	"gas/pkg/lib/grs"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// ------------------------------ WebSocket服务器 ------------------------------

var (
	// 默认 WebSocket 升级器
	defaultUpgrader = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type WebSocketServer struct {
	options    *Options
	upgrader   websocket.Upgrader // WebSocket 升级器
	httpServer *http.Server       // HTTP 服务器
	addr       string             // 监听地址（如 ":8080"）
	path       string             // WebSocket 路径（如 "/ws"）
	useTLS     bool               // 是否使用 TLS
	protoAddr  string
	once       sync.Once
}

// NewWebSocketServer 创建 WebSocket 服务器
// addr: 监听地址，格式为 "host:port" 或 "host:port/path"
// useTLS: 是否使用 TLS
// option: 可选的配置选项
func NewWebSocketServer(protoAddr, addr string, useTLS bool, option ...Option) *WebSocketServer {
	upgrader := defaultUpgrader
	opts := loadOptions(option...)
	if opts.readBufSize > 0 {
		upgrader.ReadBufferSize = opts.readBufSize
	}
	if opts.sendChanSize > 0 {
		upgrader.WriteBufferSize = opts.sendChanSize
	}

	// 解析地址和路径
	path := "/"
	if idx := strings.Index(addr, "/"); idx >= 0 {
		path = addr[idx:]
		addr = addr[:idx]
	}

	return &WebSocketServer{
		options:   opts,
		upgrader:  upgrader,
		addr:      addr,
		path:      path,
		useTLS:    useTLS,
		protoAddr: protoAddr,
	}
}

func (s *WebSocketServer) Addr() string {
	return s.addr
}

func (s *WebSocketServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc(s.path, s.handleWebSocket)
	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}
	grs.Go(func(ctx context.Context) {
		var err error
		if s.useTLS {
			err = s.listenAndServeWSTLS()
		} else {
			err = s.listenAndServeWS()
		}
		if err != nil {
			glog.Error("WebSocket监听失败", zap.Error(err))
		}
	})
	return nil
}

func (s *WebSocketServer) listenAndServeWSTLS() error {
	if s.options.tlsCertFile == "" || s.options.tlsKeyFile == "" {
		return fmt.Errorf("TLS证书文件或私钥文件未配置")
	}

	cert, err := tls.LoadX509KeyPair(s.options.tlsCertFile, s.options.tlsKeyFile)
	if err != nil {
		return fmt.Errorf("加载TLS证书失败: %w", err)
	}

	s.httpServer.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	return s.httpServer.ListenAndServeTLS("", "")
}

func (s *WebSocketServer) listenAndServeWS() error {
	return s.httpServer.ListenAndServe()
}

func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级 HTTP 连接为 WebSocket 连接
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		glog.Error("WebSocket升级失败", zap.String("addr", s.addr), zap.Error(err))
		return
	}

	connection := newWebSocketConnection(conn, Accept, s.options)
	AddConnection(connection)
}

func (s *WebSocketServer) Shutdown() error {
	var err error
	s.once.Do(func() {
		glog.Info("WebSocket服务器关闭", zap.String("addr", s.addr), zap.String("path", s.path))

		if s.httpServer != nil {
			ctx := context.Background()
			if err = s.httpServer.Shutdown(ctx); err != nil {
				return
			}
		}

	})
	return err
}
