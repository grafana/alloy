//go:build (linux || darwin) && (amd64 || arm64)

package java

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	jfrpprof "github.com/grafana/jfr-parser/pprof"
	jfrpprofPyroscope "github.com/grafana/jfr-parser/pprof/pyroscope"
	"github.com/prometheus/prometheus/model/labels"
	gopsutil "github.com/shirou/gopsutil/v3/process"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/java/asprof"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const spyName = "alloy.java"

type profilingLoop struct {
	logger     log.Logger
	output     *pyroscope.Fanout
	cfg        ProfilingConfig
	wg         sync.WaitGroup
	mutex      sync.Mutex
	pid        int
	target     discovery.Target
	cancel     context.CancelFunc
	jfrFile    string
	startTime  time.Time
	profiler   Profiler
	sampleRate int

	error            error
	lastError        time.Time
	lastPush         time.Time
	lastBytesPerType []debugInfoBytesPerType
	totalBytes       int64
	totalSamples     int64
}

type Profiler interface {
	CopyLib(pid int) error
	Execute(argv []string) (string, string, error)
}

func newProfilingLoop(pid int, target discovery.Target, logger log.Logger, profiler Profiler, output *pyroscope.Fanout, cfg ProfilingConfig) *profilingLoop {
	ctx, cancel := context.WithCancel(context.Background())
	p := &profilingLoop{
		logger:   log.With(logger, "pid", pid),
		output:   output,
		pid:      pid,
		target:   target,
		cancel:   cancel,
		jfrFile:  fmt.Sprintf("/tmp/asprof-%d-%d.jfr", os.Getpid(), pid),
		cfg:      cfg,
		profiler: profiler,
	}
	_ = level.Debug(p.logger).Log("msg", "new process", "target", fmt.Sprintf("%+v", target))

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.loop(ctx)
	}()
	return p
}

func (p *profilingLoop) loop(ctx context.Context) {
	if err := p.profiler.CopyLib(p.pid); err != nil {
		p.onError(fmt.Errorf("failed to copy libasyncProfiler.so: %w", err))
		return
	}
	defer func() {
		_ = p.stop()
	}()
	sleep := func() {
		timer := time.NewTimer(p.interval())
		defer timer.Stop()
		select {
		case <-timer.C:
			return
		case <-ctx.Done():
			return
		}
	}
	for {
		err := p.start()
		if err != nil {
			//  could happen when Alloy restarted - [ERROR] Profiler already started\n
			alive := p.onError(fmt.Errorf("failed to start: %w", err))
			if !alive {
				return
			}
		}
		sleep()
		if ctx.Err() != nil {
			return
		}
		err = p.reset()
		if err != nil {
			alive := p.onError(fmt.Errorf("failed to reset: %w", err))
			if !alive {
				return
			}
		}
	}
}

func (p *profilingLoop) cleanupJFR() {
	// first try to find through process path
	jfrFile := asprof.ProcessPath(p.jfrFile, p.pid)
	if err := os.Remove(jfrFile); os.IsNotExist(err) {
		// the process path was not found, this is possible when the target process stopped in the meantime.

		if jfrFile == p.jfrFile {
			// nothing we can do, the process path was not actually a /proc path
			return
		}

		jfrFile = p.jfrFile
		if err := os.Remove(jfrFile); os.IsNotExist(err) {
			_ = level.Debug(p.logger).Log("msg", "unable to delete jfr file, likely because target process is stopped and was containerized", "path", jfrFile, "err", err)
			// file not found on the host system, process was likely containerized and we can't delete this file anymore
			return
		} else if err != nil {
			_ = level.Warn(p.logger).Log("msg", "failed to delete jfr file at host path", "path", jfrFile, "err", err)
		}
	} else if err != nil {
		_ = level.Warn(p.logger).Log("msg", "failed to delete jfr file at process path", "path", jfrFile, "err", err)
	}
}

