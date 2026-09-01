package supportbundle

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/grafana/alloy/extension/supportbundle/internal/gather"
)

var (
	_ extension.Extension                         = (*supportBundleExtension)(nil)
	_ extensioncapabilities.ConfigSnapshotWatcher = (*supportBundleExtension)(nil)
	_ componentstatus.Watcher                     = (*supportBundleExtension)(nil)
)

const bundleRoot = "otelcol-support-bundle"

// collectionMu serializes bundle generation across ALL extension instances in
// the process. The gatherers use process-global state (the pprof profiler and
// the shared log sink), so only one bundle may run per process at a time, even
// when the collector has more than one supportbundle instance.
var collectionMu sync.Mutex

// supportBundleExtension serves a diagnostics bundle over HTTP.
type supportBundleExtension struct {
	cfg      *Config
	settings extension.Settings

	// configGatherer owns the stored config snapshot. It is also a sync gatherer.
	configGatherer *gather.Config

	// statusGatherer owns the recorded component statuses. It is also a sync gatherer.
	statusGatherer *gather.Status

	syncGatherers  []gather.Gatherer
	asyncGatherers []gather.AsyncGatherer

	server *http.Server
	ln     net.Listener

	startTime time.Time
}

func newSupportBundleExtension(cfg *Config, settings extension.Settings) *supportBundleExtension {
	return &supportBundleExtension{
		cfg:            cfg,
		settings:       settings,
		configGatherer: &gather.Config{},
		statusGatherer: gather.NewStatus(),
	}
}

func (e *supportBundleExtension) Start(ctx context.Context, host component.Host) error {
	e.startTime = time.Now()
	e.syncGatherers = []gather.Gatherer{
		gather.Metadata{},
		gather.PprofSnapshot{},
		e.configGatherer,
		e.statusGatherer,
		gather.Environment{Extra: e.cfg.EnvironmentVariables},
		gather.FeatureGates{},
		// Logs snapshots the always-on ring; it is a point-in-time read.
		gather.Logs{Sink: gather.LogCapture.Sink},
	}
	e.asyncGatherers = []gather.AsyncGatherer{
		// metrics is first so its start sample is taken before CPU profiling begins.
		gather.Metrics{
			// Read the effective (expanded) config live through the configGatherer
			// so ${env:...} endpoints resolve and config reloads are picked up. The
			// address is used only to scrape; the effective config is never written
			// to the bundle.
			Conf: e.configGatherer.EffectiveConf,
			// Keep the timeout short so a slow endpoint does not stall the bundle.
			Client: &http.Client{Timeout: 5 * time.Second},
		},
		gather.PprofWindow{},
	}

	// Turn on the log ring if the operator configured a size. Capture stays a
	// lock-free no-op until this is called with a positive size.
	gather.LogCapture.Sink.Enable(e.cfg.LogBufferSize)

	// Log capture needs the zap sink. Warn if registration failed so the
	// operator knows logs.txt will be empty.
	if gather.LogCapture.Err != nil {
		e.settings.Logger.Warn("support bundle log capture is unavailable",
			zap.Error(gather.LogCapture.Err))
	}

	mux := http.NewServeMux()
	mux.HandleFunc(e.cfg.Path, e.handleBundle)

	server, err := e.cfg.ServerConfig.ToServer(ctx, host.GetExtensions(), e.settings.TelemetrySettings, mux)
	if err != nil {
		return err
	}
	e.server = server

	ln, err := e.cfg.ServerConfig.ToListener(ctx)
	if err != nil {
		// Close the server we just built so Start does not leak it.
		_ = e.server.Close()
		e.server = nil
		return fmt.Errorf("failed to bind to address %s: %w", e.cfg.ServerConfig.NetAddr.Endpoint, err)
	}
	e.ln = ln

	go func() {
		if serveErr := e.server.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			e.settings.Logger.Error("support bundle server stopped", zap.Error(serveErr))
		}
	}()

	return nil
}

