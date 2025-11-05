package staticconvert

import (
	"bytes"
	"flag"
	"fmt"

	prom_config "github.com/prometheus/prometheus/config"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert"
	"github.com/grafana/alloy/internal/converter/internal/promtailconvert"
	"github.com/grafana/alloy/internal/converter/internal/staticconvert/internal/build"
	promtail_config "github.com/grafana/alloy/internal/loki/promtail/config"
	"github.com/grafana/alloy/internal/loki/promtail/file"
	"github.com/grafana/alloy/internal/loki/promtail/limit"
	"github.com/grafana/alloy/internal/static/config"
	"github.com/grafana/alloy/internal/static/logs"
	"github.com/grafana/alloy/syntax/scanner"
	"github.com/grafana/alloy/syntax/token/builder"

	_ "github.com/grafana/alloy/internal/static/integrations/install" // Install integrations
)

// Convert implements a Static config converter.
//
// extraArgs are supported to be passed along to the Static config parser such
// as enabling integrations-next.
func Convert(in []byte, extraArgs []string) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	args := []string{"-config.file", "convert"}
	args = append(args, extraArgs...)
	staticConfig, err := config.LoadFromFunc(fs, args, func(_, _ string, expandEnvVars bool, c *config.Config) error {
		return config.LoadBytes(in, expandEnvVars, c)
	})

	if err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to parse Static config: %s", err))
		return nil, diags
	}

	f := builder.NewFile()
	diags = AppendAll(f, staticConfig)
	diags.AddAll(common.ValidateNodes(f))

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to render Alloy config: %s", err.Error()))
		return nil, diags
	}

	if len(buf.Bytes()) == 0 {
		return nil, diags
	}

	prettyByte, newDiags := common.PrettyPrint(buf.Bytes())
	diags.AddAll(newDiags)
	return prettyByte, diags
}

// AppendAll analyzes the entire static config in memory and transforms it into
// Alloy component Arguments. It then appends each argument to the file
// builder. Exports from other components are correctly referenced to build the
// Alloy pipeline.
func AppendAll(f *builder.File, staticConfig *config.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.AddAll(appendStaticPrometheus(f, staticConfig))
	diags.AddAll(appendStaticPromtail(f, staticConfig))
	diags.AddAll(appendStaticConfig(f, staticConfig))

	diags.AddAll(validate(staticConfig))

	return diags
}

func appendStaticPrometheus(f *builder.File, staticConfig *config.Config) diag.Diagnostics {
	var diags diag.Diagnostics
	for _, instance := range staticConfig.Metrics.Configs {
		promConfig := &prom_config.Config{
			GlobalConfig:       staticConfig.Metrics.Global.Prometheus,
			ScrapeConfigs:      instance.ScrapeConfigs,
			RemoteWriteConfigs: instance.RemoteWrite,
		}

		jobNameToCompLabelsFunc := func(jobName string) string {
			name := fmt.Sprintf("metrics_%s", instance.Name)
			if jobName != "" {
				name += fmt.Sprintf("_%s", jobName)
			}

			name, err := scanner.SanitizeIdentifier(name)
			if err != nil {
				diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to sanitize job name: %s", err))
			}

			return name
		}

		// There is an edge case here with label collisions that will be caught
		// by a validation [common.ValidateNodes].
		// For example,
		//   metrics config name = "agent_test"
		//   scrape config job_name = "prometheus"
		//
		//   metrics config name = "agent"
		//   scrape config job_name = "test_prometheus"
		//
		//   results in two prometheus.scrape components with the label "metrics_agent_test_prometheus"
		diags.AddAll(prometheusconvert.AppendAllNested(f, promConfig, jobNameToCompLabelsFunc, []discovery.Target{}, nil))
	}

	return diags
}

func appendStaticPromtail(f *builder.File, staticConfig *config.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	if staticConfig.Logs == nil {
		return diags
	}

	for _, logConfig := range staticConfig.Logs.Configs {
		promtailConfig := logs.DefaultConfig()
		promtailConfig.Global = promtail_config.GlobalConfig{FileWatch: staticConfig.Logs.Global.FileWatch}
		promtailConfig.ClientConfigs = logConfig.ClientConfigs
		promtailConfig.PositionsConfig = logConfig.PositionsConfig
		promtailConfig.ScrapeConfig = logConfig.ScrapeConfig
		promtailConfig.TargetConfig = logConfig.TargetConfig
		promtailConfig.LimitsConfig = logConfig.LimitsConfig

		// We are using the
		err := promtailConfig.ServerConfig.Config.LogLevel.Set("info")
		if err != nil {
			panic("unable to set default promtail log level from the static converter.")
		}

		// We need to set this when empty so the promtail converter doesn't think it has been overridden
		if promtailConfig.Global == (promtail_config.GlobalConfig{}) {
			promtailConfig.Global.FileWatch = file.DefaultWatchConfig
		}

		if promtailConfig.LimitsConfig == (limit.Config{}) {
			promtailConfig.LimitsConfig = promtailconvert.DefaultLimitsConfig()
		}

		// There is an edge case here with label collisions that will be caught
		// by a validation [common.ValidateNodes].
		// For example,
		//   logs config name = "agent_test"
		//   scrape config job_name = "promtail"
		//
		//   logs config name = "agent"
		//   scrape config job_name = "test_promtail"
		//
		//   results in two prometheus.scrape components with the label "logs_agent_test_promtail"
		diags = promtailconvert.AppendAll(f, &promtailConfig, "logs_"+logConfig.Name, diags)
	}

	return diags
}

func appendStaticConfig(f *builder.File, staticConfig *config.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	b := build.NewConfigBuilder(f, &diags, staticConfig, &build.GlobalContext{IntegrationsLabelPrefix: "integrations"})
	b.Build()

	return diags
}
