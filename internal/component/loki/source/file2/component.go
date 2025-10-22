package file2

import (
	"encoding"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"strings"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.file2",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Build:     build,
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

// Receivers implements source.Arguments.
func (a Arguments) Receivers() []loki.LogsReceiver {
	return a.ForwardTo
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

func supportedCompressedFormats() map[string]struct{} {
	return map[string]struct{}{
		"gz":  {},
		"z":   {},
		"bz2": {},
		// TODO: add support for zip.
	}
}

func build(opts component.Options, cargs component.Arguments) (component.Component, error) {
	args := cargs.(Arguments)

	newPositionsPath := filepath.Join(opts.DataPath, "positions.yml")
	// Check to see if we can convert the legacy positions file to the new format.
	if args.LegacyPositionsFile != "" {
		positions.ConvertLegacyPositionsFile(args.LegacyPositionsFile, newPositionsPath, opts.Logger)
	}
	return source.New(opts, args, &targetsFactory{opts: opts, metrics: newMetrics(opts.Registerer)})
}
