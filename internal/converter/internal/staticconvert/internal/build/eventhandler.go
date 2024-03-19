package build

import (
	"fmt"

	"github.com/grafana/agent/internal/component/common/loki"
	flow_relabel "github.com/grafana/agent/internal/component/common/relabel"
	"github.com/grafana/agent/internal/component/loki/relabel"
	"github.com/grafana/agent/internal/component/loki/source/kubernetes_events"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	eventhandler_v2 "github.com/grafana/agent/internal/static/integrations/v2/eventhandler"
	"github.com/grafana/river/scanner"
)

func (b *ConfigBuilder) appendEventHandlerV2(config *eventhandler_v2.Config) {
	compLabel, err := scanner.SanitizeIdentifier(b.formatJobName(config.Name(), nil))
	if err != nil {
		b.diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to sanitize job name: %s", err))
	}

	b.diags.AddAll(common.ValidateSupported(common.NotDeepEquals, config.SendTimeout, eventhandler_v2.DefaultConfig.SendTimeout, "eventhandler send_timeout", "this field is not configurable in flow mode"))
	b.diags.AddAll(common.ValidateSupported(common.NotDeepEquals, config.InformerResync, eventhandler_v2.DefaultConfig.InformerResync, "eventhandler informer_resync", "this field is not configurable in flow mode"))
	b.diags.AddAll(common.ValidateSupported(common.NotDeepEquals, config.FlushInterval, eventhandler_v2.DefaultConfig.FlushInterval, "eventhandler flush_interval", "this field is not configurable in flow mode"))

	if config.CachePath != eventhandler_v2.DefaultConfig.CachePath {
		b.diags.Add(
			diag.SeverityLevelWarn,
			"The eventhandler cache_path is unnecessary in flow mode because the storage path is governed by the --storage.path cmd argument and is always local to the component.",
		)
	}

	receiver := getLogsReceiver(config)
	if len(config.ExtraLabels) > 0 {
		receiver = b.injectExtraLabels(config, receiver, compLabel)
	}

	args := toEventHandlerV2(config, receiver)

	b.f.Body().AppendBlock(common.NewBlockWithOverride(
		[]string{"loki", "source", "kubernetes_events"},
		compLabel,
		args,
	))
}

func (b *ConfigBuilder) injectExtraLabels(config *eventhandler_v2.Config, receiver common.ConvertLogsReceiver, compLabel string) common.ConvertLogsReceiver {
	var relabelConfigs []*flow_relabel.Config
	for _, extraLabel := range config.ExtraLabels {
		defaultConfig := flow_relabel.DefaultRelabelConfig
		relabelConfig := &defaultConfig
		relabelConfig.SourceLabels = []string{"__address__"}
		relabelConfig.TargetLabel = extraLabel.Name
		relabelConfig.Replacement = extraLabel.Value

		relabelConfigs = append(relabelConfigs, relabelConfig)
	}

	relabelArgs := relabel.Arguments{
		ForwardTo:      []loki.LogsReceiver{receiver},
		RelabelConfigs: relabelConfigs,
		MaxCacheSize:   relabel.DefaultArguments.MaxCacheSize,
	}

	b.f.Body().AppendBlock(common.NewBlockWithOverride(
		[]string{"loki", "relabel"},
		compLabel,
		relabelArgs,
	))

	return common.ConvertLogsReceiver{
		Expr: fmt.Sprintf("loki.relabel.%s.receiver", compLabel),
	}
}

func getLogsReceiver(config *eventhandler_v2.Config) common.ConvertLogsReceiver {
	logsReceiver := common.ConvertLogsReceiver{}
	if config.LogsInstance != "" {
		compLabel, err := scanner.SanitizeIdentifier("logs_" + config.LogsInstance)
		if err != nil {
			panic(fmt.Errorf("failed to sanitize job name: %s", err))
		}

		logsReceiver.Expr = fmt.Sprintf("loki.write.%s.receiver", compLabel)
	}

	return logsReceiver
}

func toEventHandlerV2(config *eventhandler_v2.Config, receiver common.ConvertLogsReceiver) *kubernetes_events.Arguments {
	defaultOverrides := kubernetes_events.DefaultArguments
	defaultOverrides.Client.KubeConfig = config.KubeconfigPath
	if config.Namespace != "" {
		defaultOverrides.Namespaces = []string{config.Namespace}
	}

	return &kubernetes_events.Arguments{
		ForwardTo:  []loki.LogsReceiver{receiver},
		JobName:    kubernetes_events.DefaultArguments.JobName,
		Namespaces: defaultOverrides.Namespaces,
		LogFormat:  config.LogFormat,
		Client:     defaultOverrides.Client,
	}
}
