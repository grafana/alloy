package componenttest

import (
	"context"
	"fmt"

	"github.com/grafana/agent/internal/component"
	mod "github.com/grafana/agent/internal/component/module"
	"github.com/grafana/agent/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "test.fail.module",
		Stability: featuregate.StabilityStable,
		Args:      TestFailArguments{},
		Exports:   mod.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			m, err := mod.NewModuleComponent(opts)
			if err != nil {
				return nil, err
			}
			if args.(TestFailArguments).Fail {
				return nil, fmt.Errorf("module told to fail")
			}
			err = m.LoadFlowSource(nil, args.(TestFailArguments).Content)
			if err != nil {
				return nil, err
			}
			return &TestFailModule{
				mc:      m,
				content: args.(TestFailArguments).Content,
				opts:    opts,
				fail:    args.(TestFailArguments).Fail,
				ch:      make(chan error),
			}, nil
		},
	})
}

type TestFailArguments struct {
	Content string `river:"content,attr"`
	Fail    bool   `river:"fail,attr,optional"`
}

type TestFailModule struct {
	content string
	opts    component.Options
	ch      chan error
	mc      *mod.ModuleComponent
	fail    bool
}

func (t *TestFailModule) Run(ctx context.Context) error {
	go t.mc.RunFlowController(ctx)
	<-ctx.Done()
	return nil
}

func (t *TestFailModule) UpdateContent(content string) error {
	t.content = content
	err := t.mc.LoadFlowSource(nil, t.content)
	return err
}

func (t *TestFailModule) Update(_ component.Arguments) error {
	return nil
}
