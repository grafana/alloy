package build

import (
	"github.com/grafana/agent/internal/component/common/relabel"
	"github.com/grafana/agent/internal/component/loki/source/kafka"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/loki/clients/pkg/promtail/scrapeconfig"
	"github.com/grafana/river/rivertypes"
)

func (s *ScrapeConfigBuilder) AppendKafka() {
	if s.cfg.KafkaConfig == nil {
		return
	}
	kafkaCfg := s.cfg.KafkaConfig
	args := kafka.Arguments{
		Brokers:              kafkaCfg.Brokers,
		Topics:               kafkaCfg.Topics,
		GroupID:              kafkaCfg.GroupID,
		Assignor:             kafkaCfg.Assignor,
		Version:              kafkaCfg.Version,
		Authentication:       convertKafkaAuthConfig(kafkaCfg),
		UseIncomingTimestamp: kafkaCfg.UseIncomingTimestamp,
		Labels:               convertPromLabels(kafkaCfg.Labels),
		ForwardTo:            s.getOrNewProcessStageReceivers(),
		RelabelRules:         relabel.Rules{},
	}
	override := func(val interface{}) interface{} {
		switch value := val.(type) {
		case relabel.Rules:
			return common.CustomTokenizer{Expr: s.getOrNewDiscoveryRelabelRules()}
		case rivertypes.Secret:
			return string(value)
		default:
			return val
		}
	}
	compLabel := common.LabelForParts(s.globalCtx.LabelPrefix, s.cfg.JobName)
	s.f.Body().AppendBlock(common.NewBlockWithOverrideFn(
		[]string{"loki", "source", "kafka"},
		compLabel,
		args,
		override,
	))
}

func convertKafkaAuthConfig(kafkaCfg *scrapeconfig.KafkaTargetConfig) kafka.KafkaAuthentication {
	return kafka.KafkaAuthentication{
		Type:      string(kafkaCfg.Authentication.Type),
		TLSConfig: *common.ToTLSConfig(&kafkaCfg.Authentication.TLSConfig),
		SASLConfig: kafka.KafkaSASLConfig{
			Mechanism: string(kafkaCfg.Authentication.SASLConfig.Mechanism),
			User:      kafkaCfg.Authentication.SASLConfig.User,
			Password:  rivertypes.Secret(kafkaCfg.Authentication.SASLConfig.Password.String()),
			UseTLS:    kafkaCfg.Authentication.SASLConfig.UseTLS,
			TLSConfig: *common.ToTLSConfig(&kafkaCfg.Authentication.SASLConfig.TLSConfig),
		},
	}
}
