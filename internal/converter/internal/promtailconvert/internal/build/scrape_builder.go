package build

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/discovery/relabel"
	"github.com/grafana/alloy/internal/component/loki/process"
	"github.com/grafana/alloy/internal/component/loki/process/stages"
	lokirelabel "github.com/grafana/alloy/internal/component/loki/relabel"
	lokisourcefile "github.com/grafana/alloy/internal/component/loki/source/file"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert/component"
	"github.com/grafana/alloy/internal/loki/promtail/file"
	"github.com/grafana/alloy/internal/loki/promtail/positions"
	"github.com/grafana/alloy/internal/loki/promtail/scrapeconfig"
	"github.com/grafana/alloy/syntax/scanner"
	"github.com/grafana/alloy/syntax/token/builder"
)

type ScrapeConfigBuilder struct {
	f         *builder.File
	diags     *diag.Diagnostics
	cfg       *scrapeconfig.Config
	globalCtx *GlobalContext

	allTargetsExps            []string
	processStageReceivers     []loki.LogsReceiver
	allRelabeledTargetsExpr   string
	discoveryRelabelRulesExpr string
	lokiRelabelReceiverExpr   string
}

func NewScrapeConfigBuilder(
	f *builder.File,
	diags *diag.Diagnostics,
	cfg *scrapeconfig.Config,
	globalCtx *GlobalContext,
) *ScrapeConfigBuilder {

	return &ScrapeConfigBuilder{
		f:         f,
		diags:     diags,
		cfg:       cfg,
		globalCtx: globalCtx,
	}
}

func (s *ScrapeConfigBuilder) Sanitize() {
	var err error
	s.cfg.JobName, err = scanner.SanitizeIdentifier(s.cfg.JobName)
	if err != nil {
		s.diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to sanitize job name: %s", err))
	}
}

func (s *ScrapeConfigBuilder) AppendLokiSourceFile(watchConfig *file.WatchConfig, positionsCfg *positions.Config) {
	// If there were no targets expressions collected, that means
	// we didn't have any components that produced SD targets, so
	// we can skip this component.
	if len(s.allTargetsExps) == 0 {
		return
	}

	args := lokisourcefile.Arguments{
		ForwardTo:            s.getOrNewProcessStageReceivers(),
		Encoding:             s.cfg.Encoding,
		DecompressionConfig:  convertDecompressionConfig(s.cfg.DecompressionCfg),
		FileWatch:            convertFileWatchConfig(watchConfig),
		LegacyPositionsFile:  positionsCfg.PositionsFile,
		OnPositionsFileError: lokisourcefile.OnPositionsFileErrorRestartBeginning,
		FileMatch:            convertFileMatchConfig(s.globalCtx.TargetSyncPeriod),
	}

	targets := s.getAllRelabeledTargetsExpr()
	overrideHook := func(val any) any {
		if _, ok := val.([]discovery.Target); ok {
			return common.CustomTokenizer{Expr: targets}
		}
		return val
	}

	compLabel := common.LabelForParts(s.globalCtx.LabelPrefix, s.cfg.JobName)
	s.f.Body().AppendBlock(common.NewBlockWithOverrideFn(
		[]string{"loki", "source", "file"},
		compLabel,
		args,
		overrideHook,
	))
}

func (s *ScrapeConfigBuilder) getOrNewLokiRelabel() string {
	if len(s.cfg.RelabelConfigs) == 0 {
		// If no relabels - we can send straight to the process stage.
		return logsReceiversToExpr(s.getOrNewProcessStageReceivers())
	}

	if s.lokiRelabelReceiverExpr == "" {
		args := lokirelabel.Arguments{
			ForwardTo:      s.getOrNewProcessStageReceivers(),
			RelabelConfigs: component.ToAlloyRelabelConfigs(s.cfg.RelabelConfigs),
			// max_cache_size doesnt exist in static, and we need to manually set it to default.
			// Since the default is 10_000 if we didnt set the value, it would compare the default 10k to 0 and emit 0.
			// We actually dont want to emit anything since this setting doesnt exist in static, setting to 10k matches the default
			// and ensures it doesnt get emitted.
			MaxCacheSize: lokirelabel.DefaultArguments.MaxCacheSize,
		}
		compLabel := common.LabelForParts(s.globalCtx.LabelPrefix, s.cfg.JobName)
		s.f.Body().AppendBlock(common.NewBlockWithOverride([]string{"loki", "relabel"}, compLabel, args))
		s.lokiRelabelReceiverExpr = "[loki.relabel." + compLabel + ".receiver]"
	}
	return s.lokiRelabelReceiverExpr
}

