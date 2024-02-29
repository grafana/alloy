package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const defaultPort = "9001"

type Config struct {
	ListenAddress string
}

func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cfg.ListenAddress, "bind", fmt.Sprintf(":%s", defaultPort), "Bind address")
}

func main() {
	// Parse CLI flags.
	cfg := &Config{}
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

	address, port := getAddressAndPort(cfg.ListenAddress)
	listenAddress := fmt.Sprintf("%s:%s", address, port)
	http.Handle("/metrics", promhttp.Handler())
	server := &http.Server{Addr: listenAddress, Handler: nil}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Server shutdown error: %v", err)
		}
	}()
	log.Printf("HTTP server on %s", listenAddress)

	go func() { log.Fatal(server.ListenAndServe()) }()

	labels := map[string]string{
		"address": address,
		"port":    port,
	}

	go handleCounter(setupCounter(labels))
	go handleGaugeInput(setupGauge(labels))
	go handleHistogramInput(setupHistogram(labels))
	go handleHistogramInput(setupNativeHistogram(labels))
	go handleSummary(setupSummary(labels))
	stopChan := make(chan struct{})
	<-stopChan
}

// getAddressAndPort always defines a non empty address and port
//
// The Go http server can use empty to mean any, but we want
// something meaningful in the metric labels.
func getAddressAndPort(listenAddress string) (string, string) {
	address, port, error := net.SplitHostPort(listenAddress)
	if error != nil {
		log.Fatal(error)
	}
	if address == "" {
		address = "0.0.0.0"
	}
	if port == "" {
		port = defaultPort
	}

	return address, port
}

func setupGauge(labels map[string]string) prometheus.Gauge {
	gauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   "golang",
			Name:        "gauge",
			ConstLabels: labels,
		})
	prometheus.MustRegister(gauge)
	return gauge
}

func handleGaugeInput(gauge prometheus.Gauge) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		newValue := rand.Float64() * 100
		gauge.Set(newValue)
	}
}

func setupNativeHistogram(labels map[string]string) prometheus.Histogram {
	nativeHistogram := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:                       "golang",
			Name:                            "native_histogram",
			ConstLabels:                     labels,
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: 1 * time.Hour,
		})
	prometheus.MustRegister(nativeHistogram)
	return nativeHistogram
}

func setupHistogram(labels map[string]string) prometheus.Histogram {
	histogram := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   "golang",
			Name:        "histogram",
			ConstLabels: labels,
			Buckets:     []float64{1, 10, 100, 1000},
		})
	prometheus.MustRegister(histogram)
	return histogram
}

func handleHistogramInput(histogram prometheus.Histogram) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		newValue := rand.Float64() * 1000
		histogram.Observe(newValue)
	}
}

func setupCounter(labels map[string]string) prometheus.Counter {
	counter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "golang",
			Name:        "counter",
			ConstLabels: labels,
		})
	prometheus.MustRegister(counter)
	return counter
}

func handleCounter(counter prometheus.Counter) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		counter.Inc()
	}
}

func setupSummary(labels map[string]string) prometheus.Summary {
	summary := prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace:   "golang",
			Name:        "summary",
			ConstLabels: labels,
			Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		})
	prometheus.MustRegister(summary)
	return summary
}

func handleSummary(summary prometheus.Summary) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		newValue := rand.Float64() * 1000
		summary.Observe(newValue)
	}
}
