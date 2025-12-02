//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/common/model"
	"golang.org/x/sync/errgroup" //nolint:depguard

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	http_service "github.com/grafana/alloy/internal/service/http"
)

func init() {
	component.Register(component.Registration{
		Name:      "beyla.ebpf",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Component struct {
	opts       component.Options
	mut        sync.Mutex
	args       Arguments
	argsUpdate chan Arguments

	// Subprocess-specific fields
	subprocessPort int       // Port where Beyla subprocess listens
	subprocessAddr string    // Full address (http://localhost:PORT)
	subprocessCmd  *exec.Cmd // The running subprocess
	beylaExePath   string    // Path to extracted Beyla binary
	beylaExeClose  func()    // Closes the memfd; called early after exec, cleanup is fallback
	configPath     string    // Path to config file
	cleanupFuncs   []func()  // Cleanup functions for temp files

	// OTLP receiver for traces and metrics (when Output is configured)
	otlpReceiverPort int          // Port where OTLP receiver listens for signals from Beyla
	otlpServer       *http.Server // HTTP server for OTLP receiver

	// Restart tracking
	restartCount    int
	lastRestartTime time.Time
	restartBackoff  time.Duration
	subprocessReady bool // true after first successful health check

	healthMut sync.RWMutex
	health    component.Health
}

var _ component.HealthComponent = (*Component)(nil)

const (
	SamplerAlwaysOn                = "always_on"
	SamplerAlwaysOff               = "always_off"
	SamplerTraceIDRatio            = "traceidratio"
	SamplerParentBasedAlwaysOn     = "parentbased_always_on"
	SamplerParentBasedAlwaysOff    = "parentbased_always_off"
	SamplerParentBasedTraceIDRatio = "parentbased_traceidratio"
)

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:       opts,
		args:       args,
		argsUpdate: make(chan Arguments, 1),
	}

	if err := c.registerMetrics(opts.Registerer); err != nil {
		return nil, err
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Component) registerMetrics(reg prometheus.Registerer) error {
	subReg := prometheus.WrapRegistererWith(prometheus.Labels{"subprocess": "beyla"}, reg)
	return subReg.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{
		PidFn: func() (int, error) {
			c.mut.Lock()
			cmd := c.subprocessCmd
			c.mut.Unlock()
			if cmd == nil || cmd.Process == nil {
				return 0, fmt.Errorf("subprocess not running")
			}
			return cmd.Process.Pid, nil
		},
		Namespace:    "alloy_resources",
		ReportErrors: false,
	}))
}

func (c *Component) logDeprecationWarnings() {
	if c.args.Port != "" {
		level.Warn(c.opts.Logger).Log("msg", "The 'open_port' field is deprecated. Use 'discovery.services' instead.")
	}
	if c.args.ExecutableName != "" {
		level.Warn(c.opts.Logger).Log("msg", "The 'executable_name' field is deprecated. Use 'discovery.services' instead.")
	}
	if c.args.Debug {
		level.Warn(c.opts.Logger).Log("msg", "The 'debug' field is deprecated. Use 'log_level = \"debug\"' instead.")
	}
	if len(c.args.Discovery.Services) > 0 {
		level.Warn(c.opts.Logger).Log("msg", "discovery.services is deprecated, use discovery.instrument instead")
	}
	if len(c.args.Discovery.ExcludeServices) > 0 {
		level.Warn(c.opts.Logger).Log("msg", "discovery.exclude_services is deprecated, use discovery.exclude_instrument instead")
	}
	if len(c.args.Discovery.DefaultExcludeServices) > 0 {
		level.Warn(c.opts.Logger).Log("msg", "discovery.default_exclude_services is deprecated, use discovery.default_exclude_instrument instead")
	}
}

func (c *Component) drainPendingArgsUpdates() {
	select {
	case latestArgs := <-c.argsUpdate:
		latestArgs = getLatestArgsFromChannel(c.argsUpdate, latestArgs)
		c.mut.Lock()
		c.args = latestArgs
		c.mut.Unlock()
	default:
	}
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	c.logDeprecationWarnings()

	c.mut.Lock()
	c.restartBackoff = 1 * time.Second
	c.mut.Unlock()

	c.drainPendingArgsUpdates()

	var cancel context.CancelFunc
	var cancelG *errgroup.Group
	restartTimer := time.NewTimer(0)
	defer restartTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			if cancel != nil {
				cancel()
			}
			return nil

		case newArgs := <-c.argsUpdate:
			c.handleArgsUpdate(newArgs, cancel, cancelG, restartTimer)

		case <-restartTimer.C:
			var err error
			cancel, cancelG, err = c.handleSubprocessStart(ctx, cancel, cancelG, restartTimer)
			if err != nil {
				continue
			}
		}
	}
}

