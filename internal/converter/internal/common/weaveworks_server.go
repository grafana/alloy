package common

import (
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/dskit/server"

	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/converter/diag"
)

func DefaultWeaveWorksServerCfg() server.Config {
	cfg := server.Config{}
	// NOTE: due to a bug in promtail, the default server values for loki_push_api are not applied currently, so
	// we need to comment out the following line.
	//cfg.RegisterFlags(flag.NewFlagSet("", flag.PanicOnError))
	return cfg
}

func WeaveworksServerToAlloyServer(config server.Config) *fnet.ServerConfig {
	return &fnet.ServerConfig{
		HTTP: &fnet.HTTPConfig{
			ListenAddress:      config.HTTPListenAddress,
			ListenPort:         config.HTTPListenPort,
			ConnLimit:          config.HTTPConnLimit,
			ServerReadTimeout:  config.HTTPServerReadTimeout,
			ServerWriteTimeout: config.HTTPServerWriteTimeout,
			ServerIdleTimeout:  config.HTTPServerIdleTimeout,
			TLSConfig:          convertDSKitTLSToAlloy(config.HTTPTLSConfig),
		},
		GRPC: &fnet.GRPCConfig{
			ListenAddress:              config.GRPCListenAddress,
			ListenPort:                 config.GRPCListenPort,
			ConnLimit:                  config.GRPCConnLimit,
			MaxConnectionAge:           config.GRPCServerMaxConnectionAge,
			MaxConnectionAgeGrace:      config.GRPCServerMaxConnectionAgeGrace,
			MaxConnectionIdle:          config.GRPCServerMaxConnectionIdle,
			ServerMaxRecvMsg:           config.GRPCServerMaxRecvMsgSize,
			ServerMaxSendMsg:           config.GRPCServerMaxSendMsgSize,
			ServerMaxConcurrentStreams: config.GRPCServerMaxConcurrentStreams,
			TLSConfig:                  convertDSKitTLSToAlloy(config.GRPCTLSConfig),
		},
		GracefulShutdownTimeout: config.ServerGracefulShutdownTimeout,
	}
}

func ValidateWeaveWorksServerCfg(cfg server.Config) diag.Diagnostics {
	var (
		diags      diag.Diagnostics
		defaultCfg = DefaultWeaveWorksServerCfg()
	)

	if cfg.HTTPListenNetwork != defaultCfg.HTTPListenNetwork {
		diags.Add(diag.SeverityLevelError, "http_listen_network is not supported in server config")
	}
	if cfg.GRPCListenNetwork != defaultCfg.GRPCListenNetwork {
		diags.Add(diag.SeverityLevelError, "grpc_listen_network is not supported in server config")
	}
	if cfg.CipherSuites != defaultCfg.CipherSuites {
		diags.Add(diag.SeverityLevelError, "tls_cipher_suites is not supported in server config")
	}
	if cfg.MinVersion != defaultCfg.MinVersion {
		diags.Add(diag.SeverityLevelError, "tls_min_version is not supported in server config")
	}
	if cfg.RegisterInstrumentation {
		diags.Add(diag.SeverityLevelError, "register_instrumentation is not supported in server config")
	}
	if cfg.ServerGracefulShutdownTimeout != defaultCfg.ServerGracefulShutdownTimeout {
		diags.Add(diag.SeverityLevelError, "graceful_shutdown_timeout is not supported in server config")
	}
	if cfg.GRPCServerTime != defaultCfg.GRPCServerTime {
		diags.Add(diag.SeverityLevelError, "grpc_server_keepalive_time is not supported in server config")
	}
	if cfg.GRPCServerTimeout != defaultCfg.GRPCServerTimeout {
		diags.Add(diag.SeverityLevelError, "grpc_server_keepalive_timeout is not supported in server config")
	}
	if cfg.GRPCServerMinTimeBetweenPings != defaultCfg.GRPCServerMinTimeBetweenPings {
		diags.Add(diag.SeverityLevelError, "grpc_server_min_time_between_pings is not supported in server config")
	}
	if cfg.GRPCServerPingWithoutStreamAllowed != defaultCfg.GRPCServerPingWithoutStreamAllowed {
		diags.Add(diag.SeverityLevelError, "grpc_server_ping_without_stream_allowed is not supported in server config")
	}
	if cfg.LogFormat != defaultCfg.LogFormat {
		diags.Add(diag.SeverityLevelError, "log_format is not supported in server config")
	}
	if cfg.LogLevel.String() != defaultCfg.LogLevel.String() {
		diags.Add(diag.SeverityLevelError, "log_level is not supported in server config")
	}
	if cfg.LogSourceIPs != defaultCfg.LogSourceIPs {
		diags.Add(diag.SeverityLevelError, "log_source_ips_enabled is not supported in server config")
	}
	if cfg.LogSourceIPsHeader != defaultCfg.LogSourceIPsHeader {
		diags.Add(diag.SeverityLevelError, "log_source_ips_header is not supported in server config")
	}
	if cfg.LogSourceIPsRegex != defaultCfg.LogSourceIPsRegex {
		diags.Add(diag.SeverityLevelError, "log_source_ips_regex is not supported in server config")
	}
	if cfg.LogRequestAtInfoLevel != defaultCfg.LogRequestAtInfoLevel {
		diags.Add(diag.SeverityLevelError, "log_request_at_info_level_enabled is not supported in server config")
	}
	if cfg.PathPrefix != defaultCfg.PathPrefix {
		diags.Add(diag.SeverityLevelError, "http_path_prefix is not supported in server config")
	}

	return diags
}

func convertDSKitTLSToAlloy(dskitTLS server.TLSConfig) *fnet.TLSConfig {
	if !isTLSConfigured(dskitTLS) {
		return nil
	}

	return &fnet.TLSConfig{
		Cert:         dskitTLS.TLSCert,
		Key:          alloytypes.Secret(dskitTLS.TLSKey),
		CertFile:     dskitTLS.TLSCertPath,
		KeyFile:      dskitTLS.TLSKeyPath,
		ClientAuth:   dskitTLS.ClientAuth,
		ClientCAFile: dskitTLS.ClientCAs,
		ClientCA:     dskitTLS.ClientCAsText,
	}
}

func isTLSConfigured(cfg server.TLSConfig) bool {
	return cfg.TLSCertPath != "" || cfg.TLSCert != ""
}
