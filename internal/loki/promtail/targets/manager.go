package targets

import (
	"github.com/grafana/alloy/internal/loki/promtail/positions"
	"github.com/grafana/alloy/internal/loki/promtail/targets/cloudflare"
	"github.com/grafana/alloy/internal/loki/promtail/targets/docker"
	"github.com/grafana/alloy/internal/loki/promtail/targets/file"
	"github.com/grafana/alloy/internal/loki/promtail/targets/gcplog"
	"github.com/grafana/alloy/internal/loki/promtail/targets/gelf"
	"github.com/grafana/alloy/internal/loki/promtail/targets/heroku"
	"github.com/grafana/alloy/internal/loki/promtail/targets/journal"
	"github.com/grafana/alloy/internal/loki/promtail/targets/syslog"
	"github.com/grafana/alloy/internal/loki/promtail/targets/target"
)

const (
	FileScrapeConfigs           = "fileScrapeConfigs"
	JournalScrapeConfigs        = "journalScrapeConfigs"
	SyslogScrapeConfigs         = "syslogScrapeConfigs"
	GcplogScrapeConfigs         = "gcplogScrapeConfigs"
	PushScrapeConfigs           = "pushScrapeConfigs"
	WindowsEventsConfigs        = "windowsEventsConfigs"
	KafkaConfigs                = "kafkaConfigs"
	GelfConfigs                 = "gelfConfigs"
	CloudflareConfigs           = "cloudflareConfigs"
	DockerSDConfigs             = "dockerSDConfigs"
	HerokuDrainConfigs          = "herokuDrainConfigs"
	AzureEventHubsScrapeConfigs = "azureeventhubsScrapeConfigs"
)

var (
	fileMetrics        *file.Metrics
	syslogMetrics      *syslog.Metrics
	gcplogMetrics      *gcplog.Metrics
	gelfMetrics        *gelf.Metrics
	cloudflareMetrics  *cloudflare.Metrics
	dockerMetrics      *docker.Metrics
	journalMetrics     *journal.Metrics
	herokuDrainMetrics *heroku.Metrics
)

type targetManager interface {
	Ready() bool
	Stop()
	ActiveTargets() map[string][]target.Target
	AllTargets() map[string][]target.Target
}

// TargetManagers manages a list of target managers.
type TargetManagers struct {
	targetManagers []targetManager
	positions      positions.Positions
}

// ActiveTargets returns active targets per jobs
func (tm *TargetManagers) ActiveTargets() map[string][]target.Target {
	result := map[string][]target.Target{}
	for _, t := range tm.targetManagers {
		for job, targets := range t.ActiveTargets() {
			result[job] = append(result[job], targets...)
		}
	}
	return result
}

// AllTargets returns all targets per jobs
func (tm *TargetManagers) AllTargets() map[string][]target.Target {
	result := map[string][]target.Target{}
	for _, t := range tm.targetManagers {
		for job, targets := range t.AllTargets() {
			result[job] = append(result[job], targets...)
		}
	}
	return result
}

// Ready if there's at least one ready target manager.
func (tm *TargetManagers) Ready() bool {
	for _, t := range tm.targetManagers {
		if t.Ready() {
			return true
		}
	}
	return false
}

// Stop the TargetManagers.
func (tm *TargetManagers) Stop() {
	for _, t := range tm.targetManagers {
		t.Stop()
	}
	if tm.positions != nil {
		tm.positions.Stop()
	}
}
