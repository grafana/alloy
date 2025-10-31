package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/prometheus/common/model"

	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.file",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

const (
	labelPath        = "__path__"
	labelPathExclude = "__path_exclude__"
	labelFilename    = "filename"
)

// Arguments holds values which are used to configure the loki.source.file
// component.
type Arguments struct {
	Targets             []discovery.Target  `alloy:"targets,attr"`
	ForwardTo           []loki.LogsReceiver `alloy:"forward_to,attr"`
	Encoding            string              `alloy:"encoding,attr,optional"`
	DecompressionConfig DecompressionConfig `alloy:"decompression,block,optional"`
	FileWatch           FileWatch           `alloy:"file_watch,block,optional"`
	FileMatch           FileMatch           `alloy:"file_match,block,optional"`
	TailFromEnd         bool                `alloy:"tail_from_end,attr,optional"`
	LegacyPositionsFile string              `alloy:"legacy_positions_file,attr,optional"`
}

type FileWatch struct {
	MinPollFrequency time.Duration `alloy:"min_poll_frequency,attr,optional"`
	MaxPollFrequency time.Duration `alloy:"max_poll_frequency,attr,optional"`
}

type FileMatch struct {
	Enabled         bool          `alloy:"enabled,attr,optional"`
	SyncPeriod      time.Duration `alloy:"sync_period,attr,optional"`
	IgnoreOlderThan time.Duration `alloy:"ignore_older_than,attr,optional"`
}

var DefaultArguments = Arguments{
	FileWatch: FileWatch{
		MinPollFrequency: 250 * time.Millisecond,
		MaxPollFrequency: 250 * time.Millisecond,
	},
	FileMatch: FileMatch{
		Enabled:    false,
		SyncPeriod: 10 * time.Second,
	},
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

type DecompressionConfig struct {
	Enabled      bool              `alloy:"enabled,attr"`
	InitialDelay time.Duration     `alloy:"initial_delay,attr,optional"`
	Format       CompressionFormat `alloy:"format,attr"`
}

var _ component.Component = (*Component)(nil)

// Component implements the loki.source.file component.
type Component struct {
	opts component.Options

	metrics *metrics

	// mut is used to protect access to args, resolver and scheduler.
	mut sync.RWMutex
	// args stores the latest configuration used by the component.
	// Note: receivers are stored separately with their own lock to avoid
	// unnecessary contention with scheduling operations.
	args Arguments
	// resolver translates discovery targets into concrete file paths. It can
	// be swapped at runtime (e.g., static vs. globbing) when Update() applies
	// new arguments.
	resolver resolver
	// scheduler owns the lifecycle of sources.
	scheduler *Scheduler[positions.Entry]

	// watcher is a background trigger that periodically invokes
	// scheduling when file matching is enabled.
	watcher *watcher

	handler loki.LogsReceiver
	posFile positions.Positions

	receiversMut sync.RWMutex
	receivers    []loki.LogsReceiver

	stopping atomic.Bool
}

// New creates a new loki.source.file component.
func New(o component.Options, args Arguments) (*Component, error) {
	err := os.MkdirAll(o.DataPath, 0750)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	newPositionsPath := filepath.Join(o.DataPath, "positions.yml")
	// Check to see if we can convert the legacy positions file to the new format.
	if args.LegacyPositionsFile != "" {
		positions.ConvertLegacyPositionsFile(args.LegacyPositionsFile, newPositionsPath, o.Logger)
	}
	positionsFile, err := positions.New(o.Logger, positions.Config{
		SyncPeriod:        10 * time.Second,
		PositionsFile:     newPositionsPath,
		IgnoreInvalidYaml: false,
		ReadOnly:          false,
	})
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:      o,
		metrics:   newMetrics(o.Registerer),
		handler:   loki.NewLogsReceiver(),
		receivers: args.ForwardTo,
		posFile:   positionsFile,
		scheduler: NewScheduler[positions.Entry](),
		watcher:   newWatcher(args.FileMatch.SyncPeriod, func() {}),
	}

	// Call to Update() to start sources and set receivers once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		level.Info(c.opts.Logger).Log("msg", "loki.source.file component shutting down, stopping sources and positions file")

		// We need to stop posFile first so we don't record entries we are draining
		c.posFile.Stop()
		// Start black hole drain routine to prevent deadlock when we call c.t.Stop().
		drainCtx, cancelDrain := context.WithCancel(context.Background())
		defer cancelDrain()
		go func() {
			for {
				select {
				case <-drainCtx.Done():
					return
				case <-c.handler.Chan(): // Ignore the remaining entries
				}
			}
		}()
		c.mut.Lock()
		c.stopping.Store(true)
		c.scheduler.Stop()
		close(c.handler.Chan())
		c.mut.Unlock()
	}()

	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case entry := <-c.handler.Chan():
				c.receiversMut.RLock()
				for _, receiver := range c.receivers {
					select {
					case <-ctx.Done():
						c.receiversMut.RUnlock()
						return
					case receiver.Chan() <- entry:
					}
				}
				c.receiversMut.RUnlock()
			}
		}
	})

	wg.Go(func() { c.watcher.Run(ctx) })

	wg.Wait()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	// It's important to have the same lock order in Update and Run to avoid
	// deadlocks.
	c.mut.Lock()
	defer c.mut.Unlock()

	c.receiversMut.RLock()
	if receiversChanged(c.receivers, newArgs.ForwardTo) {
		// Upgrade lock to write.
		c.receiversMut.RUnlock()
		c.receiversMut.Lock()
		c.receivers = newArgs.ForwardTo
		c.receiversMut.Unlock()
	} else {
		c.receiversMut.RUnlock()
	}

	c.args = newArgs

	// Choose resolver and watcher behavior based on FileMatch.
	if newArgs.FileMatch.Enabled {
		c.resolver = newGlobResolver()
		// It is safe to take the lock in the callback function.
		// We never invoke it directly and it's only called by the watcher.
		c.watcher.Update(newArgs.FileMatch.SyncPeriod, func() {
			c.mut.Lock()
			defer c.mut.Unlock()
			c.scheduleSources()
		})
	} else {
		c.resolver = newStaticResolver()
		c.watcher.Update(newArgs.FileMatch.SyncPeriod, func() {})
	}

	c.scheduleSources()
	return nil
}

