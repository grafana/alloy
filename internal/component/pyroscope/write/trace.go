package write

import (
	"crypto/tls"
	"fmt"
	"net/http/httptrace"
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
	l := &clientTrace{}
	l.trace = &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			l.log(
				"msg", "GetConn",
				"hostPort", hostPort,
			)
		},
		GotConn: func(info httptrace.GotConnInfo) {
			l.log(
				"msg", "GotConn",
				"Reused", info.Reused,
				"WasIdle", info.WasIdle,
				"IdleTime", info.IdleTime,
				"Conn", info.Conn,
			)
		},
		PutIdleConn: func(err error) {
			l.log(
				"msg", "PutIdleConn",
				"err", err,
			)
		},
		GotFirstResponseByte: func() {
			l.log("msg", "GotFirstResponseByte")
		},
		Got100Continue: nil,
		Got1xxResponse: nil,
		DNSStart: func(info httptrace.DNSStartInfo) {
			l.log(
				"msg", "DNSStart",
				"Host", info.Host,
			)
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			l.log(
				"msg", "DNSDone",
				"info", fmt.Sprintf("%+v", info),
				"Coalesced", info.Coalesced,
				"Err", info.Err,
			)
		},
		ConnectStart: func(network, addr string) {
			l.log(
				"msg", "ConnectStart",
				"addr", addr,
				"network", network)
		},
		ConnectDone: func(network, addr string, err error) {
			l.log(
				"msg", "ConnectDone",
				"addr", addr, "network",
				network, "err", err,
			)
		},
		TLSHandshakeStart: func() {
			l.log("msg", "TLSHandshakeStart")
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			l.log(
				"msg", "TLSHandshakeDone",
				"state", fmt.Sprintf("%+v", state),
				"err", err,
			)
		},
		WroteHeaderField: nil,
		WroteHeaders: func() {
			l.log("msg", "WroteHeaders")
		},
		Wait100Continue: nil,
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			l.log(
				"msg", "WroteRequest",
				"Err", info.Err,
			)
		},
	}
	return l
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
