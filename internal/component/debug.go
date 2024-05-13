package component

import (
	"context"
	"fmt"
	"github.com/grafana/alloy/internal/featuregate"
	"io"
	"net/http"
	"sync"
	"time"
)

func init() {
	Register(Registration{
		Name:      "debug.test",
		Args:      DebugTestComponentArguments{},
		Stability: featuregate.StabilityExperimental,
		Build: func(opts Options, args Arguments) (Component, error) {
			return NewComponent(opts, args.(DebugTestComponentArguments))
		},
	})
}

type DebugTestComponent struct {
	mut  sync.Mutex
	args DebugTestComponentArguments
	o    Options
}

func NewComponent(o Options, c DebugTestComponentArguments) (*DebugTestComponent, error) {
	return &DebugTestComponent{
		o:    o,
		args: c,
	}, nil
}

func (c *DebugTestComponent) Run(ctx context.Context) error {
	instances := make([]*instance, c.args.NumberOfConcurrentStreams)
	for i := 0; i < c.args.NumberOfConcurrentStreams; i++ {
		id := i
		instances[i] = &instance{
			id: id,
		}
		go instances[i].start(ctx, "http://localhost:12345/api/v0/web/debug/prometheus.relabel.mutator")
	}
	ctx.Done()
	return nil
}

func (c *DebugTestComponent) Update(args Arguments) error {
	c.args = args.(DebugTestComponentArguments)
	return nil
}

type DebugTestComponentArguments struct {
	NumberOfConcurrentStreams int     `alloy:"number_of_concurrent_streams,attr"`
	ChurnPercent              float32 `alloy:"churn_percent,attr,optional"`
}

type instance struct {
	mut sync.Mutex
	id  int
}

func (i *instance) start(ctx context.Context, url string) error {
	r, err := http.Get(url)
	if err != nil {
		return err
	}
	buf := make([]byte, 1024)
	// Read the response body
	t := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			_ = r.Body.Close()
			return nil
		case <-t.C:
			_, err = r.Body.Read(buf)
			if err == io.EOF {
				println(fmt.Sprintf("%d eof", i.id))

				break
			}
			if err != nil {
				println(fmt.Sprintf("%d err %s", i.id, err))
				return err
			}
			println(fmt.Sprintf("%d read %d bytes", i.id, len(buf)))
		}
	}
}
