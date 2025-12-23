package syslogtarget

// This code is copied from Promtail. The syslogtarget package is used to
// configure and run the targets that can read syslog entries and forward them
// to other loki components.

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/backoff"
	"github.com/leodido/go-syslog/v4"
	"github.com/mwitkow/go-conntrack"
	"github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/model/labels"

	scrapeconfig "github.com/grafana/alloy/internal/component/loki/source/syslog/config"
	"github.com/grafana/alloy/internal/component/loki/source/syslog/internal/syslogtarget/syslogparser"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

var (
	ProtocolUDP = "udp"
	ProtocolTCP = "tcp"
)

type Transport interface {
	Run() error
	Addr() net.Addr
	Ready() bool
	Close() error
	Wait()
}

type (
	handleMessage      func(labels.Labels, syslog.Message)
	handleMessageError func(error)
)

type baseTransport struct {
	config *scrapeconfig.SyslogTargetConfig
	logger log.Logger

	openConnections *sync.WaitGroup

	handleMessage      handleMessage
	handleMessageError handleMessageError

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (t *baseTransport) close() {
	t.ctxCancel()
}

// Ready implements SyslogTransport
func (t *baseTransport) Ready() bool {
	return t.ctx.Err() == nil
}

func (t *baseTransport) idleTimeout() time.Duration {
	if t.config.IdleTimeout != 0 {
		return t.config.IdleTimeout
	}
	return DefaultIdleTimeout
}

func (t *baseTransport) maxMessageLength() int {
	if t.config.MaxMessageLength != 0 {
		return t.config.MaxMessageLength
	}
	return DefaultMaxMessageLength
}

func (t *baseTransport) connectionLabels(ip string) labels.Labels {
	lb := labels.NewBuilder(labels.EmptyLabels())
	for k, v := range t.config.Labels {
		lb.Set(string(k), string(v))
	}

	lb.Set("__syslog_connection_ip_address", ip)
	lb.Set("__syslog_connection_hostname", lookupAddr(ip))

	return lb.Labels()
}

func ipFromConn(c net.Conn) net.IP {
	switch addr := c.RemoteAddr().(type) {
	case *net.TCPAddr:
		return addr.IP
	}

	return nil
}

func lookupAddr(addr string) string {
	names, _ := net.LookupAddr(addr)
	return strings.Join(names, ",")
}

func newBaseTransport(config *scrapeconfig.SyslogTargetConfig, handleMessage handleMessage, handleError handleMessageError, logger log.Logger) *baseTransport {
	ctx, cancel := context.WithCancel(context.Background())
	return &baseTransport{
		config:             config,
		logger:             logger,
		openConnections:    new(sync.WaitGroup),
		handleMessage:      handleMessage,
		handleMessageError: handleError,
		ctx:                ctx,
		ctxCancel:          cancel,
	}
}

type idleTimeoutConn struct {
	net.Conn
	idleTimeout time.Duration
}

func (c *idleTimeoutConn) Write(p []byte) (int, error) {
	c.setDeadline()
	return c.Conn.Write(p)
}

func (c *idleTimeoutConn) Read(b []byte) (int, error) {
	c.setDeadline()
	return c.Conn.Read(b)
}

func (c *idleTimeoutConn) setDeadline() {
	_ = c.Conn.SetDeadline(time.Now().Add(c.idleTimeout))
}

type ConnPipe struct {
	addr net.Addr
	*io.PipeReader
	*io.PipeWriter
}

func NewConnPipe(addr net.Addr) *ConnPipe {
	pr, pw := io.Pipe()
	return &ConnPipe{
		addr:       addr,
		PipeReader: pr,
		PipeWriter: pw,
	}
}

func (pipe *ConnPipe) Close() error {
	return pipe.PipeWriter.Close()
}

type TCPTransport struct {
	*baseTransport
	listener net.Listener
}

func NewSyslogTCPTransport(config *scrapeconfig.SyslogTargetConfig, handleMessage handleMessage, handleError handleMessageError, logger log.Logger) Transport {
	return &TCPTransport{
		baseTransport: newBaseTransport(config, handleMessage, handleError, logger),
	}
}

// Run implements SyslogTransport
func (t *TCPTransport) Run() error {
	l, err := net.Listen(ProtocolTCP, t.config.ListenAddress)
	l = conntrack.NewListener(l, conntrack.TrackWithName("syslog_target/"+t.config.ListenAddress))
	if err != nil {
		return fmt.Errorf("error setting up syslog target: %w", err)
	}

	var (
		tlsConfig = t.config.TLSConfig

		configuredCA   = len(tlsConfig.CA) > 0 || len(tlsConfig.CAFile) > 0
		configuredCert = len(tlsConfig.Cert) > 0 || len(tlsConfig.CertFile) > 0
		configuredKey  = len(tlsConfig.Key) > 0 || len(tlsConfig.KeyFile) > 0

		tlsEnabled = configuredCA || configuredCert || configuredKey
	)

	if tlsEnabled {
		tlsConfig, err := newTLSConfig(tlsConfig)
		if err != nil {
			return fmt.Errorf("error setting up syslog target: %w", err)
		}
		l = tls.NewListener(l, tlsConfig)
	}

	t.listener = l
	level.Info(t.logger).Log("msg", "syslog listening on address", "address", t.Addr().String(), "protocol", ProtocolTCP, "tls", tlsEnabled)

	t.openConnections.Add(1)
	go t.acceptConnections()

	return nil
}

// newTLSConfig creates TLS server settings from a [config.TLSConfig]. Use this
// function to create TLS server settings, and [config.NewTLSConfig] to create
// TLS client settings.
func newTLSConfig(config config.TLSConfig) (*tls.Config, error) {
	var (
		configuredCert = len(config.Cert) > 0 || len(config.CertFile) > 0
		configuredKey  = len(config.Key) > 0 || len(config.KeyFile) > 0
	)

	if !configuredCert || !configuredKey {
		return nil, fmt.Errorf("certificate and key must be configured")
	}

	var certBytes, keyBytes []byte

	if len(config.CertFile) > 0 {
		bb, err := os.ReadFile(config.CertFile)
		if err != nil {
			return nil, fmt.Errorf("unable to load server certificate: %w", err)
		}
		certBytes = bb
	} else if len(config.Cert) > 0 {
		certBytes = []byte(config.Cert)
	}

	if len(config.KeyFile) > 0 {
		bb, err := os.ReadFile(config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to load server key: %w", err)
		}
		keyBytes = bb
	} else if len(config.Key) > 0 {
		keyBytes = []byte(config.Key)
	}

	certs, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to load server certificate or key: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certs},
	}

	var caBytes []byte

	if len(config.CAFile) > 0 {
		bb, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("unable to load client CA certificate: %w", err)
		}
		caBytes = bb
	} else if len(config.CA) > 0 {
		caBytes = []byte(config.CA)
	}

	if len(caBytes) > 0 {
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caBytes); !ok {
			return nil, fmt.Errorf("unable to parse client CA certificate")
		}

		tlsConfig.ClientCAs = caCertPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig, nil
}

