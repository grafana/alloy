// Package features implements the features service, which exposes feature
// flags enabled on the command line to components.
package features

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"
)

// ServiceName defines the name used for the features service.
const ServiceName = "features"

// Feature is a flag that is enabled with --feature.<Name>.enabled.
type Feature struct {
	Name        string
	Description string
	// Stability is the minimum stability level required to enable the feature.
	Stability featuregate.Stability
}

// Flag returns the name of the command-line flag that enables f.
func (f Feature) Flag() string {
	return "feature." + f.Name + ".enabled"
}

var LokiConsumerPipeline = Feature{
	Name:        "loki.consumer-pipeline",
	Description: "Enable experimental consumer pipeline for passing logs between Loki components without channels",
	Stability:   featuregate.StabilityExperimental,
}

// All lists every feature that can be enabled.
var All = []Feature{
	LokiConsumerPipeline,
}

// Flags holds the command-line flag of every feature in All.
type Flags struct {
	set map[string]*bool
}

// RegisterFlags registers a flag for every feature in All with fset.
func RegisterFlags(fset *pflag.FlagSet) *Flags {
	f := &Flags{set: make(map[string]*bool, len(All))}
	for _, feature := range All {
		f.set[feature.Name] = fset.Bool(feature.Flag(), false, feature.Description)
	}
	return f
}

// Enabled returns the features enabled on the command line. It returns an error
// if a feature is enabled below its stability level.
func (f *Flags) Enabled(minStability featuregate.Stability) ([]Feature, error) {
	var enabled []Feature
	for _, feature := range All {
		if !*f.set[feature.Name] {
			continue
		}
		if err := featuregate.CheckAllowed(feature.Stability, minStability, "--"+feature.Flag()); err != nil {
			return nil, err
		}
		enabled = append(enabled, feature)
	}
	return enabled, nil
}

// Data is the runtime data exposed by the features service.
type Data interface {
	// Enabled reports whether f is enabled.
	Enabled(f Feature) bool
}

type Service struct {
	enabled map[string]bool
}

var (
	_ service.Service = (*Service)(nil)
	_ Data            = (*Service)(nil)
)

// New creates a new features service with the provided features enabled.
func New(enabled ...Feature) *Service {
	s := &Service{enabled: make(map[string]bool, len(enabled))}
	for _, f := range enabled {
		s.enabled[f.Name] = true
	}
	return s
}

func (s *Service) Enabled(f Feature) bool {
	return s.enabled[f.Name]
}

func (s *Service) Data() any {
	return s
}

func (*Service) Definition() service.Definition {
	return service.Definition{
		Name:       ServiceName,
		ConfigType: nil, // features does not accept configuration
		DependsOn:  []string{},
		Stability:  featuregate.StabilityGenerallyAvailable,
	}
}

func (*Service) Run(ctx context.Context, _ service.Host) error {
	<-ctx.Done()
	return nil
}

func (*Service) Update(_ any) error {
	return fmt.Errorf("features service does not support configuration")
}

// Enabled reports whether f is enabled using the provided service lookup. It
// returns false when the features service is not available.
func Enabled(getServiceData func(name string) (any, error), f Feature) bool {
	data, err := getServiceData(ServiceName)
	if err != nil {
		return false
	}
	d, ok := data.(Data)
	return ok && d.Enabled(f)
}
