package build

import (
	"reflect"

	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/flow/logging"
	"github.com/grafana/agent/internal/static/server"
)

func (b *ConfigBuilder) appendLogging(config *server.Config) {
	args := toLogging(config)
	if !reflect.DeepEqual(*args, logging.DefaultOptions) {
		b.f.Body().AppendBlock(common.NewBlockWithOverride(
			[]string{"logging"},
			"",
			args,
		))
	}
}

func toLogging(config *server.Config) *logging.Options {
	return &logging.Options{
		Level:  logging.Level(config.LogLevel.String()),
		Format: logging.Format(config.LogFormat),
	}
}