func (t *TCPTransport) acceptConnections() {
	defer t.openConnections.Done()

	l := log.With(t.logger, "address", t.listener.Addr().String())

	backoff := backoff.New(t.ctx, backoff.Config{
		MinBackoff: 5 * time.Millisecond,
		MaxBackoff: 1 * time.Second,
	})

	for {
		c, err := t.listener.Accept()
		if err != nil {
			if !t.Ready() {
				level.Info(l).Log("msg", "syslog server shutting down", "protocol", ProtocolTCP, "err", t.ctx.Err())
				return
			}

			if _, ok := err.(net.Error); ok {
				level.Warn(l).Log("msg", "failed to accept syslog connection", "err", err, "num_retries", backoff.NumRetries())
				backoff.Wait()
				continue
			}

			level.Error(l).Log("msg", "failed to accept syslog connection. quiting", "err", err)
			return
		}
		backoff.Reset()

		t.openConnections.Add(1)
		go t.handleConnection(c)
	}
}

func (t *TCPTransport) handleConnection(cn net.Conn) {
	defer t.openConnections.Done()

	c := &idleTimeoutConn{cn, t.idleTimeout()}

	handlerCtx, cancel := context.WithCancel(t.ctx)
	defer cancel()
	go func() {
		<-handlerCtx.Done()
		_ = c.Close()
	}()

	lbs := t.connectionLabels(ipFromConn(c).String())

	cb := func(result *syslog.Result) {
		if err := result.Error; err != nil {
			t.handleMessageError(err)
			return
		}
		t.handleMessage(lbs.Copy(), result.Message)
	}

	if t.config.SyslogFormat == scrapeconfig.SyslogFormatRaw {
		delim := t.config.RawFormatOptions.Delimiter()
		for msg, err := range syslogparser.IterStreamRaw(c, delim) {
			cb(&syslog.Result{
				Message: msg,
				Error:   err,
			})
		}

		level.Debug(t.logger).Log("msg", "syslog connection closed", "remote", c.RemoteAddr().String())
		return
	}

	err := syslogparser.ParseStream(t.config.IsRFC3164Message(), t.config.RFC3164DefaultToCurrentYear, c, cb, t.maxMessageLength())
	if err != nil {
		if errors.Is(err, io.EOF) {
			level.Debug(t.logger).Log("msg", "syslog connection closed", "remote", c.RemoteAddr().String())
		} else {
			level.Warn(t.logger).Log("msg", "error initializing syslog stream", "err", err)
		}
	}
}

