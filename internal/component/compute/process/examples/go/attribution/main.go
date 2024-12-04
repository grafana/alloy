package main

import (
	"encoding/json"
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

	state := serviceCounter{
		Counter: make(map[string]int),
	}
	stateBB := pdk.GetVar("state")

	if len(stateBB) > 0 {
		unmarshalErr := json.Unmarshal(stateBB, &state)
		if unmarshalErr != nil {
			pdk.SetError(err)
			return 1
		}
	}
	defer func() {
		stateBB, marshalErr := json.Marshal(state)
		if marshalErr != nil {
			pdk.SetError(err)
		}
		pdk.SetVar("state", stateBB)
	}()
	outPT := &Passthrough{
		Prommetrics: pt.Prommetrics,
		Metrics:     pt.Metrics,
		Logs:        pt.Logs,
		Traces:      pt.Traces,
		Lokilogs:    pt.Lokilogs,
	}
	if outPT.Lokilogs == nil {
		outPT.Lokilogs = make([]*LokiLog, 0)
	}

	unknownJobs := make(map[string]struct{})
	// Gather our attributions
	for _, metric := range pt.Prommetrics {
		var found bool
		var job string
		for _, lbl := range metric.Labels {
			if lbl.Name == "service" {
				state.Counter[lbl.Value]++
				found = true
				break
			}
			if lbl.Name == "job" {
				job = lbl.Value
				unknownJobs[lbl.Value] = struct{}{}
			}
		}
		if !found {
			state.Counter["unknown"]++
		} else {
			// If we found it delete from unknown jobs.
			delete(unknownJobs, job)
		}
	}
	// Create a log entry for each unknown job that does not have a service tag. This will make things easier to track.
	for k, _ := range unknownJobs {
		ll := &LokiLog{
			Labels: []*Label{
				&Label{
					Name:  "job",
					Value: k,
				},
			},
			Timestamp: time.Now().Unix(),
			Line:      "unknown job found " + k,
		}
		outPT.Lokilogs = append(outPT.Lokilogs, ll)
	}
	for k, v := range state.Counter {
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

type serviceCounter struct {
	Counter map[string]int
}

// this has to exist to compile with tinygo
func main() {}
