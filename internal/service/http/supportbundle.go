package http

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/static/server"
	"github.com/mackerelio/go-osstat/uptime"
	"gopkg.in/yaml.v3"
)

// SupportBundleContext groups the relevant context that is used in the HTTP
// service config for the support bundle
type SupportBundleContext struct {
	DisableSupportBundle bool     // Whether support bundle endpoint should be disabled.
	RuntimeFlags         []string // Alloy runtime flags to send with support bundle
}

// Bundle collects all the data that is exposed as a support bundle.
type Bundle struct {
	meta                 []byte
	alloyMetricsStart    []byte
	alloyMetricsEnd      []byte
	components           []byte
	peers                []byte
	runtimeFlags         []byte
	environmentVariables []byte
	sources              map[string][]byte
	remoteCfg            []byte
	heapBuf              *bytes.Buffer
	goroutineBuf         *bytes.Buffer
	blockBuf             *bytes.Buffer
	mutexBuf             *bytes.Buffer
	cpuBuf               *bytes.Buffer
}

// Metadata contains general runtime information about the current Alloy environment.
type Metadata struct {
	BuildVersion string  `yaml:"build_version"`
	OS           string  `yaml:"os"`
	Architecture string  `yaml:"architecture"`
	Uptime       float64 `yaml:"uptime"`
}

// ExportSupportBundle gathers the information required for the support bundle.
func ExportSupportBundle(ctx context.Context, runtimeFlags []string, srvAddress string, sources map[string][]byte, remoteCfg []byte, dialContext server.DialContextFunc) (*Bundle, error) {
	var httpClient http.Client
	httpClient.Transport = &http.Transport{DialContext: dialContext}

	// Gather Alloy's own metrics.
	alloyMetricsStart, err := retrieveAPIEndpoint(httpClient, srvAddress, "metrics")
	if err != nil {
		return nil, fmt.Errorf("failed to get internal Alloy metrics: %s", err)
	}

	// Gather running component configuration
	components, err := retrieveAPIEndpoint(httpClient, srvAddress, "api/v0/web/components")
	if err != nil {
		return nil, fmt.Errorf("failed to get component details: %s", err)
	}
	// Gather cluster peers information
	peers, err := retrieveAPIEndpoint(httpClient, srvAddress, "api/v0/web/peers")
	if err != nil {
		return nil, fmt.Errorf("failed to get peer details: %s", err)
	}

	// The block profiler is disabled by default. Temporarily enable recording
	// of all blocking events. Also, temporarily record all mutex contentions,
	// and defer restoring of earlier mutex profiling fraction.
	runtime.SetBlockProfileRate(1)
	old := runtime.SetMutexProfileFraction(1)
	defer func() {
		runtime.SetBlockProfileRate(0)
		runtime.SetMutexProfileFraction(old)
	}()

	// Gather runtime metadata.
	ut, err := uptime.Get()
	if err != nil {
		return nil, err
	}
	m := Metadata{
		BuildVersion: build.Version,
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		Uptime:       ut.Seconds(),
	}
	meta, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal support bundle metadata: %s", err)
	}

	// Export pprof data.
	var (
		cpuBuf       bytes.Buffer
		heapBuf      bytes.Buffer
		goroutineBuf bytes.Buffer
		blockBuf     bytes.Buffer
		mutexBuf     bytes.Buffer
	)
	err = pprof.StartCPUProfile(&cpuBuf)
	if err != nil {
		return nil, err
	}
	deadline, _ := ctx.Deadline()
	// Sleep for the remaining of the context deadline, but leave some time for
	// the rest of the bundle to be exported successfully.
	time.Sleep(time.Until(deadline) - 200*time.Millisecond)
	pprof.StopCPUProfile()

	p := pprof.Lookup("heap")
	if err := p.WriteTo(&heapBuf, 0); err != nil {
		return nil, err
	}
	p = pprof.Lookup("goroutine")
	if err := p.WriteTo(&goroutineBuf, 0); err != nil {
		return nil, err
	}
	p = pprof.Lookup("block")
	if err := p.WriteTo(&blockBuf, 0); err != nil {
		return nil, err
	}
	p = pprof.Lookup("mutex")
	if err := p.WriteTo(&mutexBuf, 0); err != nil {
		return nil, err
	}

	// Gather Alloy's own metrics after the profile completes
	alloyMetricsEnd, err := retrieveAPIEndpoint(httpClient, srvAddress, "metrics")
	if err != nil {
		return nil, fmt.Errorf("failed to get internal Alloy metrics: %s", err)
	}

	// Finally, bundle everything up to be served, either as a zip from
	// memory, or exported to a directory.
	bundle := &Bundle{
		meta:                 meta,
		alloyMetricsStart:    alloyMetricsStart,
		alloyMetricsEnd:      alloyMetricsEnd,
		components:           components,
		peers:                peers,
		sources:              sources,
		remoteCfg:            remoteCfg,
		runtimeFlags:         []byte(strings.Join(runtimeFlags, "\n")),
		environmentVariables: []byte(strings.Join(retrieveEnvironmentVariables(), "\n")),
		heapBuf:              &heapBuf,
		goroutineBuf:         &goroutineBuf,
		blockBuf:             &blockBuf,
		mutexBuf:             &mutexBuf,
		cpuBuf:               &cpuBuf,
	}

	return bundle, nil
}

