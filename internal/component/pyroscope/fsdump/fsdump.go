// Package fsdump provides a component for writing Pyroscope profiles to the filesystem.
package fsdump

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
)

func init() {
	component.Register(component.Registration{
		Name: "pyroscope.fsdump",
		//Stability: featuregate.StabilityPublicPreview,
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments represents the input state of the pyroscope.fsdump component.
type Arguments struct {
	// Target directory where profile dumps will be stored
	TargetDirectory string `alloy:"target_directory,attr"`

	// Maximum total size of files in the target directory (in bytes)
	MaxSizeBytes int64 `alloy:"max_size_bytes,attr,optional"`

	// External labels to add to all profiles
	ExternalLabels map[string]string `alloy:"external_labels,attr,optional"`

	// Relabeling rules to apply before dumping profiles
	RelabelConfigs []*alloy_relabel.Config `alloy:"rule,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		MaxSizeBytes: 1024 * 1024 * 1024, // 1GB default
	}
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.TargetDirectory == "" {
		return errors.New("target_directory is required")
	}
	return nil
}

// Exports represents the set of fields exported by the component.
type Exports struct {
	Receiver pyroscope.Appendable `alloy:"receiver,attr"`
}

// Component implements the pyroscope.fsdump component.
type Component struct {
	opts    component.Options
	cfg     Arguments
	metrics *metrics

	// Directory stats
	targetDirSize  int64
	targetDirMutex sync.Mutex

	// Relabeling
	relabelConfigs []*relabel.Config

	// Component state
	exporter pyroscope.Appendable
	exited   bool
	mut      sync.RWMutex
}

// New creates a new pyroscope.fsdump component.
func New(opts component.Options, args Arguments) (*Component, error) {
	// Initialize target directory if it doesn't exist
	err := os.MkdirAll(args.TargetDirectory, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err)
	}

	comp := &Component{
		opts:           opts,
		cfg:            args,
		metrics:        newMetrics(opts.Registerer),
		relabelConfigs: alloy_relabel.ComponentToPromRelabelConfigs(args.RelabelConfigs),
	}

	// Calculate initial directory size
	size, err := calculateDirSize(args.TargetDirectory)
	if err != nil {
		level.Warn(opts.Logger).Log("msg", "Failed to calculate initial directory size", "err", err)
	} else {
		comp.targetDirSize = size
		comp.metrics.currentSizeBytes.Set(float64(size))
	}

	// Create the exporter
	comp.exporter = &fsAppendable{
		component: comp,
	}

	opts.OnStateChange(Exports{
		Receiver: comp.exporter,
	})

	return comp, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	level.Info(c.opts.Logger).Log("msg", "Starting pyroscope.fsdump", "target_directory", c.cfg.TargetDirectory)

	// Run cleanup routine
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.mut.Lock()
			c.exited = true
			c.mut.Unlock()
			return nil

		case <-ticker.C:
			// Periodically clean up old profiles if we're over the size limit
			c.cleanupOldProfiles()
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()

	// Update configuration
	prevDir := c.cfg.TargetDirectory
	c.cfg = newArgs
	c.relabelConfigs = alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelConfigs)

	// If directory changed, we need to create it and recalculate size
	if prevDir != newArgs.TargetDirectory {
		err := os.MkdirAll(newArgs.TargetDirectory, 0755)
		if err != nil {
			return fmt.Errorf("failed to create target directory: %w", err)
		}

		size, err := calculateDirSize(newArgs.TargetDirectory)
		if err != nil {
			level.Warn(c.opts.Logger).Log("msg", "Failed to calculate directory size", "err", err)
		} else {
			c.targetDirSize = size
			c.metrics.currentSizeBytes.Set(float64(size))
		}
	}

	// Update exported state
	c.opts.OnStateChange(Exports{
		Receiver: c.exporter,
	})

	return nil
}

// calculateDirSize returns the total size of all files in the directory
func calculateDirSize(dir string) (int64, error) {
	var size int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// cleanupOldProfiles removes the oldest profiles if the directory exceeds the size limit
func (c *Component) cleanupOldProfiles() {
	c.targetDirMutex.Lock()
	defer c.targetDirMutex.Unlock()

	if c.targetDirSize <= c.cfg.MaxSizeBytes {
		return // Under the limit, no cleanup needed
	}

	// Get list of files with modification times
	type fileInfo struct {
		path    string
		modTime time.Time
		size    int64
	}

	var files []fileInfo
	err := filepath.WalkDir(c.cfg.TargetDirectory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			files = append(files, fileInfo{
				path:    path,
				modTime: info.ModTime(),
				size:    info.Size(),
			})
		}
		return nil
	})

	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "Failed to walk directory for cleanup", "err", err)
		return
	}

	// Sort by modification time (oldest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	// Remove oldest files until we're under the limit
	sizeToRemove := c.targetDirSize - c.cfg.MaxSizeBytes
	var removedSize int64
	var removedCount int

	for _, file := range files {
		if removedSize >= sizeToRemove {
			break
		}

		err := os.Remove(file.path)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "Failed to remove file during cleanup", "path", file.path, "err", err)
			continue
		}

		removedSize += file.size
		removedCount++
	}

	if removedCount > 0 {
		c.targetDirSize -= removedSize
		c.metrics.currentSizeBytes.Set(float64(c.targetDirSize))
		c.metrics.filesRemoved.Add(float64(removedCount))
		level.Info(c.opts.Logger).Log("msg", "Cleaned up old profile files", "removed_count", removedCount, "removed_bytes", removedSize)
	}
}

// updateDirSize adds the given size to the directory size counter
func (c *Component) updateDirSize(size int64) {
	c.targetDirMutex.Lock()
	defer c.targetDirMutex.Unlock()

	c.targetDirSize += size
	c.metrics.currentSizeBytes.Set(float64(c.targetDirSize))
}

// fsAppendable implements pyroscope.Appendable interface to provide an
// appendable that writes profiles to the filesystem
type fsAppendable struct {
	component *Component
}

// Appender returns an Appender implementation that writes profiles to files
func (a *fsAppendable) Appender() pyroscope.Appender {
	return a
}

// Append implements the pyroscope.Appender interface to handle profile data
func (a *fsAppendable) Append(ctx context.Context, lbls labels.Labels, samples []*pyroscope.RawSample) error {
	c := a.component

	c.mut.RLock()
	defer c.mut.RUnlock()

	if c.exited {
		return nil
	}

	c.metrics.profilesReceived.Inc()

	if len(samples) == 0 {
		return nil
	}

	// Apply relabeling rules if configured
	finalLabels := lbls
	if len(c.relabelConfigs) > 0 {
		builder := labels.NewBuilder(lbls)
		keep := relabel.ProcessBuilder(builder, c.relabelConfigs...)
		if !keep {
			c.metrics.profilesDropped.Inc()
			level.Debug(c.opts.Logger).Log("msg", "Profile dropped by relabel rules", "labels", lbls.String())
			return nil
		}
		finalLabels = builder.Labels()
	}

	// Apply external labels
	if len(c.cfg.ExternalLabels) > 0 {
		builder := labels.NewBuilder(finalLabels)
		for name, value := range c.cfg.ExternalLabels {
			builder.Set(name, value)
		}
		finalLabels = builder.Labels()
	}

	// Generate a unique base filename
	baseFilename := uuid.New().String()
	profilePath := filepath.Join(c.cfg.TargetDirectory, baseFilename+".profile")
	labelsPath := filepath.Join(c.cfg.TargetDirectory, baseFilename+".labels")

	var totalSize int64

	// Write each sample to its own set of files
	for i, sample := range samples {
		// For multiple samples, append index to filename
		sampleProfilePath := profilePath
		sampleLabelsPath := labelsPath
		if len(samples) > 1 {
			sampleProfilePath = filepath.Join(c.cfg.TargetDirectory, fmt.Sprintf("%s_%d.profile", baseFilename, i))
			sampleLabelsPath = filepath.Join(c.cfg.TargetDirectory, fmt.Sprintf("%s_%d.labels", baseFilename, i))
		}

		// Write profile data to profile file
		profileFile, err := os.Create(sampleProfilePath)
		if err != nil {
			c.metrics.writeErrors.Inc()
			return fmt.Errorf("failed to create profile file: %w", err)
		}

		n, err := profileFile.Write(sample.RawProfile)
		if err != nil {
			profileFile.Close()
			c.metrics.writeErrors.Inc()
			return fmt.Errorf("failed to write profile data: %w", err)
		}
		totalSize += int64(n)
		profileFile.Close()

		// Write labels to labels file
		labelsFile, err := os.Create(sampleLabelsPath)
		if err != nil {
			c.metrics.writeErrors.Inc()
			return fmt.Errorf("failed to create labels file: %w", err)
		}

		labelStr := finalLabels.String()
		nLabels, err := labelsFile.WriteString(labelStr)
		if err != nil {
			labelsFile.Close()
			c.metrics.writeErrors.Inc()
			return fmt.Errorf("failed to write label data: %w", err)
		}
		totalSize += int64(nLabels)
		labelsFile.Close()
	}

	// Update directory size
	c.updateDirSize(totalSize)

	// Update metrics
	c.metrics.profilesWritten.Inc()
	c.metrics.bytesWritten.Add(float64(totalSize))

	level.Debug(c.opts.Logger).Log("msg", "Profile and labels written to separate files", "base_path", filepath.Join(c.cfg.TargetDirectory, baseFilename), "size", totalSize, "labels", finalLabels.String())

	return nil
}

// AppendIngest implements the pyroscope.Appender interface for handling HTTP ingest-style profiles
func (a *fsAppendable) AppendIngest(ctx context.Context, profile *pyroscope.IncomingProfile) error {
	c := a.component

	c.mut.RLock()
	defer c.mut.RUnlock()

	if c.exited {
		return nil
	}

	c.metrics.profilesReceived.Inc()

	// Apply relabeling rules if configured
	finalLabels := profile.Labels
	if len(c.relabelConfigs) > 0 {
		builder := labels.NewBuilder(profile.Labels)
		keep := relabel.ProcessBuilder(builder, c.relabelConfigs...)
		if !keep {
			c.metrics.profilesDropped.Inc()
			level.Debug(c.opts.Logger).Log("msg", "Profile dropped by relabel rules", "labels", profile.Labels.String())
			return nil
		}
		finalLabels = builder.Labels()
	}

	// Apply external labels
	if len(c.cfg.ExternalLabels) > 0 {
		builder := labels.NewBuilder(finalLabels)
		for name, value := range c.cfg.ExternalLabels {
			builder.Set(name, value)
		}
		finalLabels = builder.Labels()
	}

	// Generate a unique base filename
	baseFilename := uuid.New().String()
	profilePath := filepath.Join(c.cfg.TargetDirectory, baseFilename+".profile")
	labelsPath := filepath.Join(c.cfg.TargetDirectory, baseFilename+".labels")

	// Write profile data to profile file
	profileFile, err := os.Create(profilePath)
	if err != nil {
		c.metrics.writeErrors.Inc()
		return fmt.Errorf("failed to create profile file: %w", err)
	}
	defer profileFile.Close()

	n, err := profileFile.Write(profile.RawBody)
	if err != nil {
		c.metrics.writeErrors.Inc()
		return fmt.Errorf("failed to write profile data: %w", err)
	}

	// Write labels to labels file
	labelsFile, err := os.Create(labelsPath)
	if err != nil {
		c.metrics.writeErrors.Inc()
		return fmt.Errorf("failed to create labels file: %w", err)
	}
	defer labelsFile.Close()

	labelStr := finalLabels.String()
	nLabels, err := labelsFile.WriteString(labelStr)
	if err != nil {
		c.metrics.writeErrors.Inc()
		return fmt.Errorf("failed to write label data: %w", err)
	}

	totalSize := int64(n + nLabels)

	// Update directory size
	c.updateDirSize(totalSize)

	// Update metrics
	c.metrics.profilesWritten.Inc()
	c.metrics.bytesWritten.Add(float64(totalSize))

	level.Debug(c.opts.Logger).Log("msg", "Profile and labels written to separate files", "profile_path", profilePath, "labels_path", labelsPath, "size", totalSize, "labels", finalLabels.String())

	return nil
}
