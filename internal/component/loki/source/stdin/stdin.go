package stdin

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/common/model"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.stdin",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	ForwardTo        []loki.LogsReceiver `alloy:"forward_to,attr"`
	Labels           map[string]string   `alloy:"labels,attr,optional"`
	ExitAfterReading bool                `alloy:"exit_after_reading,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		ExitAfterReading: true,
		Labels:           map[string]string{},
	}
}

func (a *Arguments) labelSet() model.LabelSet {
	labelSet := make(model.LabelSet, len(a.Labels))
	for k, v := range a.Labels {
		labelSet[model.LabelName(k)] = model.LabelValue(v)
	}
	return labelSet
}

type Component struct {
	opts               component.Options
	entriesChan        chan loki.Entry
	uncheckedCollector *util.UncheckedCollector

	rwMutex          sync.RWMutex
	labels           model.LabelSet
	exitAfterReading bool

	receiversMut sync.RWMutex
	receivers    []loki.LogsReceiver
}

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:               opts,
		entriesChan:        make(chan loki.Entry),
		receivers:          args.ForwardTo,
		uncheckedCollector: util.NewUncheckedCollector(nil),
	}
	opts.Registerer.MustRegister(c.uncheckedCollector)
	err := c.Update(args)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Component) Run(ctx context.Context) (err error) {
	defer c.stop()

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		level.Info(c.opts.Logger).Log("message", "no data found on stdin")
		// stdin does not have any data
		if c.exitAfterReading {
			if err := interruptAlloy(); err != nil {
				level.Error(c.opts.Logger).Log("failed to send SIGTERM to process", "err", err)
			}
		}
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go c.entriesLoop(ctx, &wg)

	wg.Add(1)
	go c.readFromStdin(ctx, &wg)

	wg.Wait()

	return nil
}

func (c *Component) entriesLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case entry := <-c.entriesChan:
			c.receiversMut.RLock()
			receivers := c.receivers
			c.receiversMut.RUnlock()

			for _, receiver := range receivers {
				select {
				case receiver.Chan() <- entry:
				case <-ctx.Done():
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}

}

func (c *Component) readFromStdin(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	reader := bufio.NewReader(os.Stdin)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		level.Info(c.opts.Logger).Log("message", "stdin goroutine postread")
		if err != nil {
			if err.Error() == "EOF" {
				if c.exitAfterReading {
					if err := interruptAlloy(); err != nil {
						level.Error(c.opts.Logger).Log("failed to send SIGTERM to process", "err", err)
					}
				}
				return
			}
			level.Error(c.opts.Logger).Log("failed to read from stdin", "err", err)
			return
		}
		line = strings.TrimSuffix(line, "\n")

		entry := loki.Entry{
			Labels: c.labels.Clone(),
			Entry: logproto.Entry{
				Timestamp: time.Now(),
				Line:      line,
			},
		}

		c.entriesChan <- entry
	}
}

func interruptAlloy() error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}

	return p.Signal(syscall.SIGTERM)
}

func (c *Component) Update(args component.Arguments) error {
	newArgs, ok := args.(Arguments)
	if !ok {
		return fmt.Errorf("invalid type of arguments: %T", args)
	}

	c.receiversMut.Lock()
	c.receivers = newArgs.ForwardTo
	c.receiversMut.Unlock()

	c.rwMutex.Lock()
	c.labels = newArgs.labelSet()
	c.exitAfterReading = newArgs.ExitAfterReading
	c.rwMutex.Unlock()

	return nil
}

func (c *Component) stop() {
	// Close the entries channel to unblock the Run method.
	close(c.entriesChan)
}
