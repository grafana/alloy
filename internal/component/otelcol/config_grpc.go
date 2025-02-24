package otelcol

import (
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelconfigauth "go.opentelemetry.io/collector/config/configauth"
	otelconfiggrpc "go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
)

const DefaultBalancerName = "round_robin"

// GRPCServerArguments holds shared gRPC settings for components which launch
// gRPC servers.
type GRPCServerArguments struct {
	Endpoint  string `alloy:"endpoint,attr,optional"`
	Transport string `alloy:"transport,attr,optional"`

	TLS *TLSServerArguments `alloy:"tls,block,optional"`

	MaxRecvMsgSize       units.Base2Bytes `alloy:"max_recv_msg_size,attr,optional"`
	MaxConcurrentStreams uint32           `alloy:"max_concurrent_streams,attr,optional"`
	ReadBufferSize       units.Base2Bytes `alloy:"read_buffer_size,attr,optional"`
	WriteBufferSize      units.Base2Bytes `alloy:"write_buffer_size,attr,optional"`

	Keepalive *KeepaliveServerArguments `alloy:"keepalive,block,optional"`

	// Auth is a binding to an otelcol.auth.* component extension which handles
	// authentication.
	// alloy name is auth instead of authentication so the user interface is the same as exporter components.
	Authentication *auth.Handler `alloy:"auth,attr,optional"`

	IncludeMetadata bool `alloy:"include_metadata,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *GRPCServerArguments) Convert() (*otelconfiggrpc.ServerConfig, error) {
	if args == nil {
		return nil, nil
	}

	// If auth is set add that to the config.
	var authentication *otelconfigauth.Authentication
	if args.Authentication != nil {
		// If a auth plugin does not implement server auth, an error will be returned here.
		serverExtension, err := args.Authentication.GetExtension(auth.Server)
		if err != nil {
			return nil, err
		}
		authentication = &otelconfigauth.Authentication{
			AuthenticatorID: serverExtension.ID,
		}
	}

	return &otelconfiggrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  args.Endpoint,
			Transport: confignet.TransportType(args.Transport),
		},

		TLSSetting: args.TLS.Convert(),

		MaxRecvMsgSizeMiB:    int(args.MaxRecvMsgSize / units.Mebibyte),
		MaxConcurrentStreams: args.MaxConcurrentStreams,
		ReadBufferSize:       int(args.ReadBufferSize),
		WriteBufferSize:      int(args.WriteBufferSize),
		Keepalive:            args.Keepalive.Convert(),
		IncludeMetadata:      args.IncludeMetadata,
		Auth:                 authentication,
	}, nil
}

// Extensions exposes extensions used by args.
func (args *GRPCServerArguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	m := make(map[otelcomponent.ID]otelcomponent.Component)
	if args.Authentication != nil {
		ext, err := args.Authentication.GetExtension(auth.Server)
		if err != nil {
			return m
		}
		m[ext.ID] = ext.Extension
	}
	return m
}

// KeepaliveServerArguments holds shared keepalive settings for components
// which launch servers.
type KeepaliveServerArguments struct {
	ServerParameters  *KeepaliveServerParamaters  `alloy:"server_parameters,block,optional"`
	EnforcementPolicy *KeepaliveEnforcementPolicy `alloy:"enforcement_policy,block,optional"`
}

// Convert converts args into the upstream type.
func (args *KeepaliveServerArguments) Convert() *otelconfiggrpc.KeepaliveServerConfig {
	if args == nil {
		return nil
	}

	return &otelconfiggrpc.KeepaliveServerConfig{
		ServerParameters:  args.ServerParameters.Convert(),
		EnforcementPolicy: args.EnforcementPolicy.Convert(),
	}
}

// KeepaliveServerParamaters holds shared keepalive settings for components
// which launch servers.
type KeepaliveServerParamaters struct {
	MaxConnectionIdle     time.Duration `alloy:"max_connection_idle,attr,optional"`
	MaxConnectionAge      time.Duration `alloy:"max_connection_age,attr,optional"`
	MaxConnectionAgeGrace time.Duration `alloy:"max_connection_age_grace,attr,optional"`
	Time                  time.Duration `alloy:"time,attr,optional"`
	Timeout               time.Duration `alloy:"timeout,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *KeepaliveServerParamaters) Convert() *otelconfiggrpc.KeepaliveServerParameters {
	if args == nil {
		return nil
	}

	return &otelconfiggrpc.KeepaliveServerParameters{
		MaxConnectionIdle:     args.MaxConnectionIdle,
		MaxConnectionAge:      args.MaxConnectionAge,
		MaxConnectionAgeGrace: args.MaxConnectionAgeGrace,
		Time:                  args.Time,
		Timeout:               args.Timeout,
	}
}