func (s *ScrapeConfigBuilder) getOrNewProcessStageReceivers() []loki.LogsReceiver {
	if s.processStageReceivers != nil {
		return s.processStageReceivers
	}
	if len(s.cfg.PipelineStages) == 0 {
		s.processStageReceivers = s.globalCtx.WriteReceivers
		return s.processStageReceivers
	}

	alloyStages := make([]stages.StageConfig, len(s.cfg.PipelineStages))
	for i, ps := range s.cfg.PipelineStages {
		if fs, ok := convertStage(ps, s.diags); ok {
			alloyStages[i] = fs
		}
	}
	args := process.Arguments{
		ForwardTo: s.globalCtx.WriteReceivers,
		Stages:    alloyStages,
	}
	compLabel := common.LabelForParts(s.globalCtx.LabelPrefix, s.cfg.JobName)
	s.f.Body().AppendBlock(common.NewBlockWithOverride([]string{"loki", "process"}, compLabel, args))
	s.processStageReceivers = []loki.LogsReceiver{common.ConvertLogsReceiver{
		Expr: fmt.Sprintf("loki.process.%s.receiver", compLabel),
	}}
	return s.processStageReceivers
}

func (s *ScrapeConfigBuilder) appendDiscoveryRelabel() {
	if s.allRelabeledTargetsExpr != "" {
		return
	}
	if len(s.cfg.RelabelConfigs) == 0 {
		// Skip the discovery.relabel component if there are no relabels needed
		s.allRelabeledTargetsExpr, s.discoveryRelabelRulesExpr = s.getAllTargetsJoinedExpr(), "null"
		return
	}

	relabelConfigs := component.ToAlloyRelabelConfigs(s.cfg.RelabelConfigs)
	args := relabel.Arguments{
		RelabelConfigs: relabelConfigs,
	}

	overrideHook := func(val any) any {
		if _, ok := val.([]discovery.Target); ok {
			return common.CustomTokenizer{Expr: s.getAllTargetsJoinedExpr()}
		}
		return val
	}

	compLabel := common.LabelForParts(s.globalCtx.LabelPrefix, s.cfg.JobName)
	s.f.Body().AppendBlock(common.NewBlockWithOverrideFn(
		[]string{"discovery", "relabel"},
		compLabel,
		args,
		overrideHook,
	))
	compName := fmt.Sprintf("discovery.relabel.%s", compLabel)
	s.allRelabeledTargetsExpr, s.discoveryRelabelRulesExpr = compName+".output", compName+".rules"
}

func (s *ScrapeConfigBuilder) getAllRelabeledTargetsExpr() string {
	s.appendDiscoveryRelabel()
	return s.allRelabeledTargetsExpr
}

func (s *ScrapeConfigBuilder) getOrNewDiscoveryRelabelRules() string {
	s.appendDiscoveryRelabel()
	return s.discoveryRelabelRulesExpr
}

func (s *ScrapeConfigBuilder) getAllTargetsJoinedExpr() string {
	targetsExpr := "[]"
	if len(s.allTargetsExps) == 1 {
		targetsExpr = s.allTargetsExps[0]
	} else if len(s.allTargetsExps) > 1 {
		formatted := make([]string, len(s.allTargetsExps))
		for i, t := range s.allTargetsExps {
			formatted[i] = fmt.Sprintf("\t\t%s,", t)
		}
		targetsExpr = fmt.Sprintf("array.concat(\n%s\n\t)", strings.Join(formatted, "\n"))
	}
	return targetsExpr
}

func convertPromLabels(labels model.LabelSet) map[string]string {
	result := make(map[string]string)
	for k, v := range labels {
		result[string(k)] = string(v)
	}
	return result
}

func convertDecompressionConfig(cfg *scrapeconfig.DecompressionConfig) lokisourcefile.DecompressionConfig {
	if cfg == nil {
		return lokisourcefile.DecompressionConfig{}
	}
	return lokisourcefile.DecompressionConfig{
		Enabled:      cfg.Enabled,
		InitialDelay: cfg.InitialDelay,
		Format:       lokisourcefile.CompressionFormat(cfg.Format),
	}
}

func convertFileWatchConfig(watchConfig *file.WatchConfig) lokisourcefile.FileWatch {
	if watchConfig == nil {
		return lokisourcefile.FileWatch{}
	}
	return lokisourcefile.FileWatch{
		MinPollFrequency: watchConfig.MinPollFrequency,
		MaxPollFrequency: watchConfig.MaxPollFrequency,
	}
}

func convertFileMatchConfig(sync time.Duration) lokisourcefile.FileMatch {
	return lokisourcefile.FileMatch{
		Enabled:    true,
		SyncPeriod: sync,
	}
}

func logsReceiversToExpr(r []loki.LogsReceiver) string {
	var exprs []string
	for _, r := range r {
		clr := r.(common.ConvertLogsReceiver)
		exprs = append(exprs, clr.Expr)
	}
	return "[" + strings.Join(exprs, ", ") + "]"
}

func toAlloyExpression(goValue any) (string, error) {
	e := builder.NewExpr()
	e.SetValue(goValue)
	var buff bytes.Buffer
	_, err := e.WriteTo(&buff)
	if err != nil {
		return "", err
	}
	return buff.String(), nil
}
