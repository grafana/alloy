//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"
)

// List of expected Windows metrics
var winMetrics = []string{
	"windows_cpu_time_total",           // cpu
	"windows_cs_logical_processors",    // cs
	"windows_logical_disk_info",        // logical_disk
	"windows_net_bytes_received_total", // net
	"windows_os_info",                  // os
	"windows_service_info",             // service
	"windows_system_system_up_time",    // system
}

// TestWindowsMetrics sets up a server to receive remote write requests
// and checks if required metrics appear within a one minute timeout
func TestWindowsMetrics(t *testing.T) {
	foundMetrics := make(map[string]bool)
	for _, metric := range winMetrics {
		foundMetrics[metric] = false
	}

	done := make(chan bool)
	srv := &http.Server{Addr: ":9090"}
	http.HandleFunc("/receive", func(w http.ResponseWriter, r *http.Request) {
		ts, _, err := handlePost(t, w, r)

		if err != nil {
			t.Log("Cancel processing request.")
			return
		}

		for _, timeseries := range ts {
			var metricName string
			for _, label := range timeseries.Labels {
				if label.Name == "__name__" {
					metricName = label.Value
					break
				}
			}
			for _, requiredMetric := range winMetrics {
				if requiredMetric == metricName && !foundMetrics[requiredMetric] {
					foundMetrics[requiredMetric] = true
				}
			}
		}

		allFound := true
		for _, found := range foundMetrics {
			if !found {
				allFound = false
				break
			}
		}

		if allFound {
			done <- true
		}
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(fmt.Errorf("could not start server: %v", err))
		}
	}()
	defer srv.Shutdown(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	select {
	case <-ctx.Done():
		missingMetrics := []string{}
		for metric, found := range foundMetrics {
			if !found {
				missingMetrics = append(missingMetrics, metric)
			}
		}
		if len(missingMetrics) > 0 {
			t.Errorf("Timeout reached. Missing metrics: %v", missingMetrics)
		} else {
			t.Log("All required metrics received.")
		}
	case <-done:
		t.Log("All required metrics received within the timeout.")
	}
}

func handlePost(t *testing.T, _ http.ResponseWriter, r *http.Request) ([]prompb.TimeSeries, []prompb.MetricMetadata, error) {
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)

	// ignore this error because the server might shutdown while a request is being processed
	if opErr, ok := err.(*net.OpError); ok && strings.Contains(opErr.Err.Error(), "use of closed network connection") {
		return nil, nil, err
	}

	require.NoError(t, err)

	data, err = snappy.Decode(nil, data)
	require.NoError(t, err)

	var req prompb.WriteRequest
	err = req.Unmarshal(data)
	require.NoError(t, err)
	return req.GetTimeseries(), req.Metadata, nil
}
