// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "go.opentelemetry.io/ebpf-profiler/collector/config"

import "time"

// Config is the configuration for the collector.
type Config struct {
	ReporterInterval       time.Duration `mapstructure:"reporter_interval"`
	MonitorInterval        time.Duration `mapstructure:"monitor_interval"`
	SamplesPerSecond       int           `mapstructure:"samples_per_second"`
	ProbabilisticInterval  time.Duration `mapstructure:"probabilistic_interval"`
	ProbabilisticThreshold uint          `mapstructure:"probabilistic_threshold"`
	Tracers                string        `mapstructure:"tracers"`
	ClockSyncInterval      time.Duration `mapstructure:"clock_sync_interval"`
	SendErrorFrames        bool          `mapstructure:"send_error_frames"`
	VerboseMode            bool          `mapstructure:"verbose_mode"`
	OffCPUThreshold        float64       `mapstructure:"off_cpu_threshold"`
	IncludeEnvVars         string        `mapstructure:"include_env_vars"`
	UProbeLinks            []string      `mapstructure:"u_probe_links"`
	LoadProbe              bool          `mapstructure:"load_probe"`
	MapScaleFactor         uint          `mapstructure:"map_scale_factor"`
	BPFVerifierLogLevel    uint          `mapstructure:"bpf_verifier_log_level"`
	NoKernelVersionCheck   bool          `mapstructure:"no_kernel_version_check"`
	MaxGRPCRetries         uint32        `mapstructure:"max_grpc_retries"`
	MaxRPCMsgSize          int           `mapstructure:"max_rpc_msg_size"`
}