// NotifyConfigSnapshot forwards the collector's configuration to the gatherer
// that owns it. The metrics gatherer reads the effective config live through the
// same configGatherer, so it picks up the new snapshot too; there is nothing
// extra to update here.
func (e *supportBundleExtension) NotifyConfigSnapshot(_ context.Context, snapshot extensioncapabilities.ConfigSnapshot) error {
	e.configGatherer.Store(snapshot)
	return nil
}

// ComponentStatusChanged forwards a component status event to the gatherer that
// owns it. The collector calls this for every component.
func (e *supportBundleExtension) ComponentStatusChanged(source *componentstatus.InstanceID, event *componentstatus.Event) {
	e.statusGatherer.Record(source, event)
}

func (e *supportBundleExtension) Shutdown(ctx context.Context) error {
	if e.server == nil {
		return nil
	}
	return e.server.Shutdown(ctx)
}

// addr returns the bound listener address. It supports tests that bind to port 0.
func (e *supportBundleExtension) addr() net.Addr {
	if e.ln == nil {
		return nil
	}
	return e.ln.Addr()
}

// startedGatherer is an async gatherer that is collecting over the window.
type startedGatherer struct {
	name   string
	finish gather.FinishFunc
}

// wait blocks for the window duration. It returns early if the request ends.
func (e *supportBundleExtension) wait(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

func (e *supportBundleExtension) handleBundle(w http.ResponseWriter, r *http.Request) {
	duration := e.resolveDuration(r)

	// Only one bundle runs at a time, per process. Profiles use process-wide state.
	collectionMu.Lock()
	defer collectionMu.Unlock()

	opts := gather.Options{
		Duration:           duration,
		BuildInfo:          e.settings.BuildInfo,
		ResourceAttributes: resourceAttributes(e.settings.Resource),
		StartTime:          e.startTime,
	}

	var files []gather.File
	var gatherErrs []string

	// Start the async collectors. They sample over the shared window.
	var started []startedGatherer
	for _, ag := range e.asyncGatherers {
		finish, err := ag.Start(r.Context(), opts)
		if err != nil {
			e.settings.Logger.Error("support bundle gatherer failed to start",
				zap.String("gatherer", ag.Name()), zap.Error(err))
			gatherErrs = append(gatherErrs, fmt.Sprintf("%s: %v", ag.Name(), err))
			continue
		}
		if finish != nil {
			started = append(started, startedGatherer{name: ag.Name(), finish: finish})
		}
	}

	// Snapshot gatherers run while the window elapses, not after it. This
	// overlaps their work with the window and captures state during it.
	type syncResult struct {
		files []gather.File
		errs  []string
	}
	syncCh := make(chan syncResult, 1)
	go func() {
		// Recover so a gatherer panic cannot crash the collector or deadlock the
		// handler. net/http only recovers the request goroutine, not this child.
		defer func() {
			if r := recover(); r != nil {
				e.settings.Logger.Error("support bundle sync gatherers panicked",
					zap.Any("panic", r))
				syncCh <- syncResult{errs: []string{fmt.Sprintf("sync gatherers panicked: %v", r)}}
			}
		}()
		f, errs := e.runSyncGatherers(r.Context(), opts)
		syncCh <- syncResult{files: f, errs: errs}
	}()

	if opts.Duration > 0 && len(started) > 0 {
		e.wait(r.Context(), opts.Duration)
	}

	sr := <-syncCh
	files = append(files, sr.files...)
	gatherErrs = append(gatherErrs, sr.errs...)

	// Finish in reverse (LIFO) order, so a collector started later closes before
	// one started earlier. This matters because metrics starts first (its start
	// sample must precede CPU profiling); finishing in reverse means the pprof
	// window closes (StopCPUProfile, mutex restore) before the metrics end scrape
	// runs, keeping the scrape out of the profile window.
	for i := len(started) - 1; i >= 0; i-- {
		gathered, err := e.finishGatherer(r.Context(), started[i])
		if err != nil {
			e.settings.Logger.Error("support bundle gatherer failed to finish",
				zap.String("gatherer", started[i].name), zap.Error(err))
			gatherErrs = append(gatherErrs, fmt.Sprintf("%s: %v", started[i].name, err))
		}
		files = append(files, gathered...)
	}

	if len(gatherErrs) > 0 {
		content := strings.Join(gatherErrs, "\n") + "\n"
		files = append(files, gather.File{Path: "errors.txt", Content: []byte(content)})
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename="+bundleRoot+".zip")

	if err := writeBundle(w, bundleRoot, files); err != nil {
		// The response body may be partly written. We can only log the error.
		e.settings.Logger.Error("failed to write support bundle", zap.Error(err))
	}
}

// resourceAttributes converts the telemetry resource to a string map.
func resourceAttributes(res pcommon.Resource) map[string]string {
	attrs := res.Attributes()
	if attrs.Len() == 0 {
		return nil
	}
	out := make(map[string]string, attrs.Len())
	for k, v := range attrs.All() {
		out[k] = v.AsString()
	}
	return out
}

// finishGatherer runs one async gatherer's finish with a panic guard, so a
// panic in one finisher cannot skip the others. Skipping matters because a
// missed pprof finish would leave CPU profiling on and the mutex rate raised.
func (e *supportBundleExtension) finishGatherer(ctx context.Context, s startedGatherer) (files []gather.File, err error) {
	defer func() {
		if r := recover(); r != nil {
			e.settings.Logger.Error("support bundle gatherer panicked on finish",
				zap.String("gatherer", s.name), zap.Any("panic", r))
			err = fmt.Errorf("panicked on finish: %v", r)
		}
	}()
	return s.finish(ctx)
}

// runSyncGatherers runs every snapshot gatherer and collects their files.
func (e *supportBundleExtension) runSyncGatherers(ctx context.Context, opts gather.Options) ([]gather.File, []string) {
	var files []gather.File
	var errs []string
	for _, g := range e.syncGatherers {
		gathered, err := g.Gather(ctx, opts)
		if err != nil {
			e.settings.Logger.Error("support bundle gatherer failed",
				zap.String("gatherer", g.Name()), zap.Error(err))
			errs = append(errs, fmt.Sprintf("%s: %v", g.Name(), err))
		}
		files = append(files, gathered...)
	}
	return files, errs
}

// resolveDuration reads the duration query param and clamps it to the allowed range.
// The value is treated as seconds unless it carries a Go duration unit suffix.
func (e *supportBundleExtension) resolveDuration(r *http.Request) time.Duration {
	duration := e.cfg.DefaultCollectionDuration

	if raw := r.URL.Query().Get("duration"); raw != "" {
		if parsed, ok := parseDuration(raw); ok {
			duration = parsed
		} else {
			e.settings.Logger.Warn("invalid duration query param, using default",
				zap.String("duration", raw))
		}
	}

	if duration < 0 {
		e.settings.Logger.Info("clamped negative collection duration to zero",
			zap.Duration("requested", duration))
		duration = 0
	}
	if duration > e.cfg.MaxCollectionDuration {
		e.settings.Logger.Info("clamped collection duration to the configured maximum",
			zap.Duration("requested", duration), zap.Duration("max", e.cfg.MaxCollectionDuration))
		duration = e.cfg.MaxCollectionDuration
	}
	return duration
}

// parseDuration parses a duration string. A bare number is seconds.
// A value with a unit suffix (for example "500ms") uses Go duration syntax.
func parseDuration(raw string) (time.Duration, bool) {
	if seconds, err := strconv.ParseFloat(raw, 64); err == nil {
		if math.IsNaN(seconds) || math.IsInf(seconds, 0) {
			return 0, false
		}
		return time.Duration(seconds * float64(time.Second)), true
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d, true
	}
	return 0, false
}
