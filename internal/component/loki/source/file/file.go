package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/grafana/tail/watch"
	"github.com/prometheus/common/model"

	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runner"
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
	pathLabel     = "__path__"
	filenameLabel = "filename"
)

// Arguments holds values which are used to configure the loki.source.file
// component.
type Arguments struct {
	Targets              []discovery.Target   `alloy:"targets,attr"`
	ForwardTo            []loki.LogsReceiver  `alloy:"forward_to,attr"`
	Encoding             string               `alloy:"encoding,attr,optional"`
	DecompressionConfig  DecompressionConfig  `alloy:"decompression,block,optional"`
	FileWatch            FileWatch            `alloy:"file_watch,block,optional"`
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

type FileWatch struct {
	MinPollFrequency time.Duration `alloy:"min_poll_frequency,attr,optional"`
	MaxPollFrequency time.Duration `alloy:"max_poll_frequency,attr,optional"`
}

var DefaultArguments = Arguments{
	FileWatch: FileWatch{
		MinPollFrequency: 250 * time.Millisecond,
		MaxPollFrequency: 250 * time.Millisecond,
	},
	OnPositionsFileError: OnPositionsFileErrorRestartBeginning,
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
	opts    component.Options
	metrics *metrics

	tasksMut sync.RWMutex
	tasks    map[positions.Entry]runnerTask

	handler loki.LogsReceiver
	posFile positions.Positions

	receiversMut sync.RWMutex
	receivers    []loki.LogsReceiver

	stopping atomic.Bool

	updateReaders chan struct{}
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
		opts:    o,
		metrics: newMetrics(o.Registerer),

		handler:       loki.NewLogsReceiver(),
		receivers:     args.ForwardTo,
		posFile:       positionsFile,
		tasks:         make(map[positions.Entry]runnerTask),
		updateReaders: make(chan struct{}, 1),
	}

	// Call to Update() to start readers and set receivers once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	runner := runner.New(func(t *runnerTask) runner.Worker {
		return &runnerReader{
			reader: t.reader,
		}
	})

	defer func() {
		level.Info(c.opts.Logger).Log("msg", "loki.source.file component shutting down, stopping readers and positions file")

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

		c.tasksMut.RLock()
		c.stopping.Store(true)
		runner.Stop()
		c.posFile.Stop()
		close(c.handler.Chan())
		c.tasksMut.RUnlock()
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case entry := <-c.handler.Chan():
				c.receiversMut.RLock()
				for _, receiver := range c.receivers {
					select {
					case <-ctx.Done():
						return
					case receiver.Chan() <- entry:
					}
				}
				c.receiversMut.RUnlock()
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.updateReaders:
				level.Debug(c.opts.Logger).Log("msg", "updating tasks", "tasks", len(c.tasks))

				c.tasksMut.RLock()
				var tasks []*runnerTask
				for _, entry := range c.tasks {
					tasks = append(tasks, &entry)
				}
				c.tasksMut.RUnlock()

				if err := runner.ApplyTasks(ctx, tasks); err != nil {
					if !errors.Is(err, context.Canceled) {
						level.Error(c.opts.Logger).Log("msg", "failed to apply tasks", "err", err)
					}
				} else {
					level.Debug(c.opts.Logger).Log("msg", "workers successfully updated", "workers", len(runner.Workers()))
				}
			}
		}
	}()

	wg.Wait()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	// It's important to have the same lock order in Update and Run to avoid
	// deadlocks.
	c.tasksMut.Lock()
	defer c.tasksMut.Unlock()

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

	c.tasks = make(map[positions.Entry]runnerTask)

	// There are cases where we have several targets with the same path + public labels
	// but the path no longer exist so we cannot create a task for it. So we need to track
	// what we have checked separately from the task map to prevent performing checks that
	// will fail multiple times.
	checked := make(map[positions.Entry]struct{})

	for _, target := range newArgs.Targets {
		path, _ := target.Get(pathLabel)

		labels := target.NonReservedLabelSet()

		// Deduplicate targets which have the same public label set.
		key := positions.Entry{Path: path, Labels: labels.String()}
		if _, exists := checked[key]; exists {
			continue
		}

		checked[key] = struct{}{}

		fi, err := os.Stat(path)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to tail file, stat failed", "error", err, "filename", path)
			c.metrics.totalBytes.DeleteLabelValues(path)
			continue
		}

		if fi.IsDir() {
			level.Info(c.opts.Logger).Log("msg", "failed to tail file", "error", "file is a directory", "filename", path)
			c.metrics.totalBytes.DeleteLabelValues(path)
			continue
		}

		c.metrics.totalBytes.WithLabelValues(path).Set(float64(fi.Size()))

		reader, err := c.createReader(readerOptions{
			path:                 path,
			labels:               labels,
			encoding:             newArgs.Encoding,
			decompressionConfig:  newArgs.DecompressionConfig,
			fileWatch:            newArgs.FileWatch,
			tailFromEnd:          newArgs.TailFromEnd,
			onPositionsFileError: newArgs.OnPositionsFileError,
			legacyPositionUsed:   newArgs.LegacyPositionsFile != "",
		})
		if err != nil {
			continue
		}

		c.tasks[key] = runnerTask{
			reader: reader,
			path:   path,
			labels: labels.String(),
			// TODO: Could fastFingerPrint work?
			readerHash: uint64(labels.Merge(model.LabelSet{filenameLabel: model.LabelValue(path)}).Fingerprint()),
		}
	}

	if len(newArgs.Targets) == 0 {
		level.Debug(c.opts.Logger).Log("msg", "no files targets were passed, nothing will be tailed")
	}

	select {
	case c.updateReaders <- struct{}{}:
	default:
	}

	return nil
}

