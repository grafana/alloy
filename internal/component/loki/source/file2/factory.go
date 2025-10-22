package file2

import (
	"os"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/tail/watch"
	"github.com/prometheus/common/model"
)

var _ source.TargetsFactory = (*targetsFactory)(nil)

type targetsFactory struct {
	opts    component.Options
	metrics *metrics
}

func (f *targetsFactory) Targets(recv loki.LogsReceiver, pos positions.Positions, isStopping func() bool, args component.Arguments) []source.Target {
	newArgs := args.(Arguments)

	var targets []source.Target

	// There are cases where we have several targets with the same path + public labels
	// So we need to track what we have checked so we don't create multiple targets or tries
	// to create a target for a file that don't exists several times.
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

		target, err := newTarget(recv, f.opts.Logger, f.metrics, pos, isStopping, targetsOptions{
			path:                path,
			labels:              labels,
			encoding:            newArgs.Encoding,
			decompressionConfig: newArgs.DecompressionConfig,
			fileWatch:           newArgs.FileWatch,
			tailFromEnd:         newArgs.TailFromEnd,
			legacyPositionUsed:  newArgs.LegacyPositionsFile != "",
		})

		if err != nil {
			level.Error(f.opts.Logger).Log("msg", "failed to create file target", "error", err, "filename", path)
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

func newTarget(revc loki.LogsReceiver, logger log.Logger, m *metrics, pos positions.Positions, isStopping func() bool, opts targetsOptions) (source.Target, error) {
	if opts.decompressionConfig.Enabled {
		return newDecompressorTarget(
			m,
			logger,
			revc,
			pos,
			opts.path,
			opts.labels,
			opts.encoding,
			opts.decompressionConfig,
			isStopping,
		)
	} else {
		pollOptions := watch.PollingFileWatcherOptions{
			MinPollFrequency: opts.fileWatch.MinPollFrequency,
			MaxPollFrequency: opts.fileWatch.MaxPollFrequency,
		}
		return newTailerTarget(
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
			isStopping,
		)
	}
}
