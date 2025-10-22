// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal_test

//TODO: Uncomment this test later. For now it's commented out because it depends on this package:
// "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
// ... which causes compilation issues with the Agent due to the fact that in Prometheus
// textparse.MetricType changed to model.MetricType:
// https://github.com/prometheus/prometheus/blob/12e317786b7ac864117f4be1a88a1aa29e5dcf9e/scrape/target.go#L89
//
// See:
// https://github.com/prometheus/prometheus/commit/8065bef172e8d88e22399504b175a8c9115e9da3
// https://github.com/prometheus/prometheus/commit/c83e1fc5748be3bd35bf0a31eb53690b412846a4

// Test that staleness markers are emitted for timeseries that intermittently disappear.
// This test runs the entire collector and end-to-end scrapes then checks with the
// Prometheus remotewrite exporter that staleness markers are emitted per timeseries.
// See https://github.com/open-telemetry/opentelemetry-collector/issues/3413
// func TestStalenessMarkersEndToEnd(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("This test can take a long time")
// 	}

// 	ctx, cancel := context.WithCancel(t.Context())

// 	// 1. Setup the server that sends series that intermittently appear and disappear.
// 	n := &atomic.Uint64{}
// 	scrapeServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
// 		// Increment the scrape count atomically per scrape.
// 		i := n.Add(1)

// 		select {
// 		case <-ctx.Done():
// 			return
// 		default:
// 		}

// 		// Alternate metrics per scrape so that every one of
// 		// them will be reported as stale.
// 		if i%2 == 0 {
// 			fmt.Fprintf(rw, `
// # HELP jvm_memory_bytes_used Used bytes of a given JVM memory area.
// # TYPE jvm_memory_bytes_used gauge
// jvm_memory_bytes_used{area="heap"} %.1f`, float64(i))
// 		} else {
// 			fmt.Fprintf(rw, `
// # HELP jvm_memory_pool_bytes_used Used bytes of a given JVM memory pool.
// # TYPE jvm_memory_pool_bytes_used gauge
// jvm_memory_pool_bytes_used{pool="CodeHeap 'non-nmethods'"} %.1f`, float64(i))
// 		}
// 	}))
// 	defer scrapeServer.Close()

// 	serverURL, err := url.Parse(scrapeServer.URL)
// 	require.NoError(t, err)

// 	// 2. Set up the Prometheus RemoteWrite endpoint.
// 	prweUploads := make(chan *prompb.WriteRequest)
// 	prweServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
// 		// Snappy decode the uploads.
// 		payload, rerr := io.ReadAll(req.Body)
// 		require.NoError(t, rerr)

// 		recv := make([]byte, len(payload))
// 		decoded, derr := snappy.Decode(recv, payload)
// 		require.NoError(t, derr)

// 		writeReq := new(prompb.WriteRequest)
// 		require.NoError(t, proto.Unmarshal(decoded, writeReq))

// 		select {
// 		case <-ctx.Done():
// 			return
// 		case prweUploads <- writeReq:
// 		}
// 	}))
// 	defer prweServer.Close()

// 	// 3. Set the OpenTelemetry Prometheus receiver.
// 	cfg := fmt.Sprintf(`
// receivers:
//   prometheus:
//     config:
//       scrape_configs:
//         - job_name: 'test'
//           scrape_interval: 100ms
//           static_configs:
//             - targets: [%q]

// processors:
//   batch:
// exporters:
//   prometheusremotewrite:
//     endpoint: %q
//     tls:
//       insecure: true

// service:
//   pipelines:
//     metrics:
//       receivers: [prometheus]
//       processors: [batch]
//       exporters: [prometheusremotewrite]`, serverURL.Host, prweServer.URL)

// 	confFile, err := os.CreateTemp(os.TempDir(), "conf-")
// 	require.NoError(t, err)
// 	defer os.Remove(confFile.Name())
// 	_, err = confFile.Write([]byte(cfg))
// 	require.NoError(t, err)
// 	// 4. Run the OpenTelemetry Collector.
// 	receivers, err := receiver.MakeFactoryMap(prometheusreceiver.NewFactory())
// 	require.NoError(t, err)
// 	exporters, err := exporter.MakeFactoryMap(prometheusremotewriteexporter.NewFactory())
// 	require.NoError(t, err)
// 	processors, err := processor.MakeFactoryMap(batchprocessor.NewFactory())
// 	require.NoError(t, err)

