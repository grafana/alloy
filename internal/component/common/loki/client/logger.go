package client

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/limit"
	"github.com/grafana/alloy/internal/component/common/loki/wal"
)

var (
	yellow = color.New(color.FgYellow)
	blue   = color.New(color.FgBlue)
)

func init() {
	if runtime.GOOS == "windows" {
		yellow.DisableColor()
		blue.DisableColor()
	}
}

type logger struct {
	*tabwriter.Writer
	sync.Mutex
	entries chan loki.Entry

	once sync.Once
}

// NewLogger creates a new client logger that logs entries instead of sending them.
func NewLogger(metrics *Metrics, log log.Logger, cfgs ...Config) (Client, error) {
	// make sure the clients config is valid
	c, err := NewManager(metrics, log, limit.Config{}, prometheus.NewRegistry(), wal.Config{}, NilNotifier, cfgs...)
	if err != nil {
		return nil, err
	}
	c.Stop()

	fmt.Println(yellow.Sprint("Clients configured:"))
	for _, cfg := range cfgs {
		yaml, err := yaml.Marshal(cfg)
		if err != nil {
			return nil, err
		}
		fmt.Println("----------------------")
		fmt.Println(string(yaml))
	}
	entries := make(chan loki.Entry)
	l := &logger{
		Writer:  tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0),
		entries: entries,
	}
	go l.run()
	return l, nil
}

func (l *logger) Stop() {
	l.once.Do(func() { close(l.entries) })
}

func (l *logger) Chan() chan<- loki.Entry {
	return l.entries
}

func (l *logger) run() {
	for e := range l.entries {
		_, _ = fmt.Fprint(l.Writer, blue.Sprint(e.Timestamp.Format("2006-01-02T15:04:05.999999999-0700")))
		_, _ = fmt.Fprint(l.Writer, "\t")
		_, _ = fmt.Fprint(l.Writer, yellow.Sprint(e.Labels.String()))
		_, _ = fmt.Fprint(l.Writer, "\t")
		_, _ = fmt.Fprint(l.Writer, e.Line)
		_, _ = fmt.Fprint(l.Writer, "\n")
		_ = l.Flush()
	}
}
func (l *logger) StopNow() { l.Stop() }

func (l *logger) Name() string {
	return ""
}