const maxRestartCount = 10

func (c *Component) scheduleRestart(timer *time.Timer) {
	c.mut.Lock()
	defer c.mut.Unlock()

	if c.restartCount >= maxRestartCount {
		level.Error(c.opts.Logger).Log("msg", "Beyla subprocess exceeded maximum restart attempts, giving up", "max_restarts", maxRestartCount)
		return
	}

	backoff := c.restartBackoff
	c.restartBackoff = c.restartBackoff * 2
	if c.restartBackoff > 30*time.Second {
		c.restartBackoff = 30 * time.Second
	}

	level.Info(c.opts.Logger).Log("msg", "scheduling subprocess restart", "backoff", backoff, "restart_count", c.restartCount)
	timer.Reset(backoff)
}

func (c *Component) handleArgsUpdate(newArgs Arguments, cancel context.CancelFunc, cancelG *errgroup.Group, restartTimer *time.Timer) {
	newArgs = getLatestArgsFromChannel(c.argsUpdate, newArgs)
	c.args = newArgs

	if cancel != nil {
		cancel()
		level.Info(c.opts.Logger).Log("msg", "waiting for Beyla subprocess to terminate")
		if err := cancelG.Wait(); err != nil {
			level.Error(c.opts.Logger).Log("msg", "Beyla subprocess terminated with error", "err", err)
			c.reportUnhealthy(err)
		}
		c.cleanup()
	}

	// Reset restart tracking on config change.
	c.mut.Lock()
	c.restartCount = 0
	c.restartBackoff = 1 * time.Second
	c.mut.Unlock()

	restartTimer.Reset(0)
}

func (c *Component) handleSubprocessStart(ctx context.Context, cancel context.CancelFunc, cancelG *errgroup.Group, restartTimer *time.Timer) (context.CancelFunc, *errgroup.Group, error) {
	if cancel != nil {
		cancel()
		level.Info(c.opts.Logger).Log("msg", "waiting for Beyla subprocess to terminate before restart")
		if err := cancelG.Wait(); err != nil {
			level.Error(c.opts.Logger).Log("msg", "Beyla subprocess terminated with error", "err", err)
		}
		c.cleanup()
	}

	c.mut.Lock()
	restartCount := c.restartCount
	c.restartCount++
	c.lastRestartTime = time.Now()
	c.mut.Unlock()

	if restartCount > 0 {
		level.Info(c.opts.Logger).Log("msg", "restarting Beyla subprocess", "restart_count", restartCount)
	} else {
		level.Info(c.opts.Logger).Log("msg", "starting Beyla subprocess")
	}

	newCtx, cancelFunc := context.WithCancel(ctx)

	if err := c.setupSubprocess(restartTimer); err != nil {
		cancelFunc()
		return cancel, cancelG, err
	}

	g, launchCtx := errgroup.WithContext(newCtx)
	g.Go(func() error { return c.runSubprocess(launchCtx) })
	g.Go(func() error { return c.healthCheckLoop(launchCtx) })

	// Monitor subprocess in background; schedule restart on unexpected exit.
	go func() {
		err := g.Wait()
		if ctx.Err() == nil && err != nil {
			level.Error(c.opts.Logger).Log("msg", "Beyla subprocess crashed, scheduling restart", "err", err)
			c.reportUnhealthy(err)
			c.scheduleRestart(restartTimer)
		}
	}()

	return cancelFunc, g, nil
}

