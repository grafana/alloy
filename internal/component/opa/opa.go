package opa

import (
	"context"
	"embed"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-policy-agent/opa/cmd"

	"github.com/prometheus/client_golang/prometheus"
)

// waitReadPeriod holds the time to wait before reading a file while the
// local.file component is running.
//
// This prevents local.file from updating too frequently and exporting partial
// writes.
const waitReadPeriod time.Duration = 30 * time.Millisecond

func init() {
	component.Register(component.Registration{
		Name:      "opa",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the opa component.
type Arguments struct {
	Input map[string]string `alloy:"input,attr,optional"`
}

// DefaultArguments provides the default arguments for the opa component.
var DefaultArguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Exports holds values which are exported by the opa component.
type Exports struct {
	// Content of the policy.
	Content string `alloy:"content,attr"`
}

// Component implements the opa component.
type Component struct {
	opts component.Options

	mut           sync.Mutex
	args          Arguments
	latestContent string

	healthMut sync.RWMutex
	health    component.Health

	// reloadCh is a buffered channel which is written to when the watched file
	// should be reloaded by the component.
	reloadCh     chan struct{}
	lastAccessed prometheus.Gauge
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

// New creates a new local.file component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts: o,

		reloadCh: make(chan struct{}, 1),
		lastAccessed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "local_file_timestamp_last_accessed_unix_seconds",
			Help: "The last successful access in unix seconds",
		}),
	}

	err := os.MkdirAll(c.opts.DataPath, 0755)
	if err != nil {
		return nil, err
	}

	err = o.Registerer.Register(c.lastAccessed)
	if err != nil {
		return nil, err
	}
	// Perform an update which will immediately set our exports to the initial
	// contents of the file.
	if err = c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()
	}()

	// Since Run _may_ get recalled if we're told to exit but still exist in the
	// config file, we may have prematurely destroyed the detector. If no
	// detector exists, we need to recreate it for Run to work properly.
	//
	// We ignore the error (indicating the file has disappeared) so we can allow
	// the detector to inform us when it comes back.
	//
	// TODO(rfratto): this is a design wart, and can hopefully be removed in
	// future iterations.
	// c.mut.Lock()
	// _ = c.configureDetector()
	// c.mut.Unlock()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.reloadCh:
			time.Sleep(waitReadPeriod)

			// We ignore the error here from readFile since readFile will log errors
			// and also report the error as the health of the component.
			c.mut.Lock()
			_ = c.readFile()
			c.mut.Unlock()
		}
	}
}

func (c *Component) readFile() error {
	// Force a re-load of the file outside of the update detection mechanism.
	// bb, err := os.ReadFile(c.args.Filename)
	// if err != nil {
	// 	c.setHealth(component.Health{
	// 		Health:     component.HealthTypeUnhealthy,
	// 		Message:    fmt.Sprintf("failed to read file: %s", err),
	// 		UpdateTime: time.Now(),
	// 	})
	// 	level.Error(c.opts.Logger).Log("msg", "failed to read file", "path", c.opts.DataPath, "err", err)
	// 	return err
	// }
	c.latestContent = "foo"
	c.lastAccessed.SetToCurrentTime()

	c.opts.OnStateChange(Exports{
		Content: c.latestContent,
	})

	c.setHealth(component.Health{
		Health:     component.HealthTypeHealthy,
		Message:    "read file",
		UpdateTime: time.Now(),
	})
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()
	c.args = newArgs

	err := os.RemoveAll(c.opts.DataPath)
	if err != nil {
		return err
	}
	err = os.MkdirAll(c.opts.DataPath, 0755)
	if err != nil {
		return err
	}

	for fn, content := range c.args.Input {
		err = os.WriteFile(path.Join(c.opts.DataPath, fn), []byte(content), 0644)
		if err != nil {
			return err
		}
	}

	bp := cmd.NewBuildParams()
	bp.BundleMode = true
	bp.OutputFile = path.Join(c.opts.DataPath, "bundle.tar.gz")
	_ = bp
	err = cmd.DoBuild(bp, []string{c.opts.DataPath})
	if err != nil {
		return err
	}

	// Force an immediate read of the file to report any potential errors early.
	// if err := c.readFile(); err != nil {
	// 	return fmt.Errorf("failed to read file: %w", err)
	// }

	// Each detector is dedicated to a single file path. We'll naively shut down
	// the existing detector (if any) before setting up a new one to make sure
	// the correct file is being watched in case the path changed between calls
	// to Update.
	// if c.detector != nil {
	// 	if err := c.detector.Close(); err != nil {
	// 		level.Error(c.opts.Logger).Log("msg", "failed to shut down old detector", "err", err)
	// 	}
	// 	c.detector = nil
	// }

	return nil
}

// CurrentHealth implements component.HealthComponent.
func (c *Component) CurrentHealth() component.Health {
	c.healthMut.RLock()
	defer c.healthMut.RUnlock()
	return c.health
}

func (c *Component) setHealth(h component.Health) {
	c.healthMut.Lock()
	defer c.healthMut.Unlock()
	c.health = h
}

//go:embed policies/*
var staticFiles embed.FS

// Handler serves metrics endpoint from the integration implementation.
func (c *Component) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// fs := http.FS(staticFiles)
		// fileServer := http.FileServer(fs)
		// fileServer.ServeHTTP(w, r)
		fs := http.FileServer(http.Dir(c.opts.DataPath))
		fs.ServeHTTP(w, r)
	})
}
