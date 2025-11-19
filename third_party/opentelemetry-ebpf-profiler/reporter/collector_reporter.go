// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reporter // import "go.opentelemetry.io/ebpf-profiler/reporter"

import (
	"context"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/collector/consumer/xconsumer"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/xsync"
	"go.opentelemetry.io/ebpf-profiler/reporter/internal/pdata"
	"go.opentelemetry.io/ebpf-profiler/reporter/samples"
)

// Assert that we implement the full Reporter interface.
var _ Reporter = (*CollectorReporter)(nil)

// CollectorReporter receives and transforms information to be Collector Collector compliant.
type CollectorReporter struct {
	*baseReporter

	nextConsumer xconsumer.Profiles
}

// NewCollector builds a new CollectorReporter
func NewCollector(cfg *Config, nextConsumer xconsumer.Profiles) (*CollectorReporter, error) {
	data, err := pdata.New(
		cfg.SamplesPerSecond,
		cfg.ExtraSampleAttrProd,
	)
	if err != nil {
		return nil, err
	}

	tree := make(samples.TraceEventsTree)

	return &CollectorReporter{
		baseReporter: &baseReporter{
			cfg:         cfg,
			name:        cfg.Name,
			version:     cfg.Version,
			pdata:       data,
			traceEvents: xsync.NewRWMutex(tree),
			runLoop: &runLoop{
				stopSignal: make(chan libpf.Void),
			},
		},
		nextConsumer: nextConsumer,
	}, nil
}

func (r *CollectorReporter) Start(ctx context.Context) error {
	// Create a child context for reporting features
	ctx, cancelReporting := context.WithCancel(ctx)

	r.runLoop.Start(ctx, r.cfg.ReportInterval, func() {
		if err := r.reportProfile(context.Background()); err != nil {
			log.Errorf("Request failed: %v", err)
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
	}()

	return nil
}

// reportProfile creates and sends out a profile.
func (r *CollectorReporter) reportProfile(ctx context.Context) error {
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
		log.Debugf("Skip sending profile with no samples")
		return nil
	}

	return r.nextConsumer.ConsumeProfiles(ctx, profiles)
}
