package component

import "time"

// TargetProvider is implemented by components that own runtime target state.
type TargetProvider interface {
	ComponentTargets() []TargetInfo
}

// TargetInfo is a point-in-time snapshot of a component-owned target.
type TargetInfo struct {
	Labels        map[string]string
	NonMetaLabels map[string]string
	Hash          uint64
	NonMetaHash   uint64
	Scrape        *ScrapeTargetInfo
}

// ScrapeTargetInfo is runtime state for a Prometheus scrape target.
type ScrapeTargetInfo struct {
	URL          string
	Health       ScrapeTargetHealth
	LastAttempt  time.Time
	LastDuration time.Duration
	LastError    error
}

// ScrapeTargetHealth is the health reported by the latest scrape attempt.
type ScrapeTargetHealth string

const (
	ScrapeTargetHealthUnknown ScrapeTargetHealth = "unknown"
	ScrapeTargetHealthUp      ScrapeTargetHealth = "up"
	ScrapeTargetHealthDown    ScrapeTargetHealth = "down"
)
