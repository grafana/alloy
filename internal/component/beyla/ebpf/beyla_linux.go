//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/common/model"
	"golang.org/x/sync/errgroup" //nolint:depguard

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/health"
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/subprocess"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
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
	opts component.Options

	argsUpdate chan Arguments

	// owned by Run() (no synchronisation)
	args             Arguments
	subprocessCancel context.CancelFunc
	subprocessGroup  *errgroup.Group
	restartTimer     *time.Timer
	otlpReceiverAddr string
	otlpServer       *http.Server
	otlpQueue        chan otlpItem
	otlpWorkerCancel context.CancelFunc
	otlpWorkersWG    sync.WaitGroup

	subprocess *subprocess.Handle
	health     *health.Reporter
}

var _ component.HealthComponent = (*Component)(nil)

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:       opts,
		args:       args,
		argsUpdate: make(chan Arguments, 1),
		subprocess: subprocess.New(),
		health:     health.New(),
	}

	if err := c.registerMetrics(opts.Registerer); err != nil {
		return nil, err
	}

	if err := c.publishExports(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	c.logDeprecationWarnings()

	c.restartTimer = time.NewTimer(0)
	defer c.restartTimer.Stop()

	for {
		if c.applyPendingArgsUpdate() {
			continue
		}

		select {
		case <-ctx.Done():
			c.stopSubprocess()
			return nil

		case newArgs := <-c.argsUpdate:
			c.handleArgsUpdate(newArgs)

		case <-c.restartTimer.C:
			c.handleSubprocessStart(ctx)
		}
	}
}

func (c *Component) Update(args component.Arguments) error {
	if err := c.publishExports(); err != nil {
		return err
	}

	c.argsUpdate <- args.(Arguments)

	return nil
}

func (c *Component) CurrentHealth() component.Health {
	return c.health.Current()
}

func (c *Component) Handler() http.Handler {
	return http.HandlerFunc(c.serveHTTP)
}

