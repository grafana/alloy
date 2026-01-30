package file

import (
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
//
// Usage:
//  1. Create a new instance for each reconciliation cycle
//  2. Call Track() for each file that will actually be tailed
//  3. Call Report() after reconciliation to detect and log duplicates
type duplicateDetector struct {
	logger log.Logger
	metric prometheus.Gauge

	// fileKeyToTargets accumulates target info during reconciliation.
	// It maps file key (inode+device or path) to the list of targets for that file.
	fileKeyToTargets map[fileKey][]targetInfo
}

// fileKey is a unique identifier for a file, using inode+device on POSIX
// systems or the file path on Windows.
type fileKey string

// targetInfo holds information about a target for duplicate detection.
type targetInfo struct {
	path   string
	labels model.LabelSet
}

// newDuplicateDetector creates a new duplicate detector.
func newDuplicateDetector(logger log.Logger, metric prometheus.Gauge) *duplicateDetector {
	return &duplicateDetector{
		logger:           logger,
		metric:           metric,
		fileKeyToTargets: make(map[fileKey][]targetInfo),
	}
}

// Track records a file that will be tailed for duplicate detection.
// This should be called during reconciliation for each file that passes
// validation and will actually be tailed.
func (d *duplicateDetector) Track(path string, labels model.LabelSet, fi os.FileInfo) {
	key := getFileKey(path, fi)
	d.fileKeyToTargets[key] = append(d.fileKeyToTargets[key], targetInfo{
		path:   path,
		labels: labels,
	})
}

// Report processes the accumulated state and reports any duplicates found.
// This should be called after reconciliation is complete.
func (d *duplicateDetector) Report() {
	d.detectDuplicates(d.fileKeyToTargets)
}

// detectDuplicates processes the grouped targets and reports duplicates.
func (d *duplicateDetector) detectDuplicates(fileKeyToTargets map[fileKey][]targetInfo) {
	duplicateCount := 0

	for key, targets := range fileKeyToTargets {
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
			"file_id", key,
			"paths", strings.Join(paths, ", "),
			"target_count", len(targets),
			"differing_labels", strings.Join(differingLabels, ", "),
		)
	}

	d.metric.Set(float64(duplicateCount))
}

// getFileKey returns a unique key for a file, using inode+device when available
// (on POSIX systems), or falling back to the file path on Windows.
func getFileKey(path string, fi os.FileInfo) fileKey {
	if fileID, ok := fileext.NewFileID(fi); ok {
		return fileKey(fileID.String())
	}
	// Fall back to path on systems where file identity isn't available.
	return fileKey(path)
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
	// We compare all label sets against the first one rather than doing pairwise
	// comparisons. This is sufficient because if all values are identical, they'll
	// all match the first; if any value differs, at least one will differ from the
	// first. This gives us O(n) comparisons instead of O(n²).
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
