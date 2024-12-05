package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/extism/go-pdk"
	opentelemetry_proto_common_v1 "github.com/grafana/alloy/internal/component/compute/process/examples/go/lib/otlp/common/v1"
	opentelemetry_proto_logs_v1 "github.com/grafana/alloy/internal/component/compute/process/examples/go/lib/otlp/logs/v1"
	"github.com/prometheus/prometheus/model/labels"
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

	log := opentelemetry_proto_logs_v1.LogsData{
		ResourceLogs: []*opentelemetry_proto_logs_v1.ResourceLogs{
			{
				ScopeLogs: []*opentelemetry_proto_logs_v1.ScopeLogs{
					{
						LogRecords: []*opentelemetry_proto_logs_v1.LogRecord{
							{
								SeverityText: "Info",
								Body: &opentelemetry_proto_common_v1.AnyValue{
									Value: &opentelemetry_proto_common_v1.AnyValue_BytesValue{
										BytesValue: []byte(logMsg),
									},
								},
								TimeUnixNano: uint64(time.Now().UnixNano()),
							},
						},
					},
				},
			},
		},
	}

	return log.MarshalVT()
}

func createLokiLog(metric *PrometheusMetric) *LokiLog {
	logMsg := "Prometheus metric: { "
	for _, lbl := range metric.Labels {
		logMsg += lbl.Name + " = " + lbl.Value + "; "
	}
	logMsg += " }"

	return &LokiLog{
		Timestamp: time.Now().Unix(),
		Line:      logMsg,
	}
}

func setError(err error) {
	if err != nil {
		pdk.SetError(fmt.Errorf("error in log_metric_sample: %v", err))
	}
}

//export process
func process() int32 {
	in := pdk.Input()
	pt, err := parsePassthrough(in)
	if err != nil {
		setError(fmt.Errorf("failed to parse input: %v", err))
		return 1
	}
	var cfg config
	err = json.Unmarshal(pdk.Input(), &cfg)
	if err != nil {
		setError(fmt.Errorf("failed to unmarshal WASM config: %v; full config: %s", err, pdk.Input()))
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

	outPT := &Passthrough{}
	for _, metric := range pt.Prommetrics {
		fingerprint := internalLabelsFingerprint(metric.Labels)
		found := metrics.find(fingerprint)
		if found {
			log := createLokiLog(metric)
			if err != nil {
				setError(err)
				return 1
			}

			outPT.Lokilogs = append(outPT.Lokilogs, log)
		}
	}
	bb, err := outPT.MarshalVT()
	if err != nil {
		setError(fmt.Errorf("failed to marshal WASM output: %v", err))
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
