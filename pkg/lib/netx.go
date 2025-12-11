package lib

import (
	"net"
	"net/http"
	"strings"

	"github.com/panjf2000/gnet/v2/pkg/errors"
)

func ParseProtoAddr(protoAddr string) (string, string, error) {
	protoAddr = strings.ToLower(protoAddr)
	if strings.Count(protoAddr, "://") != 1 {
		return "", "", errors.ErrInvalidNetworkAddress
	}
	pair := strings.SplitN(protoAddr, "://", 2)
	if len(pair) < 2 {
		return "", "", errors.ErrInvalidNetworkAddress
	}
	proto, addr := pair[0], pair[1]
	switch proto {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6", "unix":
	default:
		return "", "", errors.ErrUnsupportedProtocol
	}
	if addr == "" {
		return "", "", errors.ErrInvalidNetworkAddress
	}
	return proto, addr, nil
}

const (
	XForwardedFor = "X-Forwarded-For"
	XRealIP       = "X-Real-IP"
)

func HttpReqRemoteIp(req *http.Request) string {
	remoteAddr := req.RemoteAddr
	if ip := req.Header.Get(XForwardedFor); ip != "" {
		remoteAddr = ip
	} else if ip = req.Header.Get(XRealIP); ip != "" {
		remoteAddr = ip
	} else {
		remoteAddr, _, _ = net.SplitHostPort(remoteAddr)
	}

	if remoteAddr == "::1" {
		remoteAddr = "127.0.0.1"
	}
	return remoteAddr
}
