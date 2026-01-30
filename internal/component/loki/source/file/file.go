package file

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
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
	Targets              []discovery.Target   `alloy:"targets,attr"`
	ForwardTo            []loki.LogsReceiver  `alloy:"forward_to,attr"`
	Encoding             string               `alloy:"encoding,attr,optional"`
	DecompressionConfig  DecompressionConfig  `alloy:"decompression,block,optional"`
	FileWatch            FileWatch            `alloy:"file_watch,block,optional"`
	FileMatch            FileMatch            `alloy:"file_match,block,optional"`
	TailFromEnd          bool                 `alloy:"tail_from_end,attr,optional"`
	LegacyPositionsFile  string               `alloy:"legacy_positions_file,attr,optional"`
	OnPositionsFileError OnPositionsFileError `alloy:"on_positions_file_error,attr,optional"`
}

type OnPositionsFileError string

const (
	OnPositionsFileErrorSkip             OnPositionsFileError = "skip"
	OnPositionsFileErrorRestartEnd       OnPositionsFileError = "restart_from_end"
	OnPositionsFileErrorRestartBeginning OnPositionsFileError = "restart_from_beginning"
)

func (o OnPositionsFileError) MarshalText() ([]byte, error) {
	return []byte(string(o)), nil
}

func (o *OnPositionsFileError) UnmarshalText(text []byte) error {
	s := OnPositionsFileError(text)
	switch s {
	case OnPositionsFileErrorSkip, OnPositionsFileErrorRestartEnd, OnPositionsFileErrorRestartBeginning:
		*o = s
	default:
		return fmt.Errorf("unknown OnPositionsFileError value: %s", s)
	}
	return nil
}

func (a *Arguments) SetToDefault() {
	a.FileWatch.SetToDefault()
	a.FileMatch.SetToDefault()
	a.OnPositionsFileError = OnPositionsFileErrorRestartBeginning
}

func (a *Arguments) Validate() error {
	return a.FileMatch.Validate()
}

type FileWatch struct {
	MinPollFrequency time.Duration `alloy:"min_poll_frequency,attr,optional"`
	MaxPollFrequency time.Duration `alloy:"max_poll_frequency,attr,optional"`
}

func (a *FileWatch) SetToDefault() {
	*a = FileWatch{
		MinPollFrequency: 250 * time.Millisecond,
		MaxPollFrequency: 250 * time.Millisecond,
	}
}

type FileMatch struct {
	Enabled         bool          `alloy:"enabled,attr,optional"`
	SyncPeriod      time.Duration `alloy:"sync_period,attr,optional"`
	IgnoreOlderThan time.Duration `alloy:"ignore_older_than,attr,optional"`
}

func (a *FileMatch) SetToDefault() {
	*a = FileMatch{
		Enabled:    false,
		SyncPeriod: 10 * time.Second,
	}
}

func (a *FileMatch) Validate() error {
	if a.SyncPeriod <= 0 {
		return errors.New("sync period must be greater than 0")
	}
	return nil
}

type DecompressionConfig struct {
	Enabled      bool              `alloy:"enabled,attr"`
	InitialDelay time.Duration     `alloy:"initial_delay,attr,optional"`
	Format       CompressionFormat `alloy:"format,attr"`
}

func (d DecompressionConfig) GetFormat() string {
	if d.Enabled {
		return d.Format.String()
	}
	return ""
}

func supportedCompressedFormats() map[string]struct{} {
	return map[string]struct{}{
		"gz":  {},
		"z":   {},
		"bz2": {},
		// TODO: add support for zip.
	}
}

type CompressionFormat string

var (
	_ encoding.TextMarshaler   = CompressionFormat("")
	_ encoding.TextUnmarshaler = (*CompressionFormat)(nil)
)

func (ut CompressionFormat) String() string {
	return string(ut)
}