func (c *Component) setupSubprocess(restartTimer *time.Timer) error {
	exePath, cleanupBinary, err := c.extractBeylaExecutable()
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to extract Beyla binary", "err", err)
		c.reportUnhealthy(err)
		c.scheduleRestart(restartTimer)
		return err
	}

	c.mut.Lock()
	c.beylaExePath = exePath
	c.beylaExeClose = cleanupBinary
	c.mut.Unlock()

	if err := c.startOTLPReceiver(); err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to start OTLP receiver", "err", err)
		c.reportUnhealthy(err)
		c.cleanup()
		c.scheduleRestart(restartTimer)
		return err
	}

	port, err := findFreePort()
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to allocate port", "err", err)
		c.reportUnhealthy(err)
		c.cleanup()
		c.scheduleRestart(restartTimer)
		return err
	}

	c.mut.Lock()
	c.subprocessPort = port
	c.subprocessAddr = fmt.Sprintf("http://127.0.0.1:%d", port)
	c.mut.Unlock()

	configPath, cleanupConfig, err := c.writeConfigFile()
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to write config", "err", err)
		c.reportUnhealthy(err)
		c.cleanup()
		c.scheduleRestart(restartTimer)
		return err
	}

	c.mut.Lock()
	c.configPath = configPath
	c.cleanupFuncs = append(c.cleanupFuncs, cleanupConfig)
	c.mut.Unlock()

	return nil
}

func getLatestArgsFromChannel[A any](ch chan A, current A) A {
	for {
		select {
		case x := <-ch:
			current = x
		default:
			return current
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()
	baseTarget, err := c.baseTarget()
	if err != nil {
		return err
	}
	c.opts.OnStateChange(Exports{
		Targets: []discovery.Target{baseTarget},
	})

	newArgs := args.(Arguments)
	c.argsUpdate <- newArgs
	return nil
}

func (c *Component) baseTarget() (discovery.Target, error) {
	data, err := c.opts.GetServiceData(http_service.ServiceName)
	if err != nil {
		return discovery.EmptyTarget, fmt.Errorf("failed to get HTTP information: %w", err)
	}
	httpData := data.(http_service.Data)

	return discovery.NewTargetFromMap(map[string]string{
		model.AddressLabel:     httpData.MemoryListenAddr,
		model.SchemeLabel:      "http",
		model.MetricsPathLabel: path.Join(httpData.HTTPPathForComponent(c.opts.ID), "metrics"),
		"instance":             defaultInstance(),
		"job":                  "beyla",
	}), nil
}

func (c *Component) reportUnhealthy(err error) {
	c.healthMut.Lock()
	defer c.healthMut.Unlock()
	c.health = component.Health{
		Health:     component.HealthTypeUnhealthy,
		Message:    err.Error(),
		UpdateTime: time.Now(),
	}
}

func (c *Component) reportHealthy() {
	c.healthMut.Lock()
	defer c.healthMut.Unlock()
	c.health = component.Health{
		Health:     component.HealthTypeHealthy,
		UpdateTime: time.Now(),
	}
}

func (c *Component) CurrentHealth() component.Health {
	c.healthMut.RLock()
	defer c.healthMut.RUnlock()
	return c.health
}

func (c *Component) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.mut.Lock()
		addr := c.subprocessAddr
		ready := c.subprocessReady
		c.mut.Unlock()

		if addr == "" {
			http.Error(w, "Beyla subprocess not started", http.StatusServiceUnavailable)
			return
		}

		target, err := url.Parse(addr)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to parse subprocess URL", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			if ready {
				level.Error(c.opts.Logger).Log("msg", "proxy error", "err", err)
				http.Error(w, "subprocess unavailable", http.StatusBadGateway)
			} else {
				level.Debug(c.opts.Logger).Log("msg", "proxy error (subprocess initializing)", "err", err)
				// Return empty 200 so scrapers don't log warnings during startup.
				w.Header().Set("Content-Type", "text/plain; version=0.0.4")
				w.WriteHeader(http.StatusOK)
			}
		}

		proxy.ServeHTTP(w, r)
	})
}

func defaultInstance() string {
	hostname := os.Getenv("HOSTNAME")
	if hostname != "" {
		return hostname
	}
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

func (c *Component) cleanup() {
	c.stopOTLPReceiver()

	c.mut.Lock()
	defer c.mut.Unlock()

	if c.beylaExeClose != nil {
		c.beylaExeClose()
		c.beylaExeClose = nil
	}
	for _, cleanupFunc := range c.cleanupFuncs {
		cleanupFunc()
	}
	c.cleanupFuncs = nil
	c.beylaExePath = ""
	c.configPath = ""
	c.otlpReceiverPort = 0
	c.subprocessReady = false
}
