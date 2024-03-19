package build

import (
	"strings"

	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/static/config"
	"github.com/grafana/river/token/builder"
)

type ConfigBuilder struct {
	f         *builder.File
	diags     *diag.Diagnostics
	cfg       *config.Config
	globalCtx *GlobalContext
}

func NewConfigBuilder(f *builder.File, diags *diag.Diagnostics, cfg *config.Config, globalCtx *GlobalContext) *ConfigBuilder {
	return &ConfigBuilder{
		f:         f,
		diags:     diags,
		cfg:       cfg,
		globalCtx: globalCtx,
	}
}

func (b *ConfigBuilder) Build() {
	b.appendLogging(b.cfg.Server)
	b.appendServer(b.cfg.Server)
	b.appendIntegrations()
	b.appendTraces()
}

func splitByCommaNullOnEmpty(s string) []string {
	if s == "" {
		return nil
	}

	return strings.Split(s, ",")
}
