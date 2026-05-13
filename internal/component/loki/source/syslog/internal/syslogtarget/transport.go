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
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grafana/dskit/backoff"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/leodido/go-syslog/v4"
	"github.com/mwitkow/go-conntrack"
	"github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/model/labels"

	scrapeconfig "github.com/grafana/alloy/internal/component/loki/source/syslog/config"
	"github.com/grafana/alloy/internal/component/loki/source/syslog/internal/syslogtarget/syslogparser"
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
	handleMessage      = func(labels.Labels, syslog.Message)
	handleMessageError = func(error)
)

type baseTransport struct {
	config *scrapeconfig.SyslogTargetConfig
	logger *slog.Logger

	pendingGoroutines *sync.WaitGroup

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

func (t *baseTransport) streamParseConfig() syslogparser.StreamParseConfig {
	ciscoCfg := t.config.RFC3164CiscoComponents
	parseCfg := syslogparser.StreamParseConfig{
		MaxMessageLength:      t.maxMessageLength(),
		IsRFC3164Message:      t.config.IsRFC3164Message(),
		UseRFC3164DefaultYear: t.config.RFC3164DefaultToCurrentYear,
	}

	if ciscoCfg != nil {
		parseCfg.RFC3164CiscoComponents = &syslogparser.RFC3164CiscoComponents{
			MessageCounter:  ciscoCfg.EnableAll || ciscoCfg.MessageCounter,
			SequenceNumber:  ciscoCfg.EnableAll || ciscoCfg.SequenceNumber,
			CiscoHostname:   ciscoCfg.EnableAll || ciscoCfg.Hostname,
			SecondFractions: ciscoCfg.EnableAll || ciscoCfg.SecondFractions,
		}
	}

	return parseCfg
}

func (t *baseTransport) connectionLabels(ip string) labels.Labels {
	return t.connectionLabelsWithHostname(ip, lookupAddr(ip))
}

func (t *baseTransport) connectionLabelsWithHostname(ip, hostname string) labels.Labels {
	lb := labels.NewBuilder(labels.EmptyLabels())
	for k, v := range t.config.Labels {
		lb.Set(string(k), string(v))
	}

	lb.Set("__syslog_connection_ip_address", ip)
	lb.Set("__syslog_connection_hostname", hostname)

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

type TransportConfig struct {
	Logger         *slog.Logger
	Target         *scrapeconfig.SyslogTargetConfig
	MessageHandler handleMessage
	ErrorHandler   handleMessageError
}

func newBaseTransport(cfg TransportConfig) *baseTransport {
	ctx, cancel := context.WithCancel(context.Background())
	return &baseTransport{
		config:             cfg.Target,
		logger:             cfg.Logger,
		pendingGoroutines:  new(sync.WaitGroup),
		handleMessage:      cfg.MessageHandler,
		handleMessageError: cfg.ErrorHandler,
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

func NewSyslogTCPTransport(cfg TransportConfig) Transport {
	return &TCPTransport{
		baseTransport: newBaseTransport(cfg),
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
	t.logger.Info("syslog listening on address", "address", t.Addr().String(), "protocol", ProtocolTCP, "tls", tlsEnabled)

	t.pendingGoroutines.Add(1)
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
	defer t.pendingGoroutines.Done()

	l := t.logger.With("address", t.listener.Addr().String())

	backoff := backoff.New(t.ctx, backoff.Config{
		MinBackoff: 5 * time.Millisecond,
		MaxBackoff: 1 * time.Second,
	})

	for {
		c, err := t.listener.Accept()
		if err != nil {
			if !t.Ready() {
				l.Info("syslog server shutting down", "protocol", ProtocolTCP, "err", t.ctx.Err())
				return
			}

			if _, ok := err.(net.Error); ok {
				l.Warn("failed to accept syslog connection", "err", err, "num_retries", backoff.NumRetries())
				backoff.Wait()
				continue
			}

			l.Error("failed to accept syslog connection. quitting", "err", err)
			return
		}
		backoff.Reset()

		t.pendingGoroutines.Add(1)
		go t.handleConnection(c)
	}
}

func (t *TCPTransport) handleConnection(cn net.Conn) {
	defer t.pendingGoroutines.Done()

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

		t.logger.Debug("syslog connection closed", "remote", c.RemoteAddr().String())
		return
	}

	parseCfg := t.streamParseConfig()
	err := syslogparser.ParseStream(parseCfg, c, cb)
	if err != nil {
		if errors.Is(err, io.EOF) {
			t.logger.Debug("syslog connection closed", "remote", c.RemoteAddr().String())
		} else {
			t.logger.Warn("error initializing syslog stream", "err", err)
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
	t.pendingGoroutines.Wait()
}

// Addr implements SyslogTransport
func (t *TCPTransport) Addr() net.Addr {
	return t.listener.Addr()
}

type UDPTransport struct {
	*baseTransport
	udpConn   *net.UDPConn
	hostCache *lru.Cache[string, string]
}

func NewSyslogUDPTransport(cfg TransportConfig) Transport {
	cacheSize := cfg.Target.UDPHostCacheSize
	if cacheSize == 0 {
		cacheSize = scrapeconfig.DefaultUDPHostCacheSize
	}
	hostCache, _ := lru.New[string, string](cacheSize)
	return &UDPTransport{
		baseTransport: newBaseTransport(cfg),
		hostCache:     hostCache,
	}
}

func (t *UDPTransport) lookupHostname(ip string) string {
	if hostname, ok := t.hostCache.Get(ip); ok {
		return hostname
	}
	hostname := lookupAddr(ip)
	t.hostCache.Add(ip, hostname)
	return hostname
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
	t.logger.Info("syslog listening on address", "address", t.Addr().String(), "protocol", ProtocolUDP)

	t.pendingGoroutines.Add(1)
	go t.acceptPackets()
	return nil
}

// Close implements SyslogTransport
func (t *UDPTransport) Close() error {
	t.baseTransport.close()
	return t.udpConn.Close()
}

type datagram struct {
	addr net.Addr
	data []byte
}

func (t *UDPTransport) acceptPackets() {
	defer t.pendingGoroutines.Done()
	defer func() {
		t.logger.Info("syslog server shutting down", "protocol", ProtocolUDP, "err", t.ctx.Err())
	}()

	chanSize := t.config.UDPQueueSize
	if chanSize == 0 {
		chanSize = scrapeconfig.DefaultUDPQueueSize
	}

	ch := make(chan datagram, chanSize)
	defer close(ch)

	t.pendingGoroutines.Go(func() {
		for msg := range ch {
			t.handleDatagram(msg)
		}
	})

	for {
		if !t.Ready() {
			return
		}

		buf := make([]byte, t.maxMessageLength())

		// Unlike TCP, if datagram is larger than a buf - unread bytes are discarded.
		n, addr, err := t.udpConn.ReadFrom(buf)
		if n <= 0 && err != nil {
			if !t.Ready() {
				return
			}

			t.logger.Warn("failed to read packets", "addr", addr, "err", err)
			continue
		}

		buf = buf[:n]
		msg := datagram{
			addr: addr,
			data: buf,
		}

		select {
		case <-t.ctx.Done():
			return
		case ch <- msg:
		}
	}
}

func (t *UDPTransport) handleDatagram(msg datagram) {
	srcIP := hostFromAddr(msg.addr)
	hostname := t.lookupHostname(srcIP)
	lbs := t.connectionLabelsWithHostname(srcIP, hostname)
	r := bytes.NewReader(msg.data)

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
		return
	}

	parseCfg := t.streamParseConfig()
	err := syslogparser.ParseStream(parseCfg, r, cb)
	if err != nil {
		t.logger.Warn("error parsing syslog stream", "err", err)
	}
}

// Wait implements SyslogTransport
func (t *UDPTransport) Wait() {
	t.pendingGoroutines.Wait()
}

// Addr implements SyslogTransport
func (t *UDPTransport) Addr() net.Addr {
	return t.udpConn.LocalAddr()
}

func hostFromAddr(addr net.Addr) string {
	switch v := addr.(type) {
	case *net.TCPAddr:
		return v.IP.String()
	case *net.UDPAddr:
		return v.IP.String()
	case *net.IPAddr:
		return v.IP.String()
	default:
		// Fallback: parse from the string representation
		strAddr := addr.String()
		host, _, err := net.SplitHostPort(strAddr)
		if err != nil {
			return strAddr
		}
		return host
	}
}
