package common

import (
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/scrape"
)

// discoveryManager is an interface around discovery.Manager
type discoveryManager interface {
	Run() error
	SyncCh() <-chan map[string][]*targetgroup.Group
	ApplyConfig(cfg map[string]discovery.Configs) error
}

// scrapeManager is an interface around scrape.Manager
type scrapeManager interface {
	Run(tsets <-chan map[string][]*targetgroup.Group) error
	Stop()
	TargetsActive() map[string][]*scrape.Target
	ApplyConfig(cfg *config.Config) error
}
