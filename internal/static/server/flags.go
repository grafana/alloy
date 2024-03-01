package server

import (
	"flag"
	"math"
	"net"
	"strconv"
	"time"
)

// Flags hold static configuration options for a Server.
type Flags struct {
	RegisterInstrumentation bool
	GracefulShutdownTimeout time.Duration

	LogSourceIPs       bool
	LogSourceIPsHeader string
	LogSourceIPsRegex  string

	GRPC GRPCFlags
	HTTP HTTPFlags
}

// HTTPFlags hold static configuration options for the HTTP server.
type HTTPFlags struct {
	UseTLS bool

	InMemoryAddr string

	ListenNetwork string
	ListenAddress string // host:port
	ConnLimit     int

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// ListenHostPort splits the ListenAddress into a listen host and listen port.
// Returns an error if the ListenAddress isn't valid.
func (f HTTPFlags) ListenHostPort() (host string, port int, err error) {
	var portStr string
	host, portStr, err = net.SplitHostPort(f.ListenAddress)
	if err != nil {
		return
	}
	port, err = strconv.Atoi(portStr)
	return
}

// GRPCFlags hold static configuration options for the gRPC server.
type GRPCFlags struct {
	UseTLS bool

	InMemoryAddr string

	ListenNetwork string
	ListenAddress string // host:port
	ConnLimit     int

	MaxRecvMsgSize           int
	MaxSendMsgSize           int
	MaxConcurrentStreams     uint
	MaxConnectionIdle        time.Duration
	MaxConnectionAge         time.Duration
	MaxConnectionAgeGrace    time.Duration
	KeepaliveTime            time.Duration
	KeepaliveTimeout         time.Duration
	MinTimeBetweenPings      time.Duration
	PingWithoutStreamAllowed bool
}

// ListenHostPort splits the ListenAddress into a listen host and listen port.
// Returns an error if the ListenAddress isn't valid.
func (f GRPCFlags) ListenHostPort() (host string, port int, err error) {
	var portStr string
	host, portStr, err = net.SplitHostPort(f.ListenAddress)
	if err != nil {
		return
	}
	port, err = strconv.Atoi(portStr)
	return
}

var infinity = time.Duration(math.MaxInt64)

// Default options structs.
var (
	DefaultFlags = Flags{
		RegisterInstrumentation: true,
		GracefulShutdownTimeout: 30 * time.Second,

		HTTP: DefaultHTTPFlags,
		GRPC: DefaultGRPCFlags,
	}

	DefaultHTTPFlags = HTTPFlags{
		InMemoryAddr:  "agent.internal:12345",
		ListenNetwork: "tcp",
		ListenAddress: "127.0.0.1:12345",
		ReadTimeout:   30 * time.Second,
		WriteTimeout:  30 * time.Second,
		IdleTimeout:   120 * time.Second,
	}

	DefaultGRPCFlags = GRPCFlags{
		InMemoryAddr:          "agent.internal:12346",
		ListenNetwork:         "tcp",
		ListenAddress:         "127.0.0.1:12346",
		MaxRecvMsgSize:        4 * 1024 * 1024,
		MaxSendMsgSize:        4 * 1024 * 1024,
		MaxConcurrentStreams:  100,
		MaxConnectionIdle:     infinity,
		MaxConnectionAge:      infinity,
		MaxConnectionAgeGrace: infinity,
		KeepaliveTime:         2 * time.Hour,
		KeepaliveTimeout:      20 * time.Second,
		MinTimeBetweenPings:   5 * time.Minute,
	}
)

// RegisterFlags registers flags for c to the given FlagSet.
func (f *Flags) RegisterFlags(fs *flag.FlagSet) {
	d := DefaultFlags

	fs.BoolVar(&f.RegisterInstrumentation, "server.register-instrumentation", d.RegisterInstrumentation, "Register the instrumentation handlers (e.g., /metrics)")
	fs.DurationVar(&f.GracefulShutdownTimeout, "server.graceful-shutdown-timeout", d.GracefulShutdownTimeout, "Timeout for a graceful server shutdown")
	fs.BoolVar(&f.LogSourceIPs, "server.log.source-ips.enabled", d.LogSourceIPs, "Log IP address of client for incoming requests")
	fs.StringVar(&f.LogSourceIPsHeader, "server.log.source-ips.header", d.LogSourceIPsHeader, "Header field storing the source IPs. Only used if server.log-source-ips-enabled is true. Defaults to Forwarded, X-Real-IP, and X-Forwarded-For")
	fs.StringVar(&f.LogSourceIPsRegex, "server.log.source-ips.regex", d.LogSourceIPsRegex, "Regex for extracting the source IPs from the matched header. The first capture group will be used for the extracted IP address. Only used if server.log-source-ips-enabled is true.")

	f.HTTP.RegisterFlags(fs)
	f.GRPC.RegisterFlags(fs)
}

// RegisterFlags registers flags for c to the given FlagSet.
func (f *HTTPFlags) RegisterFlags(fs *flag.FlagSet) {
	d := DefaultHTTPFlags

	fs.BoolVar(&f.UseTLS, "server.http.enable-tls", d.UseTLS, "Enable TLS for the HTTP server.")
	fs.StringVar(&f.ListenAddress, "server.http.address", d.ListenAddress, "HTTP server listen host:port. Takes precedence over YAML listen flags when set.")
	fs.StringVar(&f.ListenNetwork, "server.http.network", d.ListenNetwork, "HTTP server listen network")
	fs.IntVar(&f.ConnLimit, "server.http.conn-limit", d.ConnLimit, "Maximum number of simultaneous HTTP connections (0 = unlimited)")
	fs.DurationVar(&f.ReadTimeout, "server.http.read-timeout", d.ReadTimeout, "HTTP server read timeout")
	fs.DurationVar(&f.WriteTimeout, "server.http.write-timeout", d.WriteTimeout, "HTTP server write timeout")
	fs.DurationVar(&f.IdleTimeout, "server.http.idle-timeout", d.IdleTimeout, "HTTP server idle timeout")
	fs.StringVar(&f.InMemoryAddr, "server.http.in-memory-addr", d.InMemoryAddr, "Address used to internally make in-memory requests to the HTTP server. Override if it collides with a real URL.")
}

// RegisterFlags registers flags for c to the given FlagSet.
func (f *GRPCFlags) RegisterFlags(fs *flag.FlagSet) {
	d := DefaultGRPCFlags

	fs.BoolVar(&f.UseTLS, "server.grpc.enable-tls", d.UseTLS, "Enable TLS for the gRPC server.")
	fs.StringVar(&f.ListenAddress, "server.grpc.address", d.ListenAddress, "gRPC server listen host:port. Takes precedence over YAML listen flags when set.")
	fs.StringVar(&f.ListenNetwork, "server.grpc.network", d.ListenNetwork, "gRPC server listen network")
	fs.IntVar(&f.ConnLimit, "server.grpc.conn-limit", d.ConnLimit, "Maximum number of simultaneous gRPC connections (0 = unlimited)")
	fs.IntVar(&f.MaxRecvMsgSize, "server.grpc.max-recv-msg-size-bytes", d.MaxRecvMsgSize, "Maximum size in bytes for received gRPC messages")
	fs.IntVar(&f.MaxSendMsgSize, "server.grpc.max-send-msg-size-bytes", d.MaxSendMsgSize, "Maximum size in bytes for send gRPC messages")
	fs.UintVar(&f.MaxConcurrentStreams, "server.grpc.max-concurrent-streams", d.MaxConcurrentStreams, "Maximum number of concurrent gRPC streams (0 = unlimited)")
	fs.DurationVar(&f.MaxConnectionIdle, "server.grpc.keepalive.max-connection-idle", d.MaxConnectionIdle, "Time to wait before closing idle gRPC connections")
	fs.DurationVar(&f.MaxConnectionAge, "server.grpc.keepalive.max-connection-age", d.MaxConnectionAge, "Maximum age for any gRPC connection for a graceful shutdown")
	fs.DurationVar(&f.MaxConnectionAgeGrace, "server.grpc.keepalive.max-connection-age-grace", d.MaxConnectionAgeGrace, "Grace period to forcibly close connections after a graceful shutdown starts")
	fs.DurationVar(&f.KeepaliveTime, "server.grpc.keepalive.time", d.KeepaliveTime, "Frequency to send keepalive pings from the server")
	fs.DurationVar(&f.KeepaliveTimeout, "server.grpc.keepalive.timeout", d.KeepaliveTimeout, "How long to wait for a keepalive pong before closing the connection")
	fs.DurationVar(&f.MinTimeBetweenPings, "server.grpc.keepalive.min-time-between-pings", d.MinTimeBetweenPings, "Maximum frequency that clients may send pings at")
	fs.BoolVar(&f.PingWithoutStreamAllowed, "server.grpc.keepalive.ping-without-stream-allowed", d.PingWithoutStreamAllowed, "Allow clients to send pings without having a gRPC stream")
	fs.StringVar(&f.InMemoryAddr, "server.grpc.in-memory-addr", d.InMemoryAddr, "Address used to internally make in-memory requests to the gRPC server. Override if it collides with a real URL.")
}
