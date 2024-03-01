package common

import (
	"encoding/json"
	"fmt"
)

type MetricsResponse struct {
	Status string   `json:"status"`
	Data   []Metric `json:"data"`
}

type MetricResponse struct {
	Status string     `json:"status"`
	Data   MetricData `json:"data"`
}

type MetricData struct {
	ResultType string         `json:"resultType"`
	Result     []MetricResult `json:"result"`
}

type HistogramRawData struct {
	Timestamp int64
	Data      HistogramData
}

type Bucket struct {
	BoundaryRule  float64
	LeftBoundary  string
	RightBoundary string
	CountInBucket string
}

type HistogramData struct {
	Count   string   `json:"count"`
	Sum     string   `json:"sum"`
	Buckets []Bucket `json:"buckets"`
}

type MetricResult struct {
	Metric    Metric            `json:"metric"`
	Value     *Value            `json:"value,omitempty"`
	Histogram *HistogramRawData `json:"histogram,omitempty"`
}

type Value struct {
	Timestamp int64
	Value     string
}

type Metric struct {
	TestName string `json:"test_name"`
	Name     string `json:"__name__"`
}

func (m *MetricResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *MetricsResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (h *HistogramRawData) UnmarshalJSON(b []byte) error {
	var arr []json.RawMessage
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	if len(arr) != 2 {
		return fmt.Errorf("expected 2 values in histogram raw data, got %d", len(arr))
	}

	if err := json.Unmarshal(arr[0], &h.Timestamp); err != nil {
		return err
	}
	return json.Unmarshal(arr[1], &h.Data)
}

func (b *Bucket) UnmarshalJSON(data []byte) error {
	var raw []interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) != 4 {
		return fmt.Errorf("expected 4 values for bucket, got %d", len(raw))
	}

	if v, ok := raw[0].(float64); ok {
		b.BoundaryRule = v
	} else {
		return fmt.Errorf("expected float64 for BoundaryRule, got %T", raw[0])
	}

	b.LeftBoundary, _ = raw[1].(string)
	b.RightBoundary, _ = raw[2].(string)
	b.CountInBucket, _ = raw[3].(string)

	return nil
}

func (v *Value) UnmarshalJSON(b []byte) error {
	var arr []interface{}
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	if len(arr) != 2 {
		return fmt.Errorf("expected 2 values, got %d", len(arr))
	}
	v.Timestamp, _ = arr[0].(int64)
	v.Value, _ = arr[1].(string)
	return nil
}
