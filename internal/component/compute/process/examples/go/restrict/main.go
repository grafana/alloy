package main

import (
	"github.com/extism/go-pdk"
	"slices"
	"strings"
)

//export process
func process() int32 {
	in := pdk.Input()
	pt, err := parsePassthrough(in)
	if err != nil {
		pdk.SetError(err)
		return 1
	}
	allowed := pt.Config["allowed_services"]
	allowedServices := strings.Split(allowed, ",")
	// If allowed is empty then allow all.
	if len(allowedServices) == 0 {
		pdk.Output(in)
		return 0
	}
	// Could practice to pass along things unchanged  unless
	// you explicitly don't want them to.
	outPT := &Passthrough{
		Prommetrics: make([]*PrometheusMetric, 0),
		Metrics:     pt.Metrics,
		Logs:        pt.Logs,
		Traces:      pt.Traces,
	}
	for _, metric := range pt.Prommetrics {
		// Check for the service label
		var serviceValue string
		for _, lbl := range metric.Labels {
			if lbl.Name == "service" {
				serviceValue = lbl.Value
				break
			}
		}

		if serviceValue == "" {
			continue
		}
		if slices.Contains(allowedServices, serviceValue) {
			outPT.Prommetrics = append(outPT.Prommetrics, metric)
		}
	}
	bb, err := outPT.MarshalVT()
	if err != nil {
		pdk.SetError(err)
		return 1
	}
	pdk.Output(bb)
	return 0
}

func parsePassthrough(bb []byte) (*Passthrough, error) {
	pt := &Passthrough{}
	err := pt.UnmarshalVT(bb)
	return pt, err
}

// this has to exist to compile with tinygo
func main() {}