func (p *profilingLoop) reset() error {
	jfrFile := asprof.ProcessPath(p.jfrFile, p.pid)
	startTime := p.startTime
	endTime := time.Now()
	sampleRate := p.sampleRate
	p.startTime = endTime
	defer p.cleanupJFR()

	err := p.stop()
	if err != nil {
		return fmt.Errorf("failed to stop : %w", err)
	}
	jfrBytes, err := os.ReadFile(jfrFile)
	if err != nil {
		return fmt.Errorf("failed to read jfr file: %w", err)
	}
	_ = level.Debug(p.logger).Log("msg", "jfr file read", "len", len(jfrBytes))

	return p.push(jfrBytes, startTime, endTime, int64(sampleRate))
}
func (p *profilingLoop) push(jfrBytes []byte, startTime time.Time, endTime time.Time, sampleRate int64) error {
	profiles, err := jfrpprof.ParseJFR(jfrBytes, &jfrpprof.ParseInput{
		StartTime:  startTime,
		EndTime:    endTime,
		SampleRate: sampleRate,
	}, new(jfrpprof.LabelsSnapshot))
	if err != nil {
		return fmt.Errorf("failed to parse jfr: %w", err)
	}
	target := p.getTarget()
	var totalSamples, totalBytes int64

	// reset the per type bytes stats
	p.lastBytesPerType = p.lastBytesPerType[:0]

	for _, req := range profiles.Profiles {
		metric := req.Metric
		sz := req.Profile.SizeVT()
		l := log.With(p.logger, "metric", metric, "sz", sz)
		ls := labels.NewBuilder(labels.EmptyLabels())
		for _, l := range jfrpprofPyroscope.Labels(target.AsMap(), profiles.JFREvent, req.Metric, "", spyName) {
			ls.Set(l.Name, l.Value)
		}
		if ls.Get(labelServiceName) == "" {
			ls.Set(labelServiceName, inferServiceName(target))
		}

		p.lastBytesPerType = append(p.lastBytesPerType, debugInfoBytesPerType{
			Type:  metric,
			Bytes: int64(sz),
		})
		totalBytes += int64(sz)
		totalSamples += int64(len(req.Profile.Sample))

		profile, err := req.Profile.MarshalVT()
		if err != nil {
			_ = l.Log("msg", "failed to marshal profile", "err", err)
			continue
		}
		samples := []*pyroscope.RawSample{{RawProfile: profile}}
		err = p.output.Appender().Append(context.Background(), ls.Labels(), samples)
		if err != nil {
			_ = l.Log("msg", "failed to push jfr", "err", err)
			continue
		}
		_ = l.Log("msg", "pushed jfr-pprof")

		p.mutex.Lock()
		p.lastPush = time.Now()
		p.totalSamples += totalSamples
		p.totalBytes += totalBytes
		p.mutex.Unlock()
	}
	return nil
}

func (p *profilingLoop) start() error {
	argv := p.generateCommand()
	stdout, stderr, err := p.profiler.Execute(argv)
	if err != nil {
		return fmt.Errorf("asprof failed to run: %w %s %s", err, stdout, stderr)
	}
	return nil
}

