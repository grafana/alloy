package build

import (
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/cloudflare"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func (s *ScrapeConfigBuilder) AppendCloudFlareConfig() {
	if s.cfg.CloudflareConfig == nil {
		return
	}

	args := cloudflare.Arguments{
		APIToken:   alloytypes.Secret(s.cfg.CloudflareConfig.APIToken),
		ZoneID:     s.cfg.CloudflareConfig.ZoneID,
		Labels:     convertPromLabels(s.cfg.CloudflareConfig.Labels),
		Workers:    s.cfg.CloudflareConfig.Workers,
		PullRange:  time.Duration(s.cfg.CloudflareConfig.PullRange),
		FieldsType: cloudflare.FieldsType(s.cfg.CloudflareConfig.FieldsType),
	}
	override := func(val any) any {
		switch conv := val.(type) {
		case []loki.LogsReceiver:
			return common.CustomTokenizer{Expr: s.getOrNewLokiRelabel()}
		case alloytypes.Secret:
			return string(conv)
		default:
			return val
		}
	}
	compLabel := common.LabelForParts(s.globalCtx.LabelPrefix, s.cfg.JobName)
	s.f.Body().AppendBlock(common.NewBlockWithOverrideFn(
		[]string{"loki", "source", "cloudflare"},
		compLabel,
		args,
		override,
	))
}
