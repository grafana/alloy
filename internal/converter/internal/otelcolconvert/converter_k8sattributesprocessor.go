package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/k8sattributes"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, k8sAttributesProcessorConverter{})
}

type k8sAttributesProcessorConverter struct{}

func (k8sAttributesProcessorConverter) Factory() component.Factory {
	return k8sattributesprocessor.NewFactory()
}

func (k8sAttributesProcessorConverter) InputComponentName() string {
	return "otelcol.processor.k8sattributes"
}

func (k8sAttributesProcessorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toK8SAttributesProcessor(state, id, cfg.(*k8sattributesprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "k8sattributes"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toK8SAttributesProcessor(state *State, id componentstatus.InstanceID, cfg *k8sattributesprocessor.Config) *k8sattributes.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	return &k8sattributes.Arguments{
		AuthType:    string(cfg.AuthType),
		Passthrough: cfg.Passthrough,
		ExtractConfig: k8sattributes.ExtractConfig{
			Metadata:        cfg.Extract.Metadata,
			Annotations:     toFilterExtract(cfg.Extract.Annotations),
			Labels:          toFilterExtract(cfg.Extract.Labels),
			OtelAnnotations: cfg.Extract.OtelAnnotations,
		},
		Filter: k8sattributes.FilterConfig{
			Node:      cfg.Filter.Node,
			Namespace: cfg.Filter.Namespace,
			Fields:    toFilterFields(cfg.Filter.Fields),
			Labels:    toFilterFields(cfg.Filter.Labels),
		},
		PodAssociations:        toPodAssociations(cfg.Association),
		Exclude:                toExclude(cfg.Exclude),
		WaitForMetadata:        cfg.WaitForMetadata,
		WaitForMetadataTimeout: cfg.WaitForMetadataTimeout,

		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},

		DebugMetrics: common.DefaultValue[k8sattributes.Arguments]().DebugMetrics,
	}
}

func toExclude(cfg k8sattributesprocessor.ExcludeConfig) k8sattributes.ExcludeConfig {
	res := k8sattributes.ExcludeConfig{
		Pods: []k8sattributes.ExcludePodConfig{},
	}

	for _, c := range cfg.Pods {
		res.Pods = append(res.Pods, k8sattributes.ExcludePodConfig{
			Name: c.Name,
		})
	}

	return res
}

func toPodAssociations(cfg []k8sattributesprocessor.PodAssociationConfig) []k8sattributes.PodAssociation {
	if len(cfg) == 0 {
		return nil
	}

	res := make([]k8sattributes.PodAssociation, 0, len(cfg))

	for i, c := range cfg {
		res = append(res, k8sattributes.PodAssociation{
			Sources: []k8sattributes.PodAssociationSource{},
		})

		for _, c2 := range c.Sources {
			res[i].Sources = append(res[i].Sources, k8sattributes.PodAssociationSource{
				From: c2.From,
				Name: c2.Name,
			})
		}
	}

	return res
}
func toFilterExtract(cfg []k8sattributesprocessor.FieldExtractConfig) []k8sattributes.FieldExtractConfig {
	if len(cfg) == 0 {
		return nil
	}

	res := make([]k8sattributes.FieldExtractConfig, 0, len(cfg))

	for _, c := range cfg {
		res = append(res, k8sattributes.FieldExtractConfig{
			TagName:  c.TagName,
			Key:      c.Key,
			KeyRegex: c.KeyRegex,
			From:     c.From,
		})
	}

	return res
}

func toFilterFields(cfg []k8sattributesprocessor.FieldFilterConfig) []k8sattributes.FieldFilterConfig {
	if len(cfg) == 0 {
		return nil
	}

	res := make([]k8sattributes.FieldFilterConfig, 0, len(cfg))

	for _, c := range cfg {
		res = append(res, k8sattributes.FieldFilterConfig{
			Key:   c.Key,
			Value: c.Value,
			Op:    c.Op,
		})
	}

	return res
}
