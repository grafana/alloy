package promtailconvert

import (
	promtailcfg "github.com/grafana/alloy/internal/loki/promtail/config"

	"github.com/grafana/alloy/internal/converter/diag"
)

// validateTopLevelConfig validates the top-level config for any unsupported features. There may still be some
// other unsupported features in scope of each config block, which are raised by their respective conversion code.
func validateTopLevelConfig(cfg *promtailcfg.Config, diags *diag.Diagnostics) {
	// WAL support is still work in progress and not documented. Enabling it won't work, so it's an error.
	if cfg.WAL.Enabled {
		diags.Add(
			diag.SeverityLevelError,
			"Promtail's WAL is currently not supported in Alloy",
		)
	}

	if cfg.LimitsConfig.ReadlineRateEnabled {
		diags.Add(
			diag.SeverityLevelWarn,
			"limits_config rate limiting settings (readline_rate, readline_burst, readline_rate_drop) have been "+
				"converted to stage.limit stages in each loki.process component. Unlike Promtail's global rate "+
				"limiter, these limits apply independently per pipeline rather than across all pipelines combined.",
		)
	}

	// We cannot migrate the tracing config to Alloy, since in promtail it relies on
	// environment variables that can be set or not and depending on what is set, different
	// features of tracing are configured. We'd need to have conditionals in the
	// Alloy config to translate this. See https://www.jaegertracing.io/docs/1.16/client-features/
	if cfg.Tracing.Enabled {
		diags.Add(
			diag.SeverityLevelWarn,
			"If you have a tracing set up for Promtail, it cannot be migrated to Alloy automatically. "+
				"Refer to the documentation on how to configure tracing in Alloy.",
		)
	}

	if cfg.TargetConfig.Stdin {
		diags.Add(
			diag.SeverityLevelError,
			"reading targets from stdin is not supported in Alloy configuration file",
		)
	}
	if cfg.ServerConfig.ProfilingEnabled {
		diags.Add(diag.SeverityLevelWarn, "server.profiling_enabled is not supported - use Alloy's "+
			"main HTTP server's profiling endpoints instead")
	}

	if cfg.ServerConfig.RegisterInstrumentation {
		diags.Add(
			diag.SeverityLevelWarn,
			"Alloy's metrics are different from the metrics emitted by Promtail. If you "+
				"rely on Promtail's metrics, you must change your configuration, for example, your alerts and dashboards.",
		)
	}

	if cfg.ServerConfig.LogLevel.String() != "info" {
		diags.Add(diag.SeverityLevelWarn, "The converter does not support converting the provided server.log_level config: "+
			"The equivalent feature in Alloy is to use the logging config block to set the level argument.")
	}

	if cfg.ServerConfig.PathPrefix != "" {
		diags.Add(diag.SeverityLevelError, "server.http_path_prefix is not supported")
	}

	if cfg.ServerConfig.HealthCheckTarget != nil && !*cfg.ServerConfig.HealthCheckTarget {
		diags.Add(diag.SeverityLevelWarn, "server.health_check_target disabling is not supported in Alloy")
	}
}
