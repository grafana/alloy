package write

import (
	"crypto/tls"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
)

type clientTrace struct {
	trace *httptrace.ClientTrace
	es    [][]any
	mu    sync.Mutex
}

func newClientTrace() *clientTrace {
	t := &clientTrace{}
	t.trace = &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			t.log(
				"msg", "GetConn",
				"hostPort", hostPort,
			)
		},
		GotConn: func(info httptrace.GotConnInfo) {
			var remoteAddr, localAddr string
			if info.Conn != nil {
				remoteAddr = info.Conn.RemoteAddr().String()
				localAddr = info.Conn.LocalAddr().String()
			}
			t.log(
				"msg", "GotConn",
				"Reused", info.Reused,
				"WasIdle", info.WasIdle,
				"IdleTime", info.IdleTime,
				"RemoteAddr", remoteAddr,
				"LocalAddr", localAddr,
			)
		},
		PutIdleConn: func(err error) {
			t.log(
				"msg", "PutIdleConn",
				"err", err,
			)
		},
		GotFirstResponseByte: func() {
			t.log("msg", "GotFirstResponseByte")
		},
		Got100Continue: nil,
		Got1xxResponse: nil,
		DNSStart: func(info httptrace.DNSStartInfo) {
			t.log(
				"msg", "DNSStart",
				"Host", info.Host,
			)
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			var addrs []string
			for _, addr := range info.Addrs {
				addrs = append(addrs, addr.String())
			}
			t.log(
				"msg", "DNSDone",
				"Addrs", strings.Join(addrs, ","),
				"Coalesced", info.Coalesced,
				"Err", info.Err,
			)
		},
		ConnectStart: func(network, addr string) {
			t.log(
				"msg", "ConnectStart",
				"addr", addr,
				"network", network)
		},
		ConnectDone: func(network, addr string, err error) {
			t.log(
				"msg", "ConnectDone",
				"addr", addr,
				"network", network,
				"err", err,
			)
		},
		TLSHandshakeStart: func() {
			t.log("msg", "TLSHandshakeStart")
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			t.log(
				"msg", "TLSHandshakeDone",
				"Version", state.Version,
				"CipherSuite", state.CipherSuite,
				"ServerName", state.ServerName,
				"NegotiatedProtocol", state.NegotiatedProtocol,
				"err", err,
			)
		},
		WroteHeaderField: nil,
		WroteHeaders: func() {
			t.log("msg", "WroteHeaders")
		},
		Wait100Continue: nil,
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			t.log(
				"msg", "WroteRequest",
				"Err", info.Err,
			)
		},
	}
	return t
}

func (t *clientTrace) log(kvs ...any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	l := append([]any{"tt", time.Now()}, kvs...)
	t.es = append(t.es, l)
}

func (t *clientTrace) flush(logger log.Logger) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, e := range t.es {
		_ = logger.Log(e...)
	}
}
