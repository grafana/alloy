package build

import (
	"fmt"
	"time"

	alloyrelabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/journal"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
)

func (s *ScrapeConfigBuilder) AppendJournalConfig() {
	jc := s.cfg.JournalConfig
	if jc == nil {
		return
	}
	args := journal.Arguments{}
	args.SetToDefault()

	if len(jc.MaxAge) > 0 {
		parsedAge, err := time.ParseDuration(jc.MaxAge)
		if err != nil {
			s.diags.Add(
				diag.SeverityLevelError,
				fmt.Sprintf("failed to parse max_age duration for journal config: %s, will use default", err),
			)
		} else {
			args.MaxAge = parsedAge
		}
	}

	args.FormatAsJson = jc.JSON
	args.Path = jc.Path
	args.ForwardTo = s.getOrNewProcessStageReceivers()
	args.Labels = convertPromLabels(jc.Labels)
	args.RelabelRules = alloyrelabel.Rules{}

	relabelRulesExpr := s.getOrNewDiscoveryRelabelRules()
	hook := func(val any) any {
		if _, ok := val.(alloyrelabel.Rules); ok {
			return common.CustomTokenizer{Expr: relabelRulesExpr}
		}
		return val
	}
	compLabel := common.LabelForParts(s.globalCtx.LabelPrefix, s.cfg.JobName)
	s.f.Body().AppendBlock(common.NewBlockWithOverrideFn(
		[]string{"loki", "source", "journal"},
		compLabel,
		args,
		hook,
	))
}
