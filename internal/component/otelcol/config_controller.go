package otelcol

import (
	"time"

	"github.com/grafana/alloy/syntax"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

// ControllerArguments defines common settings for a scraper controller
// configuration. Scraper controller receivers can squash this struct, instead
// of receiver.Settings, and extend it with more fields if needed.
type ControllerArguments struct {
	// CollectionInterval sets how frequently the scraper should be called and
	// used as the context timeout to ensure that scrapers don't exceed the
	// interval.
	CollectionInterval time.Duration `alloy:"collection_interval,attr,optional"`
	// InitialDelay sets the initial start delay for the scraper, any non
	// positive value is assumed to be immediately.
	InitialDelay time.Duration `alloy:"initial_delay,attr,optional"`
	// Timeout is an optional value used to set scraper's context deadline.
	Timeout time.Duration `alloy:"timeout,attr,optional"`
}

var (
	_ syntax.Defaulter = (*ControllerArguments)(nil)
	_ syntax.Validator = (*ControllerArguments)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (args *ControllerArguments) SetToDefault() {
	*args = ControllerArguments{
		CollectionInterval: time.Minute,
		InitialDelay:       time.Second,
		Timeout:            0,
	}
}

// Validate implements syntax.Validator.
func (args *ControllerArguments) Validate() error {
	return args.Convert().Validate()
}

// Convert converts args to the upstream type.
func (args *ControllerArguments) Convert() *scraperhelper.ControllerConfig {
	if args == nil {
		return nil
	}

	return &scraperhelper.ControllerConfig{
		CollectionInterval: args.CollectionInterval,
		InitialDelay:       args.InitialDelay,
		Timeout:            args.Timeout,
	}
}
