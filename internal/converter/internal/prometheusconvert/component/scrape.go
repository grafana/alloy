package component

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/maps"

	prom_config "github.com/prometheus/prometheus/config"
	prom_discovery "github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/scrape"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert/build"
	"github.com/grafana/alloy/internal/service/cluster"
)

func AppendPrometheusScrape(pb *build.PrometheusBlocks, scrapeConfig *prom_config.ScrapeConfig, forwardTo []storage.Appendable, targets []discovery.Target, label string) {
	scrapeArgs := toScrapeArguments(scrapeConfig, forwardTo, targets)
	name := []string{"prometheus", "scrape"}
	block := common.NewBlockWithOverride(name, label, scrapeArgs)
	summary := fmt.Sprintf("Converted scrape_configs job_name %q into...", scrapeConfig.JobName)
	detail := fmt.Sprintf("	A %s.%s component", strings.Join(name, "."), label)
	pb.PrometheusScrapeBlocks = append(pb.PrometheusScrapeBlocks, build.NewPrometheusBlock(block, name, label, summary, detail))
}

func ValidatePrometheusScrape(scrapeConfig *prom_config.ScrapeConfig) diag.Diagnostics {
	var diags diag.Diagnostics

	// https://github.com/grafana/agent/pull/5972#discussion_r1441980155
	diags.AddAll(common.ValidateSupported(common.NotEquals, scrapeConfig.TrackTimestampsStaleness, false, "scrape_configs track_timestamps_staleness", ""))
	// https://github.com/prometheus/prometheus/commit/40240c9c1cb290fe95f1e61886b23fab860aeacd
	diags.AddAll(common.ValidateSupported(common.NotEquals, scrapeConfig.NativeHistogramBucketLimit, uint(0), "scrape_configs native_histogram_bucket_limit", ""))
	// https://github.com/prometheus/prometheus/pull/12647
	diags.AddAll(common.ValidateSupported(common.NotEquals, scrapeConfig.KeepDroppedTargets, uint(0), "scrape_configs keep_dropped_targets", ""))
	diags.AddAll(common.ValidateHttpClientConfig(&scrapeConfig.HTTPClientConfig))

	return diags
}

func toScrapeArguments(scrapeConfig *prom_config.ScrapeConfig, forwardTo []storage.Appendable, targets []discovery.Target) *scrape.Arguments {
	if scrapeConfig == nil {
		return nil
	}

	return &scrape.Arguments{
		Targets:                   targets,
		ForwardTo:                 forwardTo,
		JobName:                   scrapeConfig.JobName,
		HonorLabels:               scrapeConfig.HonorLabels,
		HonorTimestamps:           scrapeConfig.HonorTimestamps,
		TrackTimestampsStaleness:  scrapeConfig.TrackTimestampsStaleness,
		Params:                    scrapeConfig.Params,
		ScrapeClassicHistograms:   scrapeConfig.ScrapeClassicHistograms,
		ScrapeNativeHistograms:    true,
		ScrapeInterval:            time.Duration(scrapeConfig.ScrapeInterval),
		ScrapeTimeout:             time.Duration(scrapeConfig.ScrapeTimeout),
		ScrapeProtocols:           convertScrapeProtocols(scrapeConfig.ScrapeProtocols),
		MetricsPath:               scrapeConfig.MetricsPath,
		Scheme:                    scrapeConfig.Scheme,
		BodySizeLimit:             scrapeConfig.BodySizeLimit,
		SampleLimit:               scrapeConfig.SampleLimit,
		TargetLimit:               scrapeConfig.TargetLimit,
		LabelLimit:                scrapeConfig.LabelLimit,
		LabelNameLengthLimit:      scrapeConfig.LabelNameLengthLimit,
		LabelValueLengthLimit:     scrapeConfig.LabelValueLengthLimit,
		HTTPClientConfig:          *common.ToHttpClientConfig(&scrapeConfig.HTTPClientConfig),
		ExtraMetrics:              false,
		EnableProtobufNegotiation: false,
		Clustering:                cluster.ComponentBlock{Enabled: false},
	}
}

func getScrapeTargets(staticConfig prom_discovery.StaticConfig) []discovery.Target {
	targets := []discovery.Target{}

	for _, target := range staticConfig {
		targetMap := map[string]string{}

		for labelName, labelValue := range target.Labels {
			targetMap[string(labelName)] = string(labelValue)
		}

		for _, labelSet := range target.Targets {
			for labelName, labelValue := range labelSet {
				targetMap[string(labelName)] = string(labelValue)
				newMap := map[string]string{}
				maps.Copy(newMap, targetMap)
				targets = append(targets, newMap)
			}
		}
	}

	return targets
}

func convertScrapeProtocols(protocols []prom_config.ScrapeProtocol) []string {
	result := make([]string, 0, len(protocols))
	for _, protocol := range protocols {
		result = append(result, string(protocol))
	}
	return result
}

func ValidateScrapeTargets(staticConfig prom_discovery.StaticConfig) diag.Diagnostics {
	return make(diag.Diagnostics, 0)
}
