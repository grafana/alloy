package supportbundle

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/grafana/agent/internal/build"
	"github.com/grafana/agent/internal/static/server"
	"github.com/mackerelio/go-osstat/uptime"
	"gopkg.in/yaml.v3"
)

// Bundle collects all the data that is exposed as a support bundle.
type Bundle struct {
	meta                  []byte
	config                []byte
	agentMetrics          []byte
	agentMetricsInstances []byte
	agentMetricsTargets   []byte
	agentLogsInstances    []byte
	agentLogsTargets      []byte
	heapBuf               *bytes.Buffer
	goroutineBuf          *bytes.Buffer
	blockBuf              *bytes.Buffer
	mutexBuf              *bytes.Buffer
	cpuBuf                *bytes.Buffer
}

// Metadata contains general runtime information about the current Agent.
type Metadata struct {
	BuildVersion string                 `yaml:"build_version"`
	OS           string                 `yaml:"os"`
	Architecture string                 `yaml:"architecture"`
	Uptime       float64                `yaml:"uptime"`
	Payload      map[string]interface{} `yaml:"payload"`
}

// Used to enforce single-flight requests to Export
var mut sync.Mutex

// Export gathers the information required for the support bundle.
func Export(ctx context.Context, enabledFeatures []string, cfg []byte, srvAddress string, dialContext server.DialContextFunc) (*Bundle, error) {
	mut.Lock()
	defer mut.Unlock()
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
		Payload:      map[string]interface{}{"enabled-features": enabledFeatures},
	}
	meta, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal support bundle metadata: %s", err)
	}

	var httpClient http.Client
	httpClient.Transport = &http.Transport{DialContext: dialContext}
	// Gather Agent's own metrics.
	resp, err := httpClient.Get("http://" + srvAddress + "/metrics")
	if err != nil {
		return nil, fmt.Errorf("failed to get internal Agent metrics: %s", err)
	}
	agentMetrics, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read internal Agent metrics: %s", err)
	}

	// Collect the Agent metrics instances and target statuses.
	resp, err = httpClient.Get("http://" + srvAddress + "/agent/api/v1/metrics/instances")
	if err != nil {
		return nil, fmt.Errorf("failed to get internal Agent metrics: %s", err)
	}
	agentMetricsInstances, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read internal Agent metrics: %s", err)
	}
	resp, err = httpClient.Get("http://" + srvAddress + "/agent/api/v1/metrics/targets")
	if err != nil {
		return nil, fmt.Errorf("failed to get Agent metrics targets: %s", err)
	}
	agentMetricsTargets, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Agent metrics targets: %s", err)
	}

	// Collect the Agent's logs instances and target statuses.
	resp, err = httpClient.Get("http://" + srvAddress + "/agent/api/v1/logs/instances")
	if err != nil {
		return nil, fmt.Errorf("failed to get Agent logs instances: %s", err)
	}
	agentLogsInstances, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Agent logs instances: %s", err)
	}

	resp, err = httpClient.Get("http://" + srvAddress + "/agent/api/v1/logs/targets")
	if err != nil {
		return nil, fmt.Errorf("failed to get Agent logs targets: %s", err)
	}
	agentLogsTargets, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Agent logs targets: %s", err)
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

	// Finally, bundle everything up to be served, either as a zip from
	// memory, or exported to a directory.
	bundle := &Bundle{
		meta:                  meta,
		config:                cfg,
		agentMetrics:          agentMetrics,
		agentMetricsInstances: agentMetricsInstances,
		agentMetricsTargets:   agentMetricsTargets,
		agentLogsInstances:    agentLogsInstances,
		agentLogsTargets:      agentLogsTargets,
		heapBuf:               &heapBuf,
		goroutineBuf:          &goroutineBuf,
		blockBuf:              &blockBuf,
		mutexBuf:              &mutexBuf,
		cpuBuf:                &cpuBuf,
	}

	return bundle, nil
}

// Serve the collected data and logs as a zip file over the given
// http.ResponseWriter.
func Serve(rw http.ResponseWriter, b *Bundle, logsBuf *bytes.Buffer) error {
	zw := zip.NewWriter(rw)
	rw.Header().Set("Content-Type", "application/zip")
	rw.Header().Set("Content-Disposition", "attachment; filename=\"agent-support-bundle.zip\"")

	zipStructure := map[string][]byte{
		"agent-metadata.yaml":          b.meta,
		"agent-config.yaml":            b.config,
		"agent-metrics.txt":            b.agentMetrics,
		"agent-metrics-instances.json": b.agentMetricsInstances,
		"agent-metrics-targets.json":   b.agentMetricsTargets,
		"agent-logs-instances.json":    b.agentLogsInstances,
		"agent-logs-targets.json":      b.agentLogsTargets,
		"agent-logs.txt":               logsBuf.Bytes(),
		"pprof/cpu.pprof":              b.cpuBuf.Bytes(),
		"pprof/heap.pprof":             b.heapBuf.Bytes(),
		"pprof/goroutine.pprof":        b.goroutineBuf.Bytes(),
		"pprof/mutex.pprof":            b.mutexBuf.Bytes(),
		"pprof/block.pprof":            b.blockBuf.Bytes(),
	}

	for fn, b := range zipStructure {
		if b != nil {
			path := append([]string{"agent-support-bundle"}, strings.Split(fn, "/")...)
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
