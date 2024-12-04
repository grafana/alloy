package main

import (
	"encoding/json"
	"time"

	"github.com/extism/go-pdk"
	"github.com/prometheus/prometheus/model/labels"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type config struct {
	Metrics []metricLabels `json:"Metrics,omitempty"`
}

// The metric name is also a label.
// We don't need to handle it in a special way.
type metricLabels = map[string]string

type metric struct {
	labels      labels.Labels
	fingerprint uint64
}

type metrics struct {
	data []metric
}

func (m *metrics) find(fingerprint uint64) bool {
	for _, metric := range m.data {
		//TODO: Also compare the label values in case of hash collision.
		if metric.fingerprint == fingerprint {
			return true
		}
	}
	return false
}

func internalLabelsFingerprint(internalLabels []*Label) uint64 {
	lblSlice := []string{}
	for _, lbl := range internalLabels {
		lblSlice = append(lblSlice, lbl.Name, lbl.Value)
	}
	promLabels := labels.FromStrings(lblSlice...)
	return promLabels.Hash()
}

func createOTelLog(metric *PrometheusMetric) ([]byte, error) {
	logMsg := "Prometheus metric: { "
	for _, lbl := range metric.Labels {
		logMsg += lbl.Name + " = " + lbl.Value + "; "
	}
	logMsg += " }"

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	il := rl.ScopeLogs().AppendEmpty()
	lg := il.LogRecords().AppendEmpty()
	lg.SetSeverityText("Info")
	lg.SetDroppedAttributesCount(1)
	lg.Body().SetStr(logMsg)
	lg.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	encoder := &plog.JSONMarshaler{}
	return encoder.MarshalLogs(ld)
}

//export process
func process() int32 {
	in := pdk.Input()
	pt, err := parsePassthrough(in)
	if err != nil {
		pdk.SetError(err)
		return 1
	}
	var cfg config
	err = json.Unmarshal(pdk.Input(), &cfg)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	if len(cfg.Metrics) == 0 {
		pdk.Output(in)
		return 0
	}

	// Hash each label so that we don't have to compare
	// all the label strings all the time.
	metrics := metrics{}
	for _, lbls := range cfg.Metrics {
		promLbls := labels.FromMap(lbls)
		metrics.data = append(metrics.data, metric{
			labels:      promLbls,
			fingerprint: promLbls.Hash(),
		})
	}

	outPT := &Passthrough{
		Logs: pt.Logs,
	}
	for _, metric := range pt.Prommetrics {
		fingerprint := internalLabelsFingerprint(metric.Labels)
		found := metrics.find(fingerprint)
		if found {
			log, err := createOTelLog(metric)
			if err != nil {
				pdk.SetError(err)
				return 1
			}

			outPT.Logs = append(outPT.Logs, log)
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
