package harness

import (
	"context"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
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

	lokiFanout *loki.Fanout
}

func NewSource(opts component.Options, args SourceArguments) (*Source, error) {
	s := &Source{
		opts: opts,

		lokiFanout: loki.NewFanout(args.ForwardTo.Logs),
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
	s.lokiFanout.UpdateChildren(newArgs.ForwardTo.Logs)
	return nil
}

func (s *Source) SendEntries(ctx context.Context, entries ...loki.Entry) error {
	for _, e := range entries {
		if err := s.lokiFanout.Send(ctx, e); err != nil {
			return err
		}
	}
	return nil
}
