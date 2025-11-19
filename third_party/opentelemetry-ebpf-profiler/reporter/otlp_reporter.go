// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reporter // import "go.opentelemetry.io/ebpf-profiler/reporter"

import (
	"context"
	"crypto/tls"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/xsync"
	"go.opentelemetry.io/ebpf-profiler/reporter/internal/pdata"
	"go.opentelemetry.io/ebpf-profiler/reporter/samples"
	"google.golang.org/grpc/encoding/gzip"
)

// Assert that we implement the full Reporter interface.
var _ Reporter = (*OTLPReporter)(nil)

var gzipOption = grpc.UseCompressor(gzip.Name)

// OTLPReporter receives and transforms information to be OTLP/profiles compliant.
type OTLPReporter struct {
	*baseReporter

	// client for the connection to the receiver.
	client pprofileotlp.GRPCClient

	// To fill in the OTLP/profiles signal with the relevant information,
	// this structure holds in long-term storage information that might
	// be duplicated in other places but not accessible for OTLPReporter.

	// pkgGRPCOperationTimeout sets the time limit for GRPC requests.
	pkgGRPCOperationTimeout time.Duration
}

// NewOTLP returns a new instance of OTLPReporter
func NewOTLP(cfg *Config) (*OTLPReporter, error) {
	data, err := pdata.New(
		cfg.SamplesPerSecond,
		cfg.ExtraSampleAttrProd,
	)
	if err != nil {
		return nil, err
	}

	eventsTree := make(samples.TraceEventsTree)

	return &OTLPReporter{
		baseReporter: &baseReporter{
			cfg:         cfg,
			name:        cfg.Name,
			version:     cfg.Version,
			pdata:       data,
			traceEvents: xsync.NewRWMutex(eventsTree),
			runLoop: &runLoop{
				stopSignal: make(chan libpf.Void),
			},
		},
		pkgGRPCOperationTimeout: cfg.GRPCOperationTimeout,
		client:                  nil,
	}, nil
}

// Start sets up and manages the reporting connection to a OTLP backend.
func (r *OTLPReporter) Start(ctx context.Context) error {
	// Create a child context for reporting features
	ctx, cancelReporting := context.WithCancel(ctx)

	// Establish the gRPC connection before going on, waiting for a response
	// from the collectionAgent endpoint.
	// Use grpc.WithBlock() in setupGrpcConnection() for this to work.
	otlpGrpcConn, err := waitGrpcEndpoint(ctx, r.cfg)
	if err != nil {
		cancelReporting()
		r.runLoop.Stop()
		return err
	}
	r.client = pprofileotlp.NewGRPCClient(otlpGrpcConn)

	r.runLoop.Start(ctx, r.cfg.ReportInterval, func() {
		if err := r.reportOTLPProfile(ctx); err != nil {
			log.Errorf("Request failed: %v", err)
		} else {
			log.Debug("OTLP profile sent successfully")
		}
	}, func() {
		// Allow the GC to purge expired entries to avoid memory leaks.
		r.pdata.Purge()
	})

	// When Stop() is called and a signal to 'stop' is received, then:
	// - cancel the reporting functions currently running (using context)
	// - close the gRPC connection with collection-agent
	go func() {
		<-r.runLoop.stopSignal
		cancelReporting()
		if err := otlpGrpcConn.Close(); err != nil {
			log.Fatalf("Stopping connection of OTLP client client failed: %v", err)
		}
	}()

	return nil
}

// reportOTLPProfile creates and sends out an OTLP profile.
func (r *OTLPReporter) reportOTLPProfile(ctx context.Context) error {
	traceEventsPtr := r.traceEvents.WLock()
	reportedEvents := (*traceEventsPtr)
	newEvents := make(samples.TraceEventsTree)
	*traceEventsPtr = newEvents
	r.traceEvents.WUnlock(&traceEventsPtr)

	profiles, err := r.pdata.Generate(reportedEvents, r.name, r.version)
	if err != nil {
		log.Errorf("pdata: %v", err)
		return nil
	}
	if profiles.SampleCount() == 0 {
		log.Debugf("Skip sending of OTLP profile with no samples")
		return nil
	}

	req := pprofileotlp.NewExportRequestFromProfiles(profiles)

	reqCtx, ctxCancel := context.WithTimeout(ctx, r.pkgGRPCOperationTimeout)
	defer ctxCancel()
	_, err = r.client.Export(reqCtx, req, gzipOption)
	return err
}

// waitGrpcEndpoint waits until the gRPC connection is established.
func waitGrpcEndpoint(ctx context.Context, cfg *Config) (*grpc.ClientConn, error) {
	// Sleep with a fixed backoff time added of +/- 20% jitter
	tick := time.NewTicker(libpf.AddJitter(cfg.GRPCStartupBackoffTime, 0.2))
	defer tick.Stop()

	var retries uint32
	for {
		if collAgentConn, err := setupGrpcConnection(ctx, cfg); err != nil {
			if retries >= cfg.MaxGRPCRetries {
				return nil, err
			}
			retries++

			log.Warnf(
				"Failed to setup gRPC connection (try %d of %d): %v",
				retries,
				cfg.MaxGRPCRetries,
				err,
			)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-tick.C:
				continue
			}
		} else {
			return collAgentConn, nil
		}
	}
}

// setupGrpcConnection sets up a gRPC connection instrumented with our auth interceptor
func setupGrpcConnection(parent context.Context, cfg *Config) (*grpc.ClientConn, error) {
	//nolint:staticcheck
	opts := []grpc.DialOption{grpc.WithBlock(),
		grpc.WithUnaryInterceptor(cfg.GRPCClientInterceptor),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(cfg.MaxRPCMsgSize),
			grpc.MaxCallSendMsgSize(cfg.MaxRPCMsgSize)),
		grpc.WithReturnConnectionError(),
	}

	if cfg.DisableTLS {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		opts = append(opts,
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
				// Support only TLS1.3+ with valid CA certificates
				MinVersion:         tls.VersionTLS13,
				InsecureSkipVerify: false,
			})))
	}

	opts = append(opts, cfg.GRPCDialOptions...)

	ctx, cancel := context.WithTimeout(parent, cfg.GRPCConnectionTimeout)
	defer cancel()
	//nolint:staticcheck
	return grpc.DialContext(ctx, cfg.CollAgentAddr, opts...)
}