// serveHTTP reverse-proxies a request to the Beyla subprocess, routing /debug/pprof
// to the pprof port when enabled and the rest to the main subprocess port.
func (c *Component) serveHTTP(w http.ResponseWriter, r *http.Request) {
	addr, profilePort, ready := c.subprocess.ProxyTarget()

	if addr == "" {
		http.Error(w, "Beyla subprocess not started", http.StatusServiceUnavailable)
		return
	}

	targetAddr, ok := resolveTargetAddr(addr, profilePort, r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	target, err := url.Parse(targetAddr)
	if err != nil {
		c.opts.Logger.Error("failed to parse subprocess URL", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if errors.Is(err, context.Canceled) {
			c.opts.Logger.Debug("proxy request cancelled", "err", err)
			return
		}
		if !ready {
			c.opts.Logger.Debug("proxy error (subprocess initializing)", "err", err)
			w.Header().Set("Content-Type", "text/plain; version=0.0.4")
			w.WriteHeader(http.StatusOK)
			return
		}
		if isSubprocessGoneErr(err) {
			c.opts.Logger.Warn("subprocess connection unavailable", "err", err)
		} else {
			c.opts.Logger.Error("proxy error", "err", err)
		}
		http.Error(w, "subprocess unavailable", http.StatusBadGateway)
	}

	proxy.ServeHTTP(w, r)
}

func isSubprocessGoneErr(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE)
}

func resolveTargetAddr(addr string, profilePort int, path string) (targetAddr string, ok bool) {
	if strings.HasPrefix(path, "/debug/pprof") {
		if profilePort == 0 {
			return "", false
		}
		return fmt.Sprintf("http://127.0.0.1:%d", profilePort), true
	}

	return addr, true
}

func (c *Component) publishExports() error {
	baseTarget, err := c.baseTarget()

	if err != nil {
		return err
	}

	c.opts.OnStateChange(Exports{
		Targets: []discovery.Target{baseTarget},
	})

	return nil
}

func (c *Component) applyPendingArgsUpdate() bool {
	select {
	case newArgs := <-c.argsUpdate:
		c.handleArgsUpdate(newArgs)
		return true
	default:
		return false
	}
}

func (c *Component) registerMetrics(reg prometheus.Registerer) error {
	subReg := prometheus.WrapRegistererWith(prometheus.Labels{"subprocess": "beyla"}, reg)

	opts := collectors.ProcessCollectorOpts{
		PidFn:        c.subprocessPid,
		Namespace:    "alloy_resources",
		ReportErrors: false,
	}

	return subReg.Register(collectors.NewProcessCollector(opts))
}

func (c *Component) subprocessPid() (int, error) {
	if pid, ok := c.subprocess.Pid(); ok {
		return pid, nil
	}

	return 0, fmt.Errorf("subprocess not running")
}

func (c *Component) logDeprecationWarnings() {
	if c.args.Port != "" { //nolint:staticcheck // intentionally reads the deprecated field to warn
		c.opts.Logger.Warn("The 'open_port' field is deprecated. Use 'discovery.services' instead.")
	}

	if c.args.ExecutableName != "" { //nolint:staticcheck // intentionally reads the deprecated field to warn
		c.opts.Logger.Warn("The 'executable_name' field is deprecated. Use 'discovery.services' instead.")
	}

	if c.args.Debug {
		c.opts.Logger.Warn("The 'debug' field is deprecated. Use 'log_level = \"debug\"' instead.")
	}

	if len(c.args.Discovery.Services) > 0 {
		c.opts.Logger.Warn("discovery.services is deprecated, use discovery.instrument instead")
	}

	if len(c.args.Discovery.ExcludeServices) > 0 {
		c.opts.Logger.Warn("discovery.exclude_services is deprecated, use discovery.exclude_instrument instead")
	}

	if len(c.args.Discovery.DefaultExcludeServices) > 0 {
		c.opts.Logger.Warn("discovery.default_exclude_services is deprecated, use discovery.default_exclude_instrument instead")
	}
}

func (c *Component) handleArgsUpdate(newArgs Arguments) {
	c.args = getLatestArgsFromChannel(c.argsUpdate, newArgs)

	c.stopSubprocess()

	c.subprocess.ResetRestartTracking()
	c.restartTimer.Reset(0)
}

func (c *Component) stopSubprocess() {
	if c.subprocessCancel == nil {
		return
	}

	c.subprocessCancel()
	c.opts.Logger.Info("waiting for Beyla subprocess to terminate")

	if err := c.subprocessGroup.Wait(); err != nil {
		c.opts.Logger.Error("Beyla subprocess terminated with error", "err", err)
	}

	c.cleanup()

	c.subprocessCancel = nil
	c.subprocessGroup = nil
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

func (c *Component) handleSubprocessStart(ctx context.Context) {
	c.stopSubprocess()

	restartCount := c.subprocess.RecordStart()

	if restartCount > 0 {
		c.opts.Logger.Info("restarting Beyla subprocess", "restart_count", restartCount)
	} else {
		c.opts.Logger.Info("starting Beyla subprocess")
	}

	newCtx, cancelFunc := context.WithCancel(ctx)

	if err := c.setupSubprocess(); err != nil {
		cancelFunc()
		return
	}

	g, launchCtx := errgroup.WithContext(newCtx)
	g.Go(func() error { return c.runSubprocess(launchCtx) })
	g.Go(func() error { return c.watchdogLoop(launchCtx) })

	go c.monitorSubprocess(ctx, g)

	c.subprocessCancel = cancelFunc
	c.subprocessGroup = g
}

// monitorSubprocess waits for the subprocess goroutines to exit and schedules a
// restart if they failed for a reason other than the parent context being cancelled
// (i.e. a crash, not an intentional stop).
func (c *Component) monitorSubprocess(ctx context.Context, g *errgroup.Group) {
	err := g.Wait()

	if ctx.Err() == nil && err != nil {
		c.opts.Logger.Error("Beyla subprocess crashed, scheduling restart", "err", err)
		c.health.SetUnhealthy(err)
		c.scheduleRestart()
	}
}

func (c *Component) setupSubprocess() error {
	exePath, cleanupBinary, err := c.extractBeylaExecutable()

	if err != nil {
		c.opts.Logger.Error("failed to extract Beyla binary", "err", err)
		c.health.SetUnhealthy(err)
		c.scheduleRestart()
		return err
	}

	c.subprocess.SetBinary(exePath, cleanupBinary)

	if err := c.startOTLPReceiver(); err != nil {
		c.opts.Logger.Error("failed to start OTLP receiver", "err", err)
		c.health.SetUnhealthy(err)
		c.cleanup()
		c.scheduleRestart()
		return err
	}

	port, err := findFreePort()

	if err != nil {
		c.opts.Logger.Error("failed to allocate port", "err", err)
		c.health.SetUnhealthy(err)
		c.cleanup()
		c.scheduleRestart()
		return err
	}

	c.subprocess.SetListen(port, fmt.Sprintf("http://127.0.0.1:%d", port))

	if err := c.allocateProfilePort(); err != nil {
		c.opts.Logger.Error("failed to allocate Beyla profile port", "err", err)
		c.health.SetUnhealthy(err)
		c.cleanup()
		c.scheduleRestart()
		return err
	}

	c.subprocess.SetHealthAddr(abstractSocketAddr("health", c.opts.ID))

	configPath, cleanupConfig, err := c.writeConfigFile()

	if err != nil {
		c.opts.Logger.Error("failed to write config", "err", err)
		c.health.SetUnhealthy(err)
		c.cleanup()
		c.scheduleRestart()
		return err
	}

	c.subprocess.SetConfig(configPath, cleanupConfig)

	return nil
}

func (c *Component) allocateProfilePort() error {
	data, err := c.opts.GetServiceData(http_service.ServiceName)

	if err != nil {
		return fmt.Errorf("failed to get HTTP service data: %w", err)
	}

	if !data.(http_service.Data).EnablePProf {
		return nil
	}

	port, err := findFreePort()

	if err != nil {
		return err
	}

	c.subprocess.SetProfilePort(port)

	return nil
}

func (c *Component) scheduleRestart() {
	backoff, count, ok := c.subprocess.NextBackoff()

	if !ok {
		c.opts.Logger.Error("Beyla subprocess exceeded maximum restart attempts, giving up", "max_restarts", subprocess.MaxRestarts)
		return
	}

	c.opts.Logger.Info("scheduling subprocess restart", "backoff", backoff, "restart_count", count)
	c.restartTimer.Reset(backoff)
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
	c.otlpReceiverAddr = ""
	c.subprocess.Reset()
}
