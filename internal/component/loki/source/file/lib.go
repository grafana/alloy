package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/runner"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/tail/watch"
)

func build(opts component.Options, cargs component.Arguments) (component.Component, error) {
	args := cargs.(Arguments)

	newPositionsPath := filepath.Join(opts.DataPath, "positions.yml")
	// Check to see if we can convert the legacy positions file to the new format.
	if args.LegacyPositionsFile != "" {
		positions.ConvertLegacyPositionsFile(args.LegacyPositionsFile, newPositionsPath, opts.Logger)
	}
	return source.New(opts, args, &factory{opts: opts, metrics: newMetrics(opts.Registerer)})
}

var _ source.TargetsFactory = (*factory)(nil)

type factory struct {
	opts    component.Options
	metrics *metrics
}

// New implements source.TargetsFactory.
func (f *factory) New(recv loki.LogsReceiver, pos positions.Positions, args component.Arguments) []source.Target {
	newArgs := args.(Arguments)

	var targets []source.Target

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
			level.Error(f.opts.Logger).Log("msg", "failed to tail file, stat failed", "error", err, "filename", path)
			f.metrics.totalBytes.DeleteLabelValues(path)
			continue
		}

		if fi.IsDir() {
			level.Info(f.opts.Logger).Log("msg", "failed to tail file", "error", "file is a directory", "filename", path)
			f.metrics.totalBytes.DeleteLabelValues(path)
			continue
		}

		f.metrics.totalBytes.WithLabelValues(path).Set(float64(fi.Size()))

		target, err := newTarget(recv, f.opts.Logger, f.metrics, pos, labels, targetsOptions{
			path:                path,
			labels:              labels,
			encoding:            newArgs.Encoding,
			decompressionConfig: newArgs.DecompressionConfig,
			fileWatch:           newArgs.FileWatch,
			tailFromEnd:         newArgs.TailFromEnd,
			legacyPositionUsed:  newArgs.LegacyPositionsFile != "",
		})

		if err != nil {
			// FIXME: Log
			continue
		}

		targets = append(targets, target)
	}

	if len(targets) == 0 {
		level.Debug(f.opts.Logger).Log("msg", "no files targets were passed, nothing will be tailed")
	}

	return targets
}

type targetsOptions struct {
	path                string
	labels              model.LabelSet
	encoding            string
	decompressionConfig DecompressionConfig
	fileWatch           FileWatch
	tailFromEnd         bool
	legacyPositionUsed  bool
}

func newTarget(revc loki.LogsReceiver, logger log.Logger, m *metrics, pos positions.Positions, labels model.LabelSet, opts targetsOptions) (source.Target, error) {
	hash := uint64(labels.Merge(model.LabelSet{filenameLabel: model.LabelValue(opts.path)}).Fingerprint())
	// FIXME clean this up.
	if opts.decompressionConfig.Enabled {
		decompressor, err := newDecompressor(
			m,
			logger,
			revc,
			pos,
			opts.path,
			opts.labels,
			opts.encoding,
			opts.decompressionConfig,
			func() bool { return false },
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create decompressor %s", err)
		}
		return &target{reader: decompressor, hash: hash}, nil
	} else {
		pollOptions := watch.PollingFileWatcherOptions{
			MinPollFrequency: opts.fileWatch.MinPollFrequency,
			MaxPollFrequency: opts.fileWatch.MaxPollFrequency,
		}
		tailer, err := newTailer(
			m,
			logger,
			revc,
			pos,
			opts.path,
			opts.labels,
			opts.encoding,
			pollOptions,
			opts.tailFromEnd,
			opts.legacyPositionUsed,
			func() bool { return false },
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create tailer %s", err)
		}
		return &target{reader: tailer, hash: hash}, nil
	}
}

// FIXME: This is just glue for now, what I really should do is to implement Target for tailer and decompessor
type target struct {
	hash   uint64
	reader reader
}

// Equals implements source.Target.
func (t *target) Equals(other runner.Task) bool {
	otherTask := other.(*target)

	if t == otherTask {
		return true
	}

	return t.hash == otherTask.hash
}

// Hash implements source.Target.
func (t *target) Hash() uint64 {
	return t.hash
}

// Run implements source.Target.
func (t *target) Run(ctx context.Context) {
	backoff := backoff.New(
		ctx,
		backoff.Config{
			MinBackoff: 1 * time.Second,
			MaxBackoff: 10 * time.Second,
			MaxRetries: 0,
		},
	)

	for {
		t.reader.Run(ctx)
		backoff.Wait()
		if !backoff.Ongoing() {
			break
		}
	}
}
