package scrape

import (
	"sort"

	"github.com/prometheus/prometheus/model/labels"
	promscrape "github.com/prometheus/prometheus/scrape"

	"github.com/grafana/alloy/internal/component"
)

var _ component.TargetProvider = (*Component)(nil)

// ComponentTargets returns a point-in-time snapshot of active scrape targets.
func (c *Component) ComponentTargets() []component.TargetInfo {
	return buildComponentTargets(c.scraper.TargetsActive())
}

func buildComponentTargets(targets map[string][]*promscrape.Target) []component.TargetInfo {
	count := 0
	for _, jobTargets := range targets {
		count += len(jobTargets)
	}

	result := make([]component.TargetInfo, 0, count)
	builder := labels.NewBuilder(labels.EmptyLabels())
	for _, jobTargets := range targets {
		for _, target := range jobTargets {
			if target == nil {
				continue
			}

			discoveredLabels := target.DiscoveredLabels(builder)
			publicLabels := target.Labels(builder)
			result = append(result, component.TargetInfo{
				Labels:        discoveredLabels.Map(),
				NonMetaLabels: publicLabels.Map(),
				Hash:          discoveredLabels.Hash(),
				NonMetaHash:   publicLabels.Hash(),
				Scrape: &component.ScrapeTargetInfo{
					URL:          target.URL().String(),
					Health:       componentScrapeTargetHealth(target.Health()),
					LastAttempt:  target.LastScrape(),
					LastDuration: target.LastScrapeDuration(),
					LastError:    target.LastError(),
				},
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Hash != result[j].Hash {
			return result[i].Hash < result[j].Hash
		}
		return result[i].Scrape.URL < result[j].Scrape.URL
	})
	return result
}

func componentScrapeTargetHealth(health promscrape.TargetHealth) component.ScrapeTargetHealth {
	switch health {
	case promscrape.HealthGood:
		return component.ScrapeTargetHealthUp
	case promscrape.HealthBad:
		return component.ScrapeTargetHealthDown
	default:
		return component.ScrapeTargetHealthUnknown
	}
}
