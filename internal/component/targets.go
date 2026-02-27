package component

import "time"

// TargetsProvider is implemented by components that have scrape targets.
// Components like prometheus.scrape and pyroscope.scrape implement this
// interface to expose their target status information via the API.
type TargetsProvider interface {
	GetTargets() []TargetInfo
}

// TargetInfo represents the status of a single scrape target.
type TargetInfo struct {
	JobName            string            // Name of the scrape job
	Endpoint           string            // URL being scraped
	State              string            // Target state: up, down, or unknown
	Labels             map[string]string // Labels applied to scraped metrics
	DiscoveredLabels   map[string]string // Labels discovered during service discovery
	LastScrape         time.Time         // Timestamp of the last scrape attempt
	LastScrapeDuration time.Duration     // Duration of the last scrape
	LastError          string            // Error from the last scrape, empty if successful
}