func retrieveAPIEndpoint(httpClient http.Client, srvAddress, endpoint string) ([]byte, error) {
	url := fmt.Sprintf("http://%s/%s", srvAddress, endpoint)
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	res, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func retrieveEnvironmentVariables() []string {
	relevantVariables := []string{
		"AUTOMEMLIMIT",
		"GODEBUG",
		"GOGC",
		"GOMAXPROCS",
		"GOMEMLIMIT",
		"HOSTNAME",
		// the proxy variables can be provided either uppercased or lowercased
		"HTTP_PROXY",
		"http_proxy",
		"HTTPS_PROXY",
		"https_proxy",
		"NO_PROXY",
		"no_proxy",
		"PPROF_BLOCK_PROFILING_RATE",
		"PPROF_MUTEX_PROFILING_PERCENT",
	}
	values := []string{}
	for _, v := range relevantVariables {
		values = append(values, fmt.Sprintf("%s=%s", v, os.Getenv(v)))
	}

	return values
}

// ServeSupportBundle the collected data and logs as a zip file over the given
// http.ResponseWriter.
func ServeSupportBundle(rw http.ResponseWriter, b *Bundle, logsBuf *bytes.Buffer) error {
	zw := zip.NewWriter(rw)
	rw.Header().Set("Content-Type", "application/zip")
	rw.Header().Set("Content-Disposition", "attachment; filename=\"alloy-support-bundle.zip\"")

	zipStructure := map[string][]byte{
		"alloy-metadata.yaml":            b.meta,
		"alloy-components.json":          b.components,
		"alloy-peers.json":               b.peers,
		"alloy-metrics-sample-start.txt": b.alloyMetricsStart,
		"alloy-metrics-sample-end.txt":   b.alloyMetricsEnd,
		"alloy-runtime-flags.txt":        b.runtimeFlags,
		"alloy-environment.txt":          b.environmentVariables,
		"alloy-logs.txt":                 logsBuf.Bytes(),
		"pprof/cpu.pprof":                b.cpuBuf.Bytes(),
		"pprof/heap.pprof":               b.heapBuf.Bytes(),
		"pprof/goroutine.pprof":          b.goroutineBuf.Bytes(),
		"pprof/mutex.pprof":              b.mutexBuf.Bytes(),
		"pprof/block.pprof":              b.blockBuf.Bytes(),
	}

	if len(b.remoteCfg) > 0 {
		zipStructure["sources/remote-config/remote.alloy"] = b.remoteCfg
	}

	for p, s := range b.sources {
		zipStructure[filepath.Join("sources", filepath.Base(p))] = s
	}

	for fn, b := range zipStructure {
		if b != nil {
			path := append([]string{"alloy-support-bundle"}, strings.Split(fn, "/")...)
			if err := writeByteSlice(zw, b, path...); err != nil {
				return err
			}
		}
	}

	err := zw.Close()
	if err != nil {
		return fmt.Errorf("failed to flush the zip writer: %v", err)
	}
	return nil
}

func writeByteSlice(zw *zip.Writer, b []byte, fn ...string) error {
	f, err := zw.Create(filepath.Join(fn...))
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err != nil {
		return err
	}
	return nil
}
