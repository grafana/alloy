package save_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/telemetry/save"
)

// decodeSample decodes a single sample based on its record type
func decodeSample(rawMessage json.RawMessage) (any, error) {
	var base save.Sample
	if err := json.Unmarshal(rawMessage, &base); err != nil {
		return nil, err
	}

	switch base.RecordType {
	case save.RecordTypeSample:
		var sample save.ValueSample
		if err := json.Unmarshal(rawMessage, &sample); err != nil {
			return nil, err
		}
		return sample, nil
	case save.RecordTypeExemplar:
		var exemplarSample save.ExemplarSample
		if err := json.Unmarshal(rawMessage, &exemplarSample); err != nil {
			return nil, err
		}
		return exemplarSample, nil
	case save.RecordTypeHistogram:
		var histogramSample save.HistogramSample
		if err := json.Unmarshal(rawMessage, &histogramSample); err != nil {
			return nil, err
		}
		return histogramSample, nil
	default:
		return nil, fmt.Errorf("unknown record type: %s", base.RecordType)
	}
}

// decodeSamples decodes different sample types from JSON messages
func decodeSamples(data []byte, samples []any) ([]any, error) {
	var rawMessages []json.RawMessage
	if err := json.Unmarshal(data, &rawMessages); err != nil {
		return nil, err
	}

	samples = samples[:0]
	for _, rawMessage := range rawMessages {
		sample, err := decodeSample(rawMessage)
		if err != nil {
			return nil, err
		}
		samples = append(samples, sample)
	}

	return samples, nil
}

func samplesToWriteRequest(samples []any) (*prompb.WriteRequest, error) {
	var timeSeries []prompb.TimeSeries

	for _, sample := range samples {
		switch s := sample.(type) {
		case save.ValueSample:
			timeSeries = append(timeSeries, prompb.TimeSeries{
				Labels: convertLabels(s.Labels),
				Samples: []prompb.Sample{
					{
						Timestamp: s.Timestamp,
						Value:     s.Value,
					},
				},
			})
		case save.ExemplarSample:
			if s.Exemplar != nil {
				// Extract exemplar labels from the exemplar struct
				exemplarLabels := make(map[string]string)
				if !s.Exemplar.Labels.IsEmpty() {
					exemplarLabels = s.Exemplar.Labels.Map()
				}

				timeSeries = append(timeSeries, prompb.TimeSeries{
					Labels: convertLabels(s.Labels),
					Exemplars: []prompb.Exemplar{
						{
							Timestamp: s.Exemplar.Ts,
							Value:     s.Exemplar.Value,
							Labels:    convertLabels(exemplarLabels),
						},
					},
				})
			}
		case save.HistogramSample:
			var histogramPb prompb.Histogram
			histogramPb.Timestamp = s.Timestamp

			// Handle native histogram
			if s.Histogram != nil {
				histogramPb.Count = &prompb.Histogram_CountInt{CountInt: s.Histogram.Count}
				histogramPb.Sum = s.Histogram.Sum
				histogramPb.Schema = s.Histogram.Schema
				histogramPb.ZeroThreshold = s.Histogram.ZeroThreshold
				histogramPb.ZeroCount = &prompb.Histogram_ZeroCountInt{ZeroCountInt: s.Histogram.ZeroCount}
				histogramPb.NegativeSpans = convertSpans(s.Histogram.NegativeSpans)
				histogramPb.PositiveSpans = convertSpans(s.Histogram.PositiveSpans)
				histogramPb.NegativeDeltas = s.Histogram.NegativeBuckets
				histogramPb.PositiveDeltas = s.Histogram.PositiveBuckets
			}

			// Handle float histogram
			if s.FloatHistogram != nil {
				histogramPb.Count = &prompb.Histogram_CountFloat{CountFloat: s.FloatHistogram.Count}
				histogramPb.Sum = s.FloatHistogram.Sum
				histogramPb.Schema = s.FloatHistogram.Schema
				histogramPb.ZeroThreshold = s.FloatHistogram.ZeroThreshold
				histogramPb.ZeroCount = &prompb.Histogram_ZeroCountFloat{ZeroCountFloat: s.FloatHistogram.ZeroCount}
				histogramPb.NegativeSpans = convertSpans(s.FloatHistogram.NegativeSpans)
				histogramPb.PositiveSpans = convertSpans(s.FloatHistogram.PositiveSpans)
				// Convert float buckets to int64 deltas (this is a simplification)
				histogramPb.NegativeDeltas = make([]int64, len(s.FloatHistogram.NegativeBuckets))
				for i, v := range s.FloatHistogram.NegativeBuckets {
					histogramPb.NegativeDeltas[i] = int64(v)
				}
				histogramPb.PositiveDeltas = make([]int64, len(s.FloatHistogram.PositiveBuckets))
				for i, v := range s.FloatHistogram.PositiveBuckets {
					histogramPb.PositiveDeltas[i] = int64(v)
				}
			}

			timeSeries = append(timeSeries, prompb.TimeSeries{
				Labels:     convertLabels(s.Labels),
				Histograms: []prompb.Histogram{histogramPb},
			})
		default:
			return nil, fmt.Errorf("unsupported sample type: %T", s)
		}
	}

	return &prompb.WriteRequest{
		Timeseries: timeSeries,
	}, nil
}

func convertSpans(spans []histogram.Span) []prompb.BucketSpan {
	result := make([]prompb.BucketSpan, len(spans))
	for i, span := range spans {
		result[i] = prompb.BucketSpan{
			Offset: span.Offset,
			Length: span.Length,
		}
	}
	return result
}

func convertLabels(labels map[string]string) []prompb.Label {
	var result []prompb.Label
	for k, v := range labels {
		result = append(result, prompb.Label{
			Name:  k,
			Value: v,
		})
	}
	return result
}

func TestSendMetrics(t *testing.T) {
	metricsFilePath := filepath.Join("..", "..", "..", "..", "telemetry", "save", "prometheus", "metrics.json")

	file, err := os.Open(metricsFilePath)
	if err != nil {
		t.Skipf("Skipping test, metrics file not found: %v", err)
		return
	}
	defer func() { _ = file.Close() }()

	var samples []any
	decoder := json.NewDecoder(file)

	for {
		var rawMessage json.RawMessage
		if err := decoder.Decode(&rawMessage); err == io.EOF {
			break // End of file
		}
		require.NoError(t, err)

		samples, err = decodeSamples(rawMessage, samples)
		require.NoError(t, err)

		if len(samples) > 0 {
			writeRequest, err := samplesToWriteRequest(samples)
			require.NoError(t, err)

			t.Logf("Converted %d time series to remote write request", len(writeRequest.Timeseries))

			err = sendWriteRequest(writeRequest, "http://localhost:9009/api/v1/push")
			if err != nil {
				t.Logf("Failed to send WriteRequest (this is expected if the endpoint is not running): %v", err)
			} else {
				t.Logf("Successfully sent WriteRequest to http://localhost:9009/api/v1/push")
			}
		}
	}
}

// sendWriteRequest encodes a WriteRequest with snappy compression and sends it to the given endpoint
func sendWriteRequest(writeRequest *prompb.WriteRequest, endpoint string) error {
	// Encode the WriteRequest to Protobuf binary format
	data, err := writeRequest.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal WriteRequest: %w", err)
	}

	// Compress the encoded request using Snappy
	compressed := snappy.Encode(nil, data)

	// Create HTTP request
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set required headers for Prometheus remote write
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
