package componenttest

import (
	"context"
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	mod "github.com/grafana/alloy/internal/runtime/internal/testcomponents/module"
)

func init() {
	component.Register(component.Registration{
		Name:      "test.fail.module",
		Stability: featuregate.StabilityGenerallyAvailable,
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
			err = m.LoadAlloySource(nil, args.(TestFailArguments).Content)
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
	Content string `alloy:"content,attr"`
	Fail    bool   `alloy:"fail,attr,optional"`
}

type TestFailModule struct {
	content string
	opts    component.Options
	ch      chan error
	mc      *mod.ModuleComponent
	fail    bool
}

func (t *TestFailModule) Run(ctx context.Context) error {
	go t.mc.RunAlloyController(ctx)
	<-ctx.Done()
	return nil
}

func (t *TestFailModule) UpdateContent(content string) error {
	t.content = content
	err := t.mc.LoadAlloySource(nil, t.content)
	return err
}

func (t *TestFailModule) Update(_ component.Arguments) error {
	return nil
}
