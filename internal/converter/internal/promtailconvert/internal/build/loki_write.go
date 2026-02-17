package build

import (
	"fmt"

	"github.com/alecthomas/units"

	"github.com/grafana/alloy/internal/component/common/loki"
	lokiwrite "github.com/grafana/alloy/internal/component/loki/write"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/loki/promtail/client"
	"github.com/grafana/alloy/internal/loki/util/flagext"
	"github.com/grafana/alloy/syntax/token/builder"
)

func NewLokiWrite(client *client.Config, diags *diag.Diagnostics, index int, labelPrefix string) (*builder.Block, loki.LogsReceiver) {
	label := "default"
	if labelPrefix != "" {
		label = labelPrefix
	}

	lokiWriteLabel := common.LabelWithIndex(index, label)

	lokiWriteArgs := toLokiWriteArguments(client, diags)
	block := common.NewBlockWithOverride([]string{"loki", "write"}, lokiWriteLabel, lokiWriteArgs)
	return block, common.ConvertLogsReceiver{
		Expr: fmt.Sprintf("loki.write.%s.receiver", lokiWriteLabel),
	}
}

func toLokiWriteArguments(config *client.Config, diags *diag.Diagnostics) *lokiwrite.Arguments {
	batchSize, err := units.ParseBase2Bytes(fmt.Sprintf("%dB", config.BatchSize))
	if err != nil {
		diags.Add(
			diag.SeverityLevelError,
			fmt.Sprintf("failed to parse BatchSize for client config %s: %s", config.Name, err.Error()),
		)
	}

	return &lokiwrite.Arguments{
		Endpoints: []lokiwrite.EndpointOptions{
			{
				Name:              config.Name,
				URL:               config.URL.String(),
				BatchWait:         config.BatchWait,
				BatchSize:         batchSize,
				HTTPClientConfig:  common.ToHttpClientConfig(&config.Client),
				Headers:           config.Headers,
				MinBackoff:        config.BackoffConfig.MinBackoff,
				MaxBackoff:        config.BackoffConfig.MaxBackoff,
				MaxBackoffRetries: config.BackoffConfig.MaxRetries,
				RemoteTimeout:     config.Timeout,
				TenantID:          config.TenantID,
				RetryOnHTTP429:    !config.DropRateLimitedBatches,
				QueueConfig:       lokiwrite.GetDefaultEndpointOptions().QueueConfig,
			},
		},
		ExternalLabels: convertFlagLabels(config.ExternalLabels),
	}
}

func convertFlagLabels(labels flagext.LabelSet) map[string]string {
	result := map[string]string{}
	for k, v := range labels.LabelSet {
		result[string(k)] = string(v)
	}
	return result
}
