package build

import (
	"reflect"

	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/server"
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
	opts := &logging.Options{}
	opts.SetToDefault()
	opts.Level = logging.Level(config.LogLevel.String())
	opts.Format = logging.Format(config.LogFormat)
	return opts
}