// 	factories := otelcol.Factories{
// 		Receivers:  receivers,
// 		Exporters:  exporters,
// 		Processors: processors,
// 	}

// 	fmp := fileprovider.NewFactory().Create(confmap.ProviderSettings{})
// 	configProvider, err := otelcol.NewConfigProvider(
// 		otelcol.ConfigProviderSettings{
// 			ResolverSettings: confmap.ResolverSettings{
// 				URIs:      []string{confFile.Name()},
// 				Providers: map[string]confmap.Provider{fmp.Scheme(): fmp},
// 			},
// 		})
// 	require.NoError(t, err)

// 	appSettings := otelcol.CollectorSettings{
// 		Factories:      func() (otelcol.Factories, error) { return factories, nil },
// 		ConfigProvider: configProvider,
// 		BuildInfo: component.BuildInfo{
// 			Command:     "otelcol",
// 			Description: "OpenTelemetry Collector",
// 			Version:     "tests",
// 		},
// 		LoggingOptions: []zap.Option{
// 			// Turn off the verbose logging from the collector.
// 			zap.WrapCore(func(zapcore.Core) zapcore.Core {
// 				return zapcore.NewNopCore()
// 			}),
// 		},
// 	}

// 	app, err := otelcol.NewCollector(appSettings)
// 	require.NoError(t, err)

// 	go func() {
// 		assert.NoError(t, app.Run(t.Context()))
// 	}()
// 	defer app.Shutdown()

// 	// Wait until the collector has actually started.
// 	for notYetStarted := true; notYetStarted; {
// 		state := app.GetState()
// 		switch state {
// 		case otelcol.StateRunning, otelcol.StateClosed, otelcol.StateClosing:
// 			notYetStarted = false
// 		case otelcol.StateStarting:
// 		}
// 		time.Sleep(10 * time.Millisecond)
// 	}

// 	// 5. Let's wait on 10 fetches.
// 	var wReqL []*prompb.WriteRequest
// 	for i := 0; i < 10; i++ {
// 		wReqL = append(wReqL, <-prweUploads)
// 	}
// 	defer cancel()

// 	// 6. Assert that we encounter the stale markers aka special NaNs for the various time series.
// 	staleMarkerCount := 0
// 	totalSamples := 0
// 	require.True(t, len(wReqL) > 0, "Expecting at least one WriteRequest")
// 	for i, wReq := range wReqL {
// 		name := fmt.Sprintf("WriteRequest#%d", i)
// 		require.True(t, len(wReq.Timeseries) > 0, "Expecting at least 1 timeSeries for:: "+name)
// 		for j, ts := range wReq.Timeseries {
// 			fullName := fmt.Sprintf("%s/TimeSeries#%d", name, j)
// 			assert.True(t, len(ts.Samples) > 0, "Expected at least 1 Sample in:: "+fullName)

// 			// We are strictly counting series directly included in the scrapes, and no
// 			// internal timeseries like "up" nor "scrape_seconds" etc.
// 			metricName := ""
// 			for _, label := range ts.Labels {
// 				if label.Name == "__name__" {
// 					metricName = label.Value
// 				}
// 			}
// 			if !strings.HasPrefix(metricName, "jvm") {
// 				continue
// 			}

// 			for _, sample := range ts.Samples {
// 				totalSamples++
// 				if value.IsStaleNaN(sample.Value) {
// 					staleMarkerCount++
// 				}
// 			}
// 		}
// 	}

// 	require.True(t, totalSamples > 0, "Expected at least 1 sample")
// 	// On every alternative scrape the prior scrape will be reported as sale.
// 	// Expect at least:
// 	//    * The first scrape will NOT return stale markers
// 	//    * (N-1 / alternatives) = ((10-1) / 2) = ~40% chance of stale markers being emitted.
// 	chance := float64(staleMarkerCount) / float64(totalSamples)
// 	require.True(t, chance >= 0.4, fmt.Sprintf("Expected at least one stale marker: %.3f", chance))
// }