// MarshalText implements encoding.TextMarshaler.
func (ut CompressionFormat) MarshalText() (text []byte, err error) {
	return []byte(ut.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (ut *CompressionFormat) UnmarshalText(text []byte) error {
	s := string(text)
	_, ok := supportedCompressedFormats()[s]
	if !ok {
		return fmt.Errorf(
			"unsupported compression format: %q - please use one of %q",
			s,
			strings.Join(slices.Collect(maps.Keys(supportedCompressedFormats())), ", "),
		)
	}
	*ut = CompressionFormat(s)
	return nil
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
	scheduler *source.Scheduler[positions.Entry]

	// watcher is a background trigger that periodically invokes
	// scheduling when file matching is enabled.
	watcher *time.Ticker

	handler loki.LogsReceiver
	posFile positions.Positions

	fanout *loki.Fanout

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
		fanout:    loki.NewFanout(args.ForwardTo),
		posFile:   positionsFile,
		scheduler: source.NewScheduler[positions.Entry](),
		watcher:   time.NewTicker(args.FileMatch.SyncPeriod),
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

		// Start black hole drain routine to prevent deadlock when we call c.scheduler.Stop().
		source.Drain(c.handler, func() {
			c.mut.Lock()
			c.stopping.Store(true)
			c.watcher.Stop()
			c.scheduler.Stop()
			close(c.handler.Chan())
			c.mut.Unlock()
		})
	}()

	var wg sync.WaitGroup

	// Start consume and fanout loop
	wg.Go(func() { source.Consume(ctx, c.handler, c.fanout) })

	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.watcher.C:
				c.mut.Lock()
				if !c.args.FileMatch.Enabled {
					c.mut.Unlock()
					continue
				}
				c.scheduleSources()
				c.mut.Unlock()
			}
		}
	})

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

	c.fanout.UpdateChildren(newArgs.ForwardTo)

	// Choose resolver on FileMatch.
	if newArgs.FileMatch.Enabled {
		c.resolver = newGlobResolver(c.opts.Logger)
	} else {
		c.resolver = newStaticResolver()
	}

	if newArgs.FileMatch.SyncPeriod != c.args.FileMatch.SyncPeriod {
		c.watcher.Reset(newArgs.FileMatch.SyncPeriod)
	}

	c.args = newArgs

	c.scheduleSources()
	return nil
}

// scheduleSources resolves desired targets and reconciles the scheduler to
// match the desired state.
// Caller must hold write lock on c.mut before calling this function.
func (c *Component) scheduleSources() {
	source.Reconcile(
		c.opts.Logger,
		c.scheduler,
		c.resolver.Resolve(c.args.Targets),
		func(target resolvedTarget) positions.Entry {
			return positions.Entry{Path: target.Path, Labels: target.Labels.String()}
		},
		func(_ positions.Entry, target resolvedTarget) (source.Source[positions.Entry], error) {
			fi, err := os.Stat(target.Path)
			if err != nil {
				c.metrics.totalBytes.DeleteLabelValues(target.Path)
				return nil, fmt.Errorf("failed to tail file, stat failed: %w", err)
			}

			if fi.IsDir() {
				c.metrics.totalBytes.DeleteLabelValues(target.Path)
				return nil, errors.New("failed to tail file, is directory")
			}

			if c.args.FileMatch.Enabled && c.args.FileMatch.IgnoreOlderThan != 0 && fi.ModTime().Before(time.Now().Add(-c.args.FileMatch.IgnoreOlderThan)) {
				return nil, source.ErrSkip
			}

			c.metrics.totalBytes.WithLabelValues(target.Path).Set(float64(fi.Size()))

			return c.newSource(sourceOptions{
				path:                 target.Path,
				labels:               target.Labels,
				encoding:             c.args.Encoding,
				decompressionConfig:  c.args.DecompressionConfig,
				fileWatch:            c.args.FileWatch,
				tailFromEnd:          c.args.TailFromEnd,
				onPositionsFileError: c.args.OnPositionsFileError,
				legacyPositionUsed:   c.args.LegacyPositionsFile != "",
			})
		},
	)
}

type debugInfo struct {
	TargetsInfo []sourceDebugInfo `alloy:"targets_info,block"`
}

type sourceDebugInfo struct {
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
		ds, ok := s.(source.DebugSource)
		if ok {
			res.TargetsInfo = append(res.TargetsInfo, ds.DebugInfo().(sourceDebugInfo))
		}
	}
	return res
}

type sourceOptions struct {
	path                 string
	labels               model.LabelSet
	encoding             string
	decompressionConfig  DecompressionConfig
	fileWatch            FileWatch
	tailFromEnd          bool
	onPositionsFileError OnPositionsFileError
	legacyPositionUsed   bool
}

// newSource will return a decompressor source if enabled, otherwise a tailer source.
func (c *Component) newSource(opts sourceOptions) (source.Source[positions.Entry], error) {
	tailer := newTailer(
		c.metrics,
		c.opts.Logger,
		c.handler,
		c.posFile,
		c.IsStopping,
		opts,
	)

	// When decompression is enabled we don't retry starting tailer.
	if opts.decompressionConfig.Enabled {
		return tailer, nil
	}

	return source.NewSourceWithRetry(tailer, backoff.Config{
		MinBackoff: 1 * time.Second,
		MaxBackoff: 10 * time.Second,
	}), nil
}

func (c *Component) IsStopping() bool {
	return c.stopping.Load()
}
