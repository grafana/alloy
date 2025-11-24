package otelcolconvert

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/awss3"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
)

func init() {
	converters = append(converters, awss3ReceiverConverter{})
}

type awss3ReceiverConverter struct{}

func (awss3ReceiverConverter) Factory() component.Factory {
	return awss3receiver.NewFactory()
}

func (awss3ReceiverConverter) InputComponentName() string { return "" }

func (awss3ReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	label := state.AlloyComponentLabel()

	args, diags := toAWSS3Receiver(state, id, cfg.(*awss3receiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "awss3"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toAWSS3Receiver(state *State, id componentstatus.InstanceID, cfg *awss3receiver.Config) (*awss3.Arguments, diag.Diagnostics) {
	var diags diag.Diagnostics
	nextLogs := state.Next(id, pipeline.SignalLogs)

	// TODO(x1unix): map Encodings
	if len(cfg.Encodings) > 0 {
		diags.Add(
			diag.SeverityLevelWarn,
			fmt.Sprintf("%s: encodings are not supported at this moment", StringifyInstanceID(id)),
		)
	}

	if cfg.Notifications.OpAMP != nil {
		diags.Add(
			diag.SeverityLevelWarn,
			fmt.Sprintf("%s: notifications.opampextension field is not supported", StringifyInstanceID(id)),
		)
	}

	args := &awss3.Arguments{
		StartTime: cfg.StartTime,
		EndTime:   cfg.EndTime,
		S3Downloader: awss3.S3DownloaderConfig{
			Region:              cfg.S3Downloader.Region,
			S3Bucket:            cfg.S3Downloader.S3Bucket,
			S3Prefix:            cfg.S3Downloader.S3Prefix,
			S3Partition:         cfg.S3Downloader.S3Partition,
			FilePrefix:          cfg.S3Downloader.FilePrefix,
			Endpoint:            cfg.S3Downloader.Endpoint,
			EndpointPartitionID: cfg.S3Downloader.EndpointPartitionID,
			S3ForcePathStyle:    cfg.S3Downloader.S3ForcePathStyle,
		},
		Output: &otelcol.ConsumerArguments{
			Logs: ToTokenizedConsumers(nextLogs),
		},
	}

	if cfg.SQS != nil {
		args.SQS = &awss3.SQSConfig{
			QueueURL:            cfg.SQS.QueueURL,
			Region:              cfg.SQS.Region,
			Endpoint:            cfg.SQS.Endpoint,
			WaitTimeSeconds:     cfg.SQS.WaitTimeSeconds,
			MaxNumberOfMessages: cfg.SQS.MaxNumberOfMessages,
		}
	}

	return args, diags
}
