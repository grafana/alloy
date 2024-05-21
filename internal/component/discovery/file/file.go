package file

import (
	"time"

	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/file"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.file",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Files           []string      `alloy:"files,attr"`
	RefreshInterval time.Duration `alloy:"refresh_interval,attr,optional"`
}

var DefaultArguments = Arguments{
	RefreshInterval: 5 * time.Minute,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) Convert() *prom_discovery.SDConfig {
	return &prom_discovery.SDConfig{
		Files:           a.Files,
		RefreshInterval: model.Duration(a.RefreshInterval),
	}
}

func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.DiscovererConfig, error) {
		newArgs := args.(Arguments)
		return newArgs.Convert(), nil
	})
}