func (p *profilingLoop) generateCommand() []string {
	cfg := p.getConfig()
	p.startTime = time.Now()
	p.sampleRate = cfg.SampleRate
	argv := make([]string, 0)
	// asprof cli reference: https://github.com/async-profiler/async-profiler?tab=readme-ov-file#profiler-options
	argv = append(argv,
		"-f", p.jfrFile,
		"-o", "jfr",
	)

	if cfg.Interval > 0 {
		argv = append(argv, "-i", strconv.Itoa(int(cfg.Interval.Microseconds())))
	}
	if cfg.CPU {
		argv = append(argv, "-e", "itimer")
	}
	if cfg.All {
		argv = append(argv, "--all")
	}
	if cfg.Event != "" {
		argv = append(argv, "-e", cfg.Event)
	}
	if cfg.PerThread {
		argv = append(argv, "-t")
	}
	if cfg.Wall != "" {
		argv = append(argv, "--wall", cfg.Wall)
	}
	if cfg.AllUser {
		argv = append(argv, "--all-user")
	}
	if cfg.Filter != "" {
		argv = append(argv, "--filter", cfg.Filter)
	}
	if cfg.Sched {
		argv = append(argv, "--sched")
	}
	if cfg.TTSP {
		argv = append(argv, "--ttsp")
	}
	if cfg.Begin != "" {
		argv = append(argv, "--begin", cfg.Begin)
	}
	if cfg.End != "" {
		argv = append(argv, "--end", cfg.End)
	}
	if cfg.NoStop {
		argv = append(argv, "--nostop")
	}
	if cfg.Proc != "" {
		argv = append(argv, "--proc", cfg.Proc)
	}
	if cfg.TargetCPU > 0 {
		argv = append(argv, "--target-cpu", strconv.Itoa(cfg.TargetCPU))
	}
	if cfg.RecordCPU {
		argv = append(argv, "--record-cpu")
	}
	if cfg.Alloc != "" {
		argv = append(argv, "--alloc", cfg.Alloc)
	}
	if cfg.Live {
		argv = append(argv, "--live")
	}
	if cfg.NativeMem != "" {
		argv = append(argv, "--nativemem", cfg.NativeMem)
	}
	if cfg.NoFree {
		argv = append(argv, "--nofree")
	}
	if cfg.Lock != "" {
		argv = append(argv, "--lock", cfg.Lock)
	}
	if cfg.NativeLock != "" {
		argv = append(argv, "--nativelock", cfg.NativeLock)
	}
	if cfg.LogLevel != "" {
		argv = append(argv, "-L", cfg.LogLevel)
	}
	for _, pattern := range cfg.Include {
		argv = append(argv, "-I", pattern)
	}
	for _, pattern := range cfg.Exclude {
		argv = append(argv, "-X", pattern)
	}
	if cfg.JStackDepth > 0 && cfg.JStackDepth != 2048 {
		argv = append(argv, "-j", strconv.Itoa(cfg.JStackDepth))
	}
	if cfg.CStack != "" {
		argv = append(argv, "--cstack", cfg.CStack)
	}
	if len(cfg.Features) > 0 {
		argv = append(argv, "-F", strings.Join(cfg.Features, ","))
	}
	for _, trace := range cfg.Trace {
		argv = append(argv, "--trace", trace)
	}
	if cfg.JFRSync != "" {
		argv = append(argv, "--jfrsync", cfg.JFRSync)
	}
	if cfg.Signal != "" {
		argv = append(argv, "--signal", cfg.Signal)
	}
	if cfg.Clock != "" && cfg.Clock != "tsc" {
		argv = append(argv, "--clock", cfg.Clock)
	}

	argv = append(argv,
		"start",
		"--timeout", strconv.Itoa(int(p.interval().Seconds())),
		strconv.Itoa(p.pid),
	)

	_ = level.Debug(p.logger).Log("cmd", strings.Join(argv, " "))
	return argv
}

func (p *profilingLoop) getConfig() ProfilingConfig {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.cfg
}

func (p *profilingLoop) stop() error {
	argv := []string{
		"stop",
		"-o", "jfr",
		strconv.Itoa(p.pid),
	}
	_ = level.Debug(p.logger).Log("msg", "asprof", "cmd", strings.Join(argv, " "))
	stdout, stderr, err := p.profiler.Execute(argv)
	if err != nil {
		return fmt.Errorf("asprof failed to run: %w %s %s", err, stdout, stderr)
	}
	_ = level.Debug(p.logger).Log("msg", "asprof stopped", "stdout", stdout, "stderr", stderr)
	return nil
}

func (p *profilingLoop) update(target discovery.Target, config ProfilingConfig) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.target = target
	p.cfg = config
}

// Close stops profiling this profilingLoop
func (p *profilingLoop) Close() error {
	p.cancel()
	p.wg.Wait()
	p.cleanupJFR()
	return nil
}

func (p *profilingLoop) onError(err error) bool {
	alive := p.alive()
	if alive {
		_ = level.Error(p.logger).Log("err", err)
	} else {
		_ = level.Debug(p.logger).Log("err", err)
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.error = err
	p.lastError = time.Now()
	return alive
}

func (p *profilingLoop) debugInfo() *debugInfoProfiledTarget {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	d := &debugInfoProfiledTarget{
		TotalBytes:   p.totalBytes,
		TotalSamples: p.totalSamples,
		LastProfiled: p.lastPush,
		LastError:    p.lastError,
		PID:          p.pid,
		Target:       p.target,
	}

	// expose per profile type bytes
	if len(p.lastBytesPerType) > 0 {
		d.LastProfileBytesPerType = make(map[string]int64)
		for _, b := range p.lastBytesPerType {
			d.LastProfileBytesPerType[b.Type] += b.Bytes
		}
	}

	// expose error message if given
	if p.error != nil {
		d.ErrorMsg = p.error.Error()
	}
	return d
}

func (p *profilingLoop) interval() time.Duration {
	return p.getConfig().Interval
}

func (p *profilingLoop) getTarget() discovery.Target {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.target
}

func (p *profilingLoop) alive() bool {
	exists, err := gopsutil.PidExists(int32(p.pid))
	if err != nil {
		_ = level.Error(p.logger).Log("msg", "failed to check if process is alive", "err", err)
	}
	return err == nil && exists
}
