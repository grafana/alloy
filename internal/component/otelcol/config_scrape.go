package otelcol

import (
	"errors"
	"fmt"
	"time"

	scraperhelper "go.opentelemetry.io/collector/receiver/scraperhelper"
)

var (
	errNonPositiveInterval = errors.New("requires positive value")
	errGreaterThanZero     = errors.New("requires a value greater than zero")
)

// ScraperControllerArguments defines common settings for a scraper controller
// configuration.
type ScraperControllerArguments struct {
	CollectionInterval time.Duration `river:"collection_interval,attr,optional"`
	InitialDelay       time.Duration `river:"initial_delay,attr,optional"`
	Timeout            time.Duration `river:"timeout,attr,optional"`
}

// DefaultScraperControllerArguments holds default settings for ScraperControllerArguments.
var DefaultScraperControllerArguments = ScraperControllerArguments{
	CollectionInterval: time.Minute,
	InitialDelay:       time.Second,
	Timeout:            0 * time.Second,
}

// SetToDefault implements river.Defaulter.
func (args *ScraperControllerArguments) SetToDefault() {
	*args = DefaultScraperControllerArguments
}

// Convert converts args into the upstream type.
func (args *ScraperControllerArguments) Convert() *scraperhelper.ScraperControllerSettings {
	if args == nil {
		return nil
	}

	return &scraperhelper.ScraperControllerSettings{
		CollectionInterval: args.CollectionInterval,
		InitialDelay:       args.InitialDelay,
		Timeout:            args.Timeout,
	}
}

// Validate returns an error if args is invalid.
func (args *ScraperControllerArguments) Validate() error {
	if args.CollectionInterval <= 0 {
		return fmt.Errorf(`"collection_interval": %w`, errNonPositiveInterval)
	}
	if args.Timeout < 0 {
		return fmt.Errorf(`"timeout": %w`, errGreaterThanZero)
	}
	return nil
}
