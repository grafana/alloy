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
	//TODO(thampiotr): this default value should be imported from promtail once it's made public there.
	var maxAge = time.Hour * 7 // use default value
	if len(jc.MaxAge) > 0 {
		parsedAge, err := time.ParseDuration(jc.MaxAge)
		if err != nil {
			s.diags.Add(
				diag.SeverityLevelError,
				fmt.Sprintf("failed to parse max_age duration for journal config: %s, will use default", err),
			)
		} else {
			maxAge = parsedAge
		}
	}
	args := journal.Arguments{
		FormatAsJson: jc.JSON,
		MaxAge:       maxAge,
		Path:         jc.Path,
		ForwardTo:    s.getOrNewProcessStageReceivers(),
		Labels:       convertPromLabels(jc.Labels),
		RelabelRules: alloyrelabel.Rules{},
	}
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