// KeepaliveEnforcementPolicy holds shared keepalive settings for components
// which launch servers.
type KeepaliveEnforcementPolicy struct {
	MinTime             time.Duration `alloy:"min_time,attr,optional"`
	PermitWithoutStream bool          `alloy:"permit_without_stream,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *KeepaliveEnforcementPolicy) Convert() *otelconfiggrpc.KeepaliveEnforcementPolicy {
	if args == nil {
		return nil
	}

	return &otelconfiggrpc.KeepaliveEnforcementPolicy{
		MinTime:             args.MinTime,
		PermitWithoutStream: args.PermitWithoutStream,
	}
}

// GRPCClientArguments holds shared gRPC settings for components which launch
// gRPC clients.
// NOTE: When changing this structure, note that similar structures such as
// loadbalancing.GRPCClientArguments may also need to be changed.
type GRPCClientArguments struct {
	Endpoint string `alloy:"endpoint,attr"`

	Compression CompressionType `alloy:"compression,attr,optional"`

	TLS       TLSClientArguments        `alloy:"tls,block,optional"`
	Keepalive *KeepaliveClientArguments `alloy:"keepalive,block,optional"`

	ReadBufferSize  units.Base2Bytes  `alloy:"read_buffer_size,attr,optional"`
	WriteBufferSize units.Base2Bytes  `alloy:"write_buffer_size,attr,optional"`
	WaitForReady    bool              `alloy:"wait_for_ready,attr,optional"`
	Headers         map[string]string `alloy:"headers,attr,optional"`
	BalancerName    string            `alloy:"balancer_name,attr,optional"`
	Authority       string            `alloy:"authority,attr,optional"`

	// Auth is a binding to an otelcol.auth.* component extension which handles
	// authentication.
	// alloy name is auth instead of authentication to not break user interface compatibility.
	Authentication *auth.Handler `alloy:"auth,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *GRPCClientArguments) Convert() (*otelconfiggrpc.ClientConfig, error) {
	if args == nil {
		return nil, nil
	}

	opaqueHeaders := make(map[string]configopaque.String)
	for headerName, headerVal := range args.Headers {
		opaqueHeaders[headerName] = configopaque.String(headerVal)
	}

	// Configure authentication if args.Auth is set.
	var authentication *otelconfigauth.Authentication
	if args.Authentication != nil {
		ext, err := args.Authentication.GetExtension(auth.Client)
		if err != nil {
			return nil, err
		}

		authentication = &otelconfigauth.Authentication{AuthenticatorID: ext.ID}
	}

	// Set default value for `balancer_name` to sync up with upstream's
	balancerName := args.BalancerName
	if balancerName == "" {
		balancerName = DefaultBalancerName
	}

	return &otelconfiggrpc.ClientConfig{
		Endpoint: args.Endpoint,

		Compression: args.Compression.Convert(),

		TLSSetting: *args.TLS.Convert(),
		Keepalive:  args.Keepalive.Convert(),

		ReadBufferSize:  int(args.ReadBufferSize),
		WriteBufferSize: int(args.WriteBufferSize),
		WaitForReady:    args.WaitForReady,
		Headers:         opaqueHeaders,
		BalancerName:    balancerName,
		Authority:       args.Authority,

		Auth: authentication,
	}, nil
}

// Extensions exposes extensions used by args.
func (args *GRPCClientArguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	m := make(map[otelcomponent.ID]otelcomponent.Component)
	if args.Authentication != nil {
		ext, err := args.Authentication.GetExtension(auth.Client)
		if err != nil {
			return m
		}
		m[ext.ID] = ext.Extension
	}
	return m
}

// KeepaliveClientArguments holds shared keepalive settings for components
// which launch clients.
type KeepaliveClientArguments struct {
	PingWait            time.Duration `alloy:"ping_wait,attr,optional"`
	PingResponseTimeout time.Duration `alloy:"ping_response_timeout,attr,optional"`
	PingWithoutStream   bool          `alloy:"ping_without_stream,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *KeepaliveClientArguments) Convert() *otelconfiggrpc.KeepaliveClientConfig {
	if args == nil {
		return nil
	}

	return &otelconfiggrpc.KeepaliveClientConfig{
		Time:                args.PingWait,
		Timeout:             args.PingResponseTimeout,
		PermitWithoutStream: args.PingWithoutStream,
	}
}
