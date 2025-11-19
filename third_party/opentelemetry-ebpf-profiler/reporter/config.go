// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reporter // import "go.opentelemetry.io/ebpf-profiler/reporter"

import (
	"time"

	"google.golang.org/grpc"

	"go.opentelemetry.io/ebpf-profiler/reporter/samples"
)

type Config struct {
	// Name defines the name of the agent.
	Name string

	// Version defines the version of the agent.
	Version string

	// CollAgentAddr defines the destination of the backend connection.
	CollAgentAddr string

	// MaxRPCMsgSize defines the maximum size of a gRPC message.
	MaxRPCMsgSize int

	// Disable secure communication with Collection Agent.
	DisableTLS bool
	// samplesPerSecond defines the number of samples per second.
	SamplesPerSecond int

	// Number of connection attempts to the collector after which we give up retrying.
	MaxGRPCRetries uint32

	GRPCOperationTimeout   time.Duration
	GRPCStartupBackoffTime time.Duration
	GRPCConnectionTimeout  time.Duration
	ReportInterval         time.Duration

	// gRPCInterceptor is the client gRPC interceptor, e.g., for sending gRPC metadata.
	GRPCClientInterceptor grpc.UnaryClientInterceptor

	// ExtraSampleAttrProd is an optional hook point for adding custom
	// attributes to samples.
	ExtraSampleAttrProd samples.SampleAttrProducer

	// GRPCDialOptions allows passing additional gRPC dial options when establishing
	// the connection to the collector. These options are appended after the default options.
	GRPCDialOptions []grpc.DialOption
}
