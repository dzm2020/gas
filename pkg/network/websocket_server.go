package network

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/grs"

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
	*baseServer
	upgrader   websocket.Upgrader // WebSocket 升级器
	httpServer *http.Server       // HTTP 服务器
	path       string             // WebSocket 路径（如 "/ws"）
	useTLS     bool               // 是否使用 TLS
}

// NewWebSocketServer 创建 WebSocket 服务器
func NewWebSocketServer(base *baseServer, path string) *WebSocketServer {
	upgrader := defaultUpgrader
	if base.options.ReadBufSize > 0 {
		upgrader.ReadBufferSize = base.options.ReadBufSize
	}
	if base.options.SendBufferSize > 0 {
		upgrader.WriteBufferSize = base.options.SendBufferSize
	}
	return &WebSocketServer{
		baseServer: base,
		upgrader:   upgrader,
		path:       path,
		useTLS:     base.network == "wss",
	}
}

func (s *WebSocketServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc(s.path, s.handleWebSocket)
	s.httpServer = &http.Server{
		Addr:    s.address,
		Handler: mux,
	}
	s.waitGroup.Add(1)
	grs.Go(func(ctx context.Context) {
		defer s.waitGroup.Done()
		glog.Info("WebSocket监听", zap.String("addr", s.Addr()), zap.String("path", s.path))
		if err := s.listenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			glog.Error("WebSocket监听", zap.String("addr", s.Addr()), zap.String("path", s.path), zap.Error(err))
			return
		}
	})
	return nil
}

func (s *WebSocketServer) listenAndServe() error {
	if s.useTLS {
		if s.options.TLSCertFile == "" || s.options.TLSKeyFile == "" {
			return fmt.Errorf("TLS证书文件或私钥文件未配置")
		}
		cert, err := tls.LoadX509KeyPair(s.options.TLSCertFile, s.options.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("加载TLS证书失败: %w", err)
		}
		s.httpServer.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		return s.httpServer.ListenAndServeTLS("", "")
	} else {
		return s.httpServer.ListenAndServe()
	}
}

func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级 HTTP 连接为 WebSocket 连接
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		glog.Error("WebSocket升级失败", zap.String("addr", s.Addr()), zap.Error(err))
		return
	}

	wsConn := newWebSocketConnection(s.ctx, conn, Accept, s.options)
	AddConnection(wsConn)

	s.waitGroup.Add(1)
	grs.Go(func(ctx context.Context) {
		wsConn.readLoop()
		s.waitGroup.Done()
	})

	s.waitGroup.Add(1)
	grs.Go(func(ctx context.Context) {
		wsConn.writeLoop()
		s.waitGroup.Done()
	})

	s.waitGroup.Add(1)
	grs.Go(func(ctx context.Context) {
		wsConn.heartLoop(wsConn)
		s.waitGroup.Done()
	})
}

func (s *WebSocketServer) Shutdown(ctx context.Context) {
	if !s.Stop() {
		return
	}

	s.baseServer.Shutdown(ctx)
	_ = s.httpServer.Shutdown(ctx)

	glog.Info("WebSocket服务器关闭", zap.String("addr", s.Addr()), zap.String("path", s.path))
	return
}
