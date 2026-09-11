package gather

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"gopkg.in/yaml.v3"
)

// Config writes the collector's running configuration to the bundle. It owns
// the config snapshot state. The collector sends snapshots through the
// extension, which forwards them to Store.
//
// The gatherer writes only the unexpanded configuration to the bundle. The
// unexpanded form has sensitive fields redacted, so the bundle does not leak
// secrets. It also keeps the effective (expanded) configuration in memory for
// internal use only, such as resolving the telemetry metrics endpoint. It never
// writes the effective form to the bundle.
type Config struct {
	mu         sync.Mutex
	unexpanded *confmap.Conf
	effective  *confmap.Conf
}

func (*Config) Name() string { return "config" }

// Store keeps the snapshot's configurations.
func (g *Config) Store(snapshot extensioncapabilities.ConfigSnapshot) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.unexpanded = snapshot.Unexpanded()
	g.effective = snapshot.Effective()
}

// EffectiveConf returns the effective (expanded) config, for internal use only.
// It holds secrets, so callers must never write it to the bundle.
func (g *Config) EffectiveConf() *confmap.Conf {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.effective
}

func (g *Config) Gather(_ context.Context, _ Options) ([]File, error) {
	g.mu.Lock()
	conf := g.unexpanded
	g.mu.Unlock()

	if conf == nil {
		// The collector has not sent a config snapshot yet.
		return nil, nil
	}

	data, err := yaml.Marshal(conf.ToStringMap())
	if err != nil {
		return nil, err
	}
	return []File{{Path: "config.yaml", Content: data}}, nil
}
