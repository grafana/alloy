package file

import (
	"iter"
	"os"
	"slices"
	"strings"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tail/fileext"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// duplicateDetector detects when the same file is being tailed multiple times
// with different label sets, which causes duplicate log lines.
type duplicateDetector struct {
	logger log.Logger
	metric prometheus.Gauge
}

// newDuplicateDetector creates a new duplicate detector.
func newDuplicateDetector(logger log.Logger, metric prometheus.Gauge) *duplicateDetector {
	return &duplicateDetector{
		logger: logger,
		metric: metric,
	}
}

// targetInfo holds information about a target for duplicate detection.
type targetInfo struct {
	path   string
	labels model.LabelSet
}

// Detect iterates through all targets, collects them, and checks for files
// that have multiple targets with different label sets. It returns an iterator
// over the collected targets for further processing.
func (d *duplicateDetector) Detect(it iter.Seq[resolvedTarget]) iter.Seq[resolvedTarget] {
	var targets []resolvedTarget
	fileIDToTargets := make(map[string][]targetInfo)

	for target := range it {
		targets = append(targets, target)

		// Stat for duplicate detection.
		fi, err := os.Stat(target.Path)
		if err != nil {
			continue
		}

		fileKey := getFileKey(target.Path, fi)
		fileIDToTargets[fileKey] = append(fileIDToTargets[fileKey], targetInfo{
			path:   target.Path,
			labels: target.Labels,
		})
	}

	d.detectDuplicates(fileIDToTargets)

	return func(yield func(resolvedTarget) bool) {
		for _, t := range targets {
			if !yield(t) {
				return
			}
		}
	}
}

// detectDuplicates processes the grouped targets and reports duplicates.
func (d *duplicateDetector) detectDuplicates(fileIDToTargets map[string][]targetInfo) {
	duplicateCount := 0

	for fileKey, targets := range fileIDToTargets {
		if len(targets) <= 1 {
			continue
		}

		// Collect all label sets to find which labels differ.
		allLabelSets := make([]model.LabelSet, 0, len(targets))
		var paths []string
		for _, t := range targets {
			allLabelSets = append(allLabelSets, t.labels)
			paths = append(paths, t.path)
		}

		// Find labels that have different values across targets.
		differingLabels := findDifferingLabels(allLabelSets)
		if len(differingLabels) == 0 {
			continue // All label sets are identical, not a real duplicate issue
		}

		duplicateCount++

		level.Warn(d.logger).Log(
			"msg", "file has multiple targets with different labels which will cause duplicate log lines",
			"file_id", fileKey,
			"paths", strings.Join(paths, ", "),
			"target_count", len(targets),
			"differing_labels", strings.Join(differingLabels, ", "),
		)
	}

	d.metric.Set(float64(duplicateCount))
}

// getFileKey returns a unique key for a file, using inode+device when available
// (on POSIX systems), or falling back to the file path on Windows.
func getFileKey(path string, fi os.FileInfo) string {
	if fileID, ok := fileext.NewFileID(fi); ok {
		return fileID.String()
	}
	// Fall back to path on systems where file identity isn't available.
	return path
}

// findDifferingLabels returns the names of labels that have different values
// across the provided label sets.
func findDifferingLabels(labelSets []model.LabelSet) []string {
	if len(labelSets) < 2 {
		return nil
	}

	// Collect all label names across all sets.
	allNames := make(map[model.LabelName]struct{})
	for _, ls := range labelSets {
		for name := range ls {
			allNames[name] = struct{}{}
		}
	}

	// Check which labels have differing values.
	var differing []string
	for name := range allNames {
		firstValue, firstHas := labelSets[0][name]
		for _, ls := range labelSets[1:] {
			value, has := ls[name]
			if has != firstHas || value != firstValue {
				differing = append(differing, string(name))
				break
			}
		}
	}

	slices.Sort(differing)
	return differing
}
