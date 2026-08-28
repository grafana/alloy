package build

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/kubernetes_events"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	eventhandler_v2 "github.com/grafana/alloy/internal/static/integrations/v2/eventhandler"
	"github.com/grafana/alloy/syntax/scanner"
	"github.com/prometheus/prometheus/model/labels"
)

func (b *ConfigBuilder) appendEventHandlerV2(config *eventhandler_v2.Config) {
	compLabel, err := scanner.SanitizeIdentifier(b.formatJobName(config.Name(), nil))
	if err != nil {
		b.diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to sanitize job name: %s", err))
	}

	b.diags.AddAll(common.ValidateSupported(common.NotDeepEquals, config.SendTimeout, eventhandler_v2.DefaultConfig.SendTimeout, "eventhandler send_timeout", "this field is not configurable in Alloy"))
	b.diags.AddAll(common.ValidateSupported(common.NotDeepEquals, config.InformerResync, eventhandler_v2.DefaultConfig.InformerResync, "eventhandler informer_resync", "this field is not configurable in Alloy"))
	b.diags.AddAll(common.ValidateSupported(common.NotDeepEquals, config.FlushInterval, eventhandler_v2.DefaultConfig.FlushInterval, "eventhandler flush_interval", "this field is not configurable in Alloy"))

	if config.CachePath != eventhandler_v2.DefaultConfig.CachePath {
		b.diags.Add(
			diag.SeverityLevelWarn,
			"The eventhandler cache_path is unnecessary in Alloy because the storage path is governed by the --storage.path cmd argument and is always local to the component.",
		)
	}

	receiver := getLogsReceiver(config)
	if config.ExtraLabels.Len() > 0 {
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
	var relabelConfigs []*alloy_relabel.Config
	config.ExtraLabels.Range(func(extraLabel labels.Label) {
		defaultConfig := alloy_relabel.DefaultRelabelConfig
		relabelConfig := &defaultConfig
		relabelConfig.SourceLabels = []string{"__address__"}
		relabelConfig.TargetLabel = extraLabel.Name
		relabelConfig.Replacement = extraLabel.Value

		relabelConfigs = append(relabelConfigs, relabelConfig)
	})

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
