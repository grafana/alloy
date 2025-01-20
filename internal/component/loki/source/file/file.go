package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runner"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/tail/watch"
	"github.com/prometheus/common/model"
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
	Targets             []discovery.Target  `alloy:"targets,attr"`
	ForwardTo           []loki.LogsReceiver `alloy:"forward_to,attr"`
	Encoding            string              `alloy:"encoding,attr,optional"`
	DecompressionConfig DecompressionConfig `alloy:"decompression,block,optional"`
	FileWatch           FileWatch           `alloy:"file_watch,block,optional"`
	TailFromEnd         bool                `alloy:"tail_from_end,attr,optional"`
	LegacyPositionsFile string              `alloy:"legacy_positions_file,attr,optional"`
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

	updateMut sync.Mutex

	mut       sync.RWMutex
	args      Arguments
	handler   loki.LogsReceiver
	receivers []loki.LogsReceiver
	posFile   positions.Positions
	tasks     map[positions.Entry]runnerTask

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
		c.mut.RLock()
		runner.Stop()
		c.posFile.Stop()
		close(c.handler.Chan())
		c.mut.RUnlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.handler.Chan():
			c.mut.RLock()
			for _, receiver := range c.receivers {
				receiver.Chan() <- entry
			}
			c.mut.RUnlock()
		case <-c.updateReaders:
			c.mut.Lock()
			var tasks []*runnerTask
			for _, entry := range c.tasks {
				tasks = append(tasks, &entry)
			}
			runner.ApplyTasks(ctx, tasks)
			c.mut.Unlock()
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.updateMut.Lock()
	defer c.updateMut.Unlock()

	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()
	c.args = newArgs
	c.receivers = newArgs.ForwardTo

	c.tasks = make(map[positions.Entry]runnerTask)

	if len(newArgs.Targets) == 0 {
		level.Debug(c.opts.Logger).Log("msg", "no files targets were passed, nothing will be tailed")
	}

	for _, target := range newArgs.Targets {
		path := target[pathLabel]

		labels := make(model.LabelSet)
		for k, v := range target {
			if strings.HasPrefix(k, model.ReservedLabelPrefix) {
				continue
			}
			labels[model.LabelName(k)] = model.LabelValue(v)
		}

		// Deduplicate targets which have the same public label set.
		readersKey := positions.Entry{Path: path, Labels: labels.String()}
		if _, exist := c.tasks[readersKey]; exist {
			continue
		}

		c.reportSize(path)

		reader, err := c.createReader(path, labels)
		if err != nil {
			continue
		}

		c.tasks[readersKey] = runnerTask{
			reader: reader,
			path:   path,
			labels: labels.String(),
			// TODO: Could fastFingerPrint work?
			readerHash: uint64(labels.Merge(model.LabelSet{filenameLabel: model.LabelValue(path)}).Fingerprint()),
		}
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
func (c *Component) DebugInfo() interface{} {
	c.mut.Lock()
	defer c.mut.Unlock()
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

// Returns the elements from set b which are missing from set a
func missing(as map[positions.Entry]reader, bs map[positions.Entry]struct{}) map[positions.Entry]struct{} {
	c := map[positions.Entry]struct{}{}
	for a := range bs {
		if _, ok := as[a]; !ok {
			c[a] = struct{}{}
		}
	}
	return c
}

// For most files, createReader returns a tailer implementation. If the file suffix alludes to it being
// a compressed file, then a decompressor will be created instead.
func (c *Component) createReader(path string, labels model.LabelSet) (reader, error) {
	fi, err := os.Stat(path)
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to tail file, stat failed", "error", err, "filename", path)
		c.metrics.totalBytes.DeleteLabelValues(path)
		return nil, fmt.Errorf("failed to stat path %s", path)
	}

	if fi.IsDir() {
		level.Info(c.opts.Logger).Log("msg", "failed to tail file", "error", "file is a directory", "filename", path)
		c.metrics.totalBytes.DeleteLabelValues(path)
		return nil, fmt.Errorf("failed to tail file, it was a directory %s", path)
	}

	var reader reader
	if c.args.DecompressionConfig.Enabled {
		decompressor, err := newDecompressor(
			c.metrics,
			c.opts.Logger,
			c.handler,
			c.posFile,
			path,
			labels,
			c.args.Encoding,
			c.args.DecompressionConfig,
		)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to create decompressor", "error", err, "filename", path)
			return nil, fmt.Errorf("failed to create decompressor %s", err)
		}
		reader = decompressor
	} else {
		pollOptions := watch.PollingFileWatcherOptions{
			MinPollFrequency: c.args.FileWatch.MinPollFrequency,
			MaxPollFrequency: c.args.FileWatch.MaxPollFrequency,
		}
		tailer, err := newTailer(
			c.metrics,
			c.opts.Logger,
			c.handler,
			c.posFile,
			path,
			labels,
			c.args.Encoding,
			pollOptions,
			c.args.TailFromEnd,
		)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to create tailer", "error", err, "filename", path)
			return nil, fmt.Errorf("failed to create tailer %s", err)
		}
		reader = tailer
	}

	return reader, nil
}

func (c *Component) reportSize(path string) {
	fi, err := os.Stat(path)
	if err != nil {
		return
	}
	c.metrics.totalBytes.WithLabelValues(path).Set(float64(fi.Size()))
}