// DebugInfo returns information about the status of tailed targets.
// TODO(@tpaschalis) Decorate with more debug information once it's made
// available, such as the last time a log line was read.
func (c *Component) DebugInfo() any {
	c.tasksMut.RLock()
	defer c.tasksMut.RUnlock()
	var res readerDebugInfo
	for e, task := range c.tasks {
		offset, _ := c.posFile.Get(e.Path, e.Labels)
		res.TargetsInfo = append(res.TargetsInfo, targetInfo{
			Path:       e.Path,
			Labels:     e.Labels,
			IsRunning:  task.reader.IsRunning(),
			ReadOffset: offset,
		})
	}
	return res
}

type readerDebugInfo struct {
	TargetsInfo []targetInfo `alloy:"targets_info,block"`
}

type targetInfo struct {
	Path       string `alloy:"path,attr"`
	Labels     string `alloy:"labels,attr"`
	IsRunning  bool   `alloy:"is_running,attr"`
	ReadOffset int64  `alloy:"read_offset,attr"`
}

type readerOptions struct {
	path                 string
	labels               model.LabelSet
	encoding             string
	decompressionConfig  DecompressionConfig
	fileWatch            FileWatch
	tailFromEnd          bool
	onPositionsFileError OnPositionsFileError
	legacyPositionUsed   bool
}

// For most files, createReader returns a tailer implementation. If the file suffix alludes to it being
// a compressed file, then a decompressor will be created instead.
func (c *Component) createReader(opts readerOptions) (reader, error) {
	var reader reader
	if opts.decompressionConfig.Enabled {
		decompressor, err := newDecompressor(
			c.metrics,
			c.opts.Logger,
			c.handler,
			c.posFile,
			opts.path,
			opts.labels,
			opts.encoding,
			opts.decompressionConfig,
			opts.onPositionsFileError,
			c.IsStopping,
		)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to create decompressor", "error", err, "filename", opts.path)
			return nil, fmt.Errorf("failed to create decompressor %s", err)
		}
		reader = decompressor
	} else {
		pollOptions := watch.PollingFileWatcherOptions{
			MinPollFrequency: opts.fileWatch.MinPollFrequency,
			MaxPollFrequency: opts.fileWatch.MaxPollFrequency,
		}
		tailer, err := newTailer(
			c.metrics,
			c.opts.Logger,
			c.handler,
			c.posFile,
			opts.path,
			opts.labels,
			opts.encoding,
			pollOptions,
			opts.tailFromEnd,
			opts.legacyPositionUsed,
			opts.onPositionsFileError,
			c.IsStopping,
		)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to create tailer", "error", err, "filename", opts.path)
			return nil, fmt.Errorf("failed to create tailer %s", err)
		}
		reader = tailer
	}

	return reader, nil
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
