package scrape

import "github.com/grafana/alloy/internal/component"

var name = "prometheus.scrape"

func init() {
	component.Register(component.Registration{Name: name}) // want `component name must be a constant string`
}
