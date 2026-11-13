package harness

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
)

func registerSource() {
	component.Register(component.Registration{
		Name:      "pipelinetest.source",
		Stability: featuregate.StabilityExperimental,
		Args:      SourceArguments{},
		Exports:   SourceExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewSource(opts, args.(SourceArguments))
		},
	})
}

type SourceArguments struct {
	ForwardTo ForwardTo `alloy:"forward_to,block"`
}

type ForwardTo struct {
	Logs    []loki.LogsReceiver    `alloy:"logs,attr,optional"`
	Metrics []storage.AppendableV2 `alloy:"metrics,attr,optional"`
	Traces  []otelcol.Consumer     `alloy:"traces,attr,optional"`
}

type SourceExports struct{}

type Source struct {
	opts component.Options

	mut     sync.Mutex
	metrics []storage.AppendableV2
	traces  []otelcol.Consumer

	lokiPipelineV2 bool
	lokiFanout     *loki.Fanout
	lokiFanoutV2   *loki.FanoutV2
}

func NewSource(opts component.Options, args SourceArguments) (*Source, error) {
	svc, err := opts.GetServiceData(lokipipelinev2.ServiceName)
	if err != nil {
		return nil, err
	}

	s := &Source{
		opts:           opts,
		lokiPipelineV2: svc.(lokipipelinev2.Data).PipelineV2Enabled(),
		metrics:        args.ForwardTo.Metrics,
		traces:         args.ForwardTo.Traces,
	}

	if s.lokiPipelineV2 {
		s.lokiFanoutV2 = loki.NewFanoutV2(args.ForwardTo.Logs)
	} else {
		s.lokiFanout = loki.NewFanout(args.ForwardTo.Logs)
	}

	s.opts.OnStateChange(SourceExports{})

	return s, nil
}

var _ component.Component = (*Source)(nil)

func (s *Source) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (s *Source) Update(args component.Arguments) error {
	newArgs := args.(SourceArguments)
	s.mut.Lock()
	s.metrics = newArgs.ForwardTo.Metrics
	s.traces = newArgs.ForwardTo.Traces
	s.mut.Unlock()
	if s.lokiPipelineV2 {
		s.lokiFanoutV2.UpdateChildren(newArgs.ForwardTo.Logs)
	} else {
		s.lokiFanout.UpdateChildren(newArgs.ForwardTo.Logs)
	}
	return nil
}

func (s *Source) SendEntries(ctx context.Context, entries ...loki.Entry) error {
	if s.lokiPipelineV2 {
		batch := loki.NewBatch()
		for _, e := range entries {
			batch.AddEntry(e.Labels, e.Created(), e.Entry)
		}
		return s.lokiFanoutV2.Consume(ctx, batch)
	}

	for _, e := range entries {
		if err := s.lokiFanout.Send(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// InputSample is one Prometheus sample sent by a Source.
type InputSample struct {
	Labels    labels.Labels
	Value     float64
	Timestamp time.Time
	Metadata  metadata.Metadata
}

// SendSamples appends samples to every metrics receiver in one transaction
// per receiver. A zero timestamp means now.
func (s *Source) SendSamples(ctx context.Context, samples ...InputSample) error {
	s.mut.Lock()
	children := slices.Clone(s.metrics)
	s.mut.Unlock()

	if len(children) == 0 {
		return errors.New("source has no metrics receivers to send to")
	}

	now := time.Now()
	for _, child := range children {
		app := child.AppenderV2(ctx)
		for _, sample := range samples {
			ts := sample.Timestamp
			if ts.IsZero() {
				ts = now
			}
			_, err := app.Append(0, sample.Labels, 0, ts.UnixMilli(), sample.Value, nil, nil, storage.AppendV2Options{
				Metadata: sample.Metadata,
			})
			if err != nil {
				_ = app.Rollback()
				return fmt.Errorf("append sample %s: %w", sample.Labels, err)
			}
		}
		if err := app.Commit(); err != nil {
			return fmt.Errorf("commit samples: %w", err)
		}
	}
	return nil
}

// SendTraces sends traces to every traces consumer.
func (s *Source) SendTraces(ctx context.Context, td ptrace.Traces) error {
	s.mut.Lock()
	children := slices.Clone(s.traces)
	s.mut.Unlock()

	if len(children) == 0 {
		return errors.New("source has no traces consumers to send to")
	}

	for _, child := range children {
		// Consumers may mutate the data they receive, so each gets its own copy.
		cp := ptrace.NewTraces()
		td.CopyTo(cp)
		if err := child.ConsumeTraces(ctx, cp); err != nil {
			return fmt.Errorf("consume traces: %w", err)
		}
	}
	return nil
}
