// Package file provides an otelcol.storage.file component.
package file

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"time"

	"github.com/grafana/alloy/internal/component"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.storage.file",
		Stability: featuregate.StabilityPublicPreview,
		Args:      Arguments{},
		Exports:   extension.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := filestorage.NewFactory()
			xargs := args.(Arguments)
			return extension.New(opts, fact, xargs)
		},
	})
}

// Arguments configures the otelcol.file.storage component.
type Arguments struct {
	// Directory specifies the directory where the file storage will store its data.
	Directory string `alloy:"directory,attr,optional"`

	// Timeout specifies the timeout for file storage operations.
	Timeout time.Duration `alloy:"timeout,attr,optional"`

	// Compaction configures file storage compaction.
	Compaction *CompactionConfig `alloy:"compaction,block,optional"`

	// FSync specifies that fsync should be called after each database write.
	FSync bool `alloy:"fsync,attr,optional"`

	// CreateDirectory specifies that the directory should be created automatically by the extension on start.
	CreateDirectory bool `alloy:"create_directory,attr,optional"`

	// DirectoryPermissions specifies the permissions for the directory if it must be created.
	DirectoryPermissions string `alloy:"directory_permissions,attr,optional"`

	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// CompationConfig configures optional file storage compaction.
type CompactionConfig struct {
	// OnStart specifies that compaction is attempted each time on start.
	OnStart bool `alloy:"on_start,attr,optional"`

	// OnRebound specifies that compaction is attempted online, when rebound conditions are met.
	OnRebound bool `alloy:"on_rebound,attr,optional"`

	// Directory specifies where the temporary files for compaction will be stored.
	Directory string `alloy:"directory,attr,optional"`

	// ReboundNeededThresholdMiB specifies the minimum total allocated size (both used and empty)
	// to mark the need for online compaction.
	ReboundNeededThresholdMiB int64 `alloy:"rebound_needed_threshold_mib,attr,optional"`

	// ReboundTriggerThresholdMiB is used when compaction is marked as needed. When used allocated data size drops
	// below the specified value, the compaction starts and the flag marking need for compaction is cleared.
	ReboundTriggerThresholdMiB int64 `alloy:"rebound_trigger_threshold_mib,attr,optional"`

	// MaxTransactionSize specifies the maximum number of items that might be present in a single compaction iteration.
	MaxTransactionSize int64 `alloy:"max_transaction_size,attr,optional"`

	// CheckInterval specifies the frequency of compaction checks.
	CheckInterval time.Duration `alloy:"check_interval,attr,optional"`

	// CleanupOnStart specifies that removal of temporary files is performed on start.
	CleanupOnStart bool `alloy:"cleanup_on_start,attr,optional"`
}

// Convert converts the CompactionConfig to the underlying config type.
func (c *CompactionConfig) Convert() *filestorage.CompactionConfig {
	if c == nil {
		return nil
	}

	return &filestorage.CompactionConfig{
		OnStart:                    c.OnStart,
		OnRebound:                  c.OnRebound,
		Directory:                  c.Directory,
		ReboundNeededThresholdMiB:  c.ReboundNeededThresholdMiB,
		ReboundTriggerThresholdMiB: c.ReboundTriggerThresholdMiB,
		MaxTransactionSize:         c.MaxTransactionSize,
		CheckInterval:              c.CheckInterval,
		CleanupOnStart:             c.CleanupOnStart,
	}
}

var _ extension.Arguments = Arguments{}
var _ syntax.Validator = (*Arguments)(nil)
var _ syntax.Defaulter = (*Arguments)(nil)

// defaults copied from OpenTelemetry Collector
var (
	defaultMaxTransactionSize         int64 = 65536
	defaultReboundTriggerThresholdMib int64 = 10
	defaultReboundNeededThresholdMib  int64 = 100
	defaultCompactionInterval               = time.Second * 5

	defaultCompaction = CompactionConfig{
		// Directory:                  getDefaultDirectory(),
		OnStart:                    false,
		OnRebound:                  false,
		MaxTransactionSize:         defaultMaxTransactionSize,
		ReboundNeededThresholdMiB:  defaultReboundNeededThresholdMib,
		ReboundTriggerThresholdMiB: defaultReboundTriggerThresholdMib,
		CheckInterval:              defaultCompactionInterval,
		CleanupOnStart:             false,
	}

	DefaultConfig = Arguments{
		// Directory: getDefaultDirectory(), // Default directory must have the context of Alloy's storage.path
		Timeout: time.Second,
		// Compaction:           &defaultCompaction, // Default compaction must not be the same pointer across SetToDefault calls
		FSync:                false,
		CreateDirectory:      true,
		DirectoryPermissions: "0750",
	}
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultConfig
	// Copy the default compaction config to avoid reusing the same pointer.
	compaction := defaultCompaction
	args.Compaction = &compaction
	args.DebugMetrics.SetToDefault()
}

func (args *Arguments) Validate() error {
	var errs error
	// Unlike the upstream, we don't set the default directory here as its dependent on Alloy's storage.path.
	// Without that context we cannot set the default directory.
	if args.Directory != "" {
		if err := validateDirectory(args.Directory, args.CreateDirectory); err != nil {
			errs = errors.Join(errs, fmt.Errorf("directory: %w", err))
		}
	}
	if args.Compaction.Directory != "" {
		if err := validateDirectory(args.Compaction.Directory, args.CreateDirectory); err != nil {
			errs = errors.Join(errs, fmt.Errorf("compaction directory: %w", err))
		}
	}

	if args.Compaction.MaxTransactionSize < 0 {
		errs = errors.Join(errs, errors.New("max transaction size for compaction cannot be less than 0"))
	}

	if args.Compaction.OnRebound && args.Compaction.CheckInterval <= 0 {
		errs = errors.Join(errs, errors.New("compaction check interval must be positive when rebound compaction is set"))
	}

	if args.CreateDirectory {
		permissions, err := strconv.ParseInt(args.DirectoryPermissions, 8, 32)
		if err != nil {
			errs = errors.Join(errs, errors.New("directory_permissions value must be a valid octal representation"))
		} else if permissions&int64(os.ModePerm) != permissions {
			errs = errors.Join(errs, errors.New("directory_permissions contain invalid bits for file access"))
		}
	}
	return errs
}

func validateDirectory(dir string, createDirectory bool) error {
	if info, err := os.Stat(dir); err != nil {
		if !createDirectory && os.IsNotExist(err) {
			return fmt.Errorf("directory must exist: %w. You can enable the create_directory option to automatically create it", err)
		}

		fsErr := &fs.PathError{}
		if errors.As(err, &fsErr) && !os.IsNotExist(err) {
			return fmt.Errorf("problem accessing configured directory: %s, err: %w", dir, fsErr)
		}
	} else if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	return nil
}

// Convert implements extension.Arguments.
func (args Arguments) Convert(opts component.Options) (otelcomponent.Config, error) {
	// Convert the Arguments to the underlying config type.
	f := &filestorage.Config{
		Directory:            args.Directory,
		Timeout:              args.Timeout,
		Compaction:           args.Compaction.Convert(),
		FSync:                args.FSync,
		CreateDirectory:      args.CreateDirectory,
		DirectoryPermissions: args.DirectoryPermissions,
	}

	// TODO - find a way to sync these default values with the node.
	// Without them being settable in syntax.Defaulter they are not exposed at that layer.
	if f.Directory == "" {
		// If the directory is not set, use the default directory.
		f.Directory = opts.DataPath
	}

	if f.Compaction.Directory == "" {
		f.Compaction.Directory = f.Directory
	}

	// Validate sets the internal directorypermissions mask
	if err := f.Validate(); err != nil {
		return nil, fmt.Errorf("invalid file storage config: %w", err)
	}

	return f, nil
}

// ExportsHandler implements extension.Arguments.
func (args Arguments) ExportsHandler() bool {
	return true
}

// Extensions implements extension.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements extension.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements extension.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
