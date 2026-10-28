package harness

import (
	"context"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
)

func init() {
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
	Logs []loki.LogsReceiver `alloy:"logs,attr"`
}

type SourceExports struct{}

type Source struct {
	opts component.Options

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