// scheduleSources resolves desired targets and reconciles the scheduler to
// match the desired state.
// Caller must hold write lock on c.mut before calling this function.
func (c *Component) scheduleSources() {
	// shouldRun tracks the set of unique (path, labels) pairs that should be
	// active after this reconciliation pass, used to identify stale sources.
	shouldRun := make(map[positions.Entry]struct{}, len(c.args.Targets))

	for target, err := range c.resolver.Resolve(c.args.Targets) {
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to resolve target", "error", err)
			continue
		}

		// Deduplicate targets which have the same public label set.
		key := positions.Entry{Path: target.Path, Labels: target.Labels.String()}
		if _, ok := shouldRun[key]; ok {
			continue
		}

		shouldRun[key] = struct{}{}

		// Task is already scheduled
		if c.scheduler.Contains(key) {
			continue
		}

		fi, err := os.Stat(target.Path)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to tail file, stat failed", "error", err, "filename", target.Path)
			c.metrics.totalBytes.DeleteLabelValues(target.Path)
			continue
		}

		if fi.IsDir() {
			level.Info(c.opts.Logger).Log("msg", "failed to tail file", "error", "file is a directory", "filename", target.Path)
			c.metrics.totalBytes.DeleteLabelValues(target.Path)
			continue
		}

		if c.args.FileMatch.Enabled && c.args.FileMatch.IgnoreOlderThan != 0 && fi.ModTime().Before(time.Now().Add(-c.args.FileMatch.IgnoreOlderThan)) {
			continue
		}

		c.metrics.totalBytes.WithLabelValues(target.Path).Set(float64(fi.Size()))

		source, err := c.newSource(sourceOptions{
			path:                target.Path,
			labels:              target.Labels,
			encoding:            c.args.Encoding,
			decompressionConfig: c.args.DecompressionConfig,
			fileWatch:           c.args.FileWatch,
			tailFromEnd:         c.args.TailFromEnd,
			legacyPositionUsed:  c.args.LegacyPositionsFile != "",
		})
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to create file source", "error", err, "filename", target.Path)
			continue
		}

		c.scheduler.ScheduleSource(source)
	}

	var toDelete []Source[positions.Entry]

	// Avoid mutating the scheduler state during iteration. Collect sources to
	// remove and stop them in a separate loop.
	for source := range c.scheduler.Sources() {
		if _, ok := shouldRun[source.Key()]; ok {
			continue
		}
		toDelete = append(toDelete, source)
	}

	for _, s := range toDelete {
		c.scheduler.StopSource(s) // stops without blocking
	}
}

type debugInfo struct {
	TargetsInfo []targetInfo `alloy:"targets_info,block"`
}

type targetInfo struct {
	Path       string `alloy:"path,attr"`
	Labels     string `alloy:"labels,attr"`
	IsRunning  bool   `alloy:"is_running,attr"`
	ReadOffset int64  `alloy:"read_offset,attr"`
}

// DebugInfo returns information about the status of tailed targets.
// TODO(@tpaschalis) Decorate with more debug information once it's made
// available, such as the last time a log line was read.
func (c *Component) DebugInfo() any {
	c.mut.RLock()
	defer c.mut.RUnlock()
	var res debugInfo
	for s := range c.scheduler.Sources() {
		offset, _ := c.posFile.Get(s.Key().Path, s.Key().Labels)
		res.TargetsInfo = append(res.TargetsInfo, targetInfo{
			Path:       s.Key().Path,
			Labels:     s.Key().Labels,
			IsRunning:  s.IsRunning(),
			ReadOffset: offset,
		})
	}
	return res
}

type sourceOptions struct {
	path                string
	labels              model.LabelSet
	encoding            string
	decompressionConfig DecompressionConfig
	fileWatch           FileWatch
	tailFromEnd         bool
	legacyPositionUsed  bool
}

// newSource will return a decompressor source if enabled, otherwise a tailer source.
func (c *Component) newSource(opts sourceOptions) (Source[positions.Entry], error) {
	if opts.decompressionConfig.Enabled {
		decompressor, err := newDecompressor(
			c.metrics,
			c.opts.Logger,
			c.handler,
			c.posFile,
			c.IsStopping,
			opts,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create decompressor %w", err)
		}
		return decompressor, nil
	}
	tailer, err := newTailer(
		c.metrics,
		c.opts.Logger,
		c.handler,
		c.posFile,
		c.IsStopping,
		opts,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tailer %w", err)
	}
	return NewSourceWithRetry(tailer, backoff.Config{
		MinBackoff: 1 * time.Second,
		MaxBackoff: 10 * time.Second,
	}), nil
}

func (c *Component) IsStopping() bool {
	return c.stopping.Load()
}

func receiversChanged(prev, next []loki.LogsReceiver) bool {
	if len(prev) != len(next) {
		return true
	}
	for i := range prev {
		if !reflect.DeepEqual(prev[i], next[i]) {
			return true
		}
	}
	return false
}
