package build

import (
	"fmt"

	"github.com/alecthomas/units"

	"github.com/grafana/alloy/internal/component/loki/process/stages"
	"github.com/grafana/alloy/internal/loki/promtail/limit"
)

// buildLimitsConfigStages converts the global Promtail limits_config into
// equivalent loki.process pipeline stages.
//
// Note: limits_config.max_streams has no equivalent in Alloy's pipeline stages
// and is silently omitted. A diagnostic warning is emitted by the top-level
// validator when max_streams is set.
//
// The conversion is inherently approximate: Promtail's limits apply globally
// across all pipelines, whereas the returned stages are injected into each
// per-scrape-config loki.process component individually.
func buildLimitsConfigStages(cfg limit.Config) []stages.StageConfig {
	var result []stages.StageConfig

	if cfg.ReadlineRateEnabled {
		result = append(result, stages.StageConfig{
			LimitConfig: &stages.LimitConfig{
				Rate:  cfg.ReadlineRate,
				Burst: cfg.ReadlineBurst,
				Drop:  cfg.ReadlineRateDrop,
			},
		})
	}

	if cfg.MaxLineSize > 0 {
		lineSizeBytes, err := units.ParseBase2Bytes(fmt.Sprintf("%dB", cfg.MaxLineSize.Val()))
		if err != nil {
			// MaxLineSize.Val() returns an int of raw bytes, so "%dB" is always valid.
			// This branch is unreachable in practice.
			return result
		}
		if cfg.MaxLineSizeTruncate {
			result = append(result, stages.StageConfig{
				TruncateConfig: &stages.TruncateConfig{
					Rules: []*stages.RuleConfig{
						{
							Limit:      lineSizeBytes,
							SourceType: stages.TruncateSourceLine,
						},
					},
				},
			})
		} else {
			result = append(result, stages.StageConfig{
				DropConfig: &stages.DropConfig{
					LongerThan: lineSizeBytes,
				},
			})
		}
	}

	return result
}