// Close implements SyslogTransport
func (t *TCPTransport) Close() error {
	t.baseTransport.close()
	return t.listener.Close()
}

// Wait implements SyslogTransport
func (t *TCPTransport) Wait() {
	t.openConnections.Wait()
}

// Addr implements SyslogTransport
func (t *TCPTransport) Addr() net.Addr {
	return t.listener.Addr()
}

type UDPTransport struct {
	*baseTransport
	udpConn *net.UDPConn
}

func NewSyslogUDPTransport(config *scrapeconfig.SyslogTargetConfig, handleMessage handleMessage, handleError handleMessageError, logger log.Logger) Transport {
	return &UDPTransport{
		baseTransport: newBaseTransport(config, handleMessage, handleError, logger),
	}
}

// Run implements SyslogTransport
func (t *UDPTransport) Run() error {
	var err error
	addr, err := net.ResolveUDPAddr(ProtocolUDP, t.config.ListenAddress)
	if err != nil {
		return fmt.Errorf("error resolving UDP address: %w", err)
	}
	t.udpConn, err = net.ListenUDP(ProtocolUDP, addr)
	if err != nil {
		return fmt.Errorf("error setting up syslog target: %w", err)
	}
	_ = t.udpConn.SetReadBuffer(1024 * 1024)
	level.Info(t.logger).Log("msg", "syslog listening on address", "address", t.Addr().String(), "protocol", ProtocolUDP)

	t.openConnections.Add(1)
	go t.acceptPackets()
	return nil
}

// Close implements SyslogTransport
func (t *UDPTransport) Close() error {
	t.baseTransport.close()
	return t.udpConn.Close()
}

func (t *UDPTransport) acceptPackets() {
	defer t.openConnections.Done()

	var (
		n    int
		addr net.Addr
		err  error
	)
	streams := make(map[string]*ConnPipe)
	buf := make([]byte, t.maxMessageLength())

	for {
		if !t.Ready() {
			level.Info(t.logger).Log("msg", "syslog server shutting down", "protocol", ProtocolUDP, "err", t.ctx.Err())
			for _, stream := range streams {
				if err = stream.Close(); err != nil {
					level.Error(t.logger).Log("msg", "failed to close pipe", "err", err)
				}
			}
			return
		}
		n, addr, err = t.udpConn.ReadFrom(buf)
		if n <= 0 && err != nil {
			level.Warn(t.logger).Log("msg", "failed to read packets", "addr", addr, "err", err)
			continue
		}

		stream, ok := streams[addr.String()]
		if !ok {
			stream = NewConnPipe(addr)
			streams[addr.String()] = stream
			t.openConnections.Add(1)
			go t.handleRcv(stream)
		}
		if _, err := stream.Write(buf[:n]); err != nil {
			level.Warn(t.logger).Log("msg", "failed to write to stream", "addr", addr, "err", err)
		}
	}
}

func (t *UDPTransport) handleRcv(c *ConnPipe) {
	defer t.openConnections.Done()

	udpAddr, _ := net.ResolveUDPAddr("udp", c.addr.String())
	lbs := t.connectionLabels(udpAddr.IP.String())

	for {
		datagram := make([]byte, t.maxMessageLength())
		n, err := c.Read(datagram)
		if err != nil {
			if err == io.EOF {
				break
			}

			level.Warn(t.logger).Log("msg", "error reading from pipe", "err", err)
			continue
		}

		r := bytes.NewReader(datagram[:n])
		cb := func(result *syslog.Result) {
			if err := result.Error; err != nil {
				t.handleMessageError(err)
			} else {
				t.handleMessage(lbs.Copy(), result.Message)
			}
		}

		if t.config.SyslogFormat == scrapeconfig.SyslogFormatRaw {
			delim := t.config.RawFormatOptions.Delimiter()
			for msg, err := range syslogparser.IterStreamRaw(r, delim) {
				cb(&syslog.Result{
					Message: msg,
					Error:   err,
				})
			}
			continue
		}

		err = syslogparser.ParseStream(t.config.IsRFC3164Message(), t.config.RFC3164DefaultToCurrentYear, r, func(result *syslog.Result) {
			if err := result.Error; err != nil {
				t.handleMessageError(err)
			} else {
				t.handleMessage(lbs.Copy(), result.Message)
			}
		}, t.maxMessageLength())
		if err != nil {
			level.Warn(t.logger).Log("msg", "error parsing syslog stream", "err", err)
		}
	}
}

// Wait implements SyslogTransport
func (t *UDPTransport) Wait() {
	t.openConnections.Wait()
}

// Addr implements SyslogTransport
func (t *UDPTransport) Addr() net.Addr {
	return t.udpConn.LocalAddr()
}
