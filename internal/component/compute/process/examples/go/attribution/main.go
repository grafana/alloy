package main

import (
	"github.com/extism/go-pdk"
	"time"
)

//export process
func process() int32 {
	in := pdk.Input()
	pt, err := parsePassthrough(in)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	outPT := &Passthrough{
		Prommetrics: make([]*PrometheusMetric, 0),
		Metrics:     pt.Metrics,
		Logs:        pt.Logs,
		Traces:      pt.Traces,
	}
	attributions := make(map[string]int)

	// Gather our attributions
	for _, metric := range pt.Prommetrics {
		var found bool
		for _, lbl := range metric.Labels {
			if lbl.Name == "service" {
				attributions[lbl.Value]++
				found = true
				break
			}
		}
		if !found {
			attributions["unknown"]++
		}
		outPT.Prommetrics = append(outPT.Prommetrics, metric)
	}
	for k, v := range attributions {
		m := &PrometheusMetric{
			Labels: []*Label{
				&Label{
					Name:  "service",
					Value: k,
				},
				&Label{
					Name:  "__name__",
					Value: "service_attribution",
				},
			},
			Value:       float64(v),
			Timestampms: time.Now().UnixMilli(),
		}
		outPT.Prommetrics = append(outPT.Prommetrics, m)
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
