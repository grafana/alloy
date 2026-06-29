package string

import (
	"context"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/loki/pkg/push"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.string",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Source    string            `alloy:"source,attr"`
	ForwardTo loki.LogsReceiver `alloy:"forward_to,attr"`
}

var _ component.Component = (*Component)(nil)

type Component struct {
	mut sync.RWMutex

	opts     component.Options
	args     Arguments
	stringIn chan string
	receiver loki.LogsReceiver
}

func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:     o,
		receiver: args.ForwardTo,
		// channel capacity is added so that we capture the
		// first scrape/initial value of the source component
		stringIn: make(chan string, 10),
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case value := <-c.stringIn:
			entry := loki.Entry{
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      value,
				},
			}
			c.receiver.Chan() <- entry
		}
	}
}

func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()
	c.args = newArgs

	select {
	case c.stringIn <- c.args.Source:
	default:
	}

	return nil
}
