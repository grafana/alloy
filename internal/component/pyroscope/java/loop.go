//go:build linux && (amd64 || arm64)

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
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/java/asprof"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	jfrpprof "github.com/grafana/jfr-parser/pprof"
	jfrpprofPyroscope "github.com/grafana/jfr-parser/pprof/pyroscope"
	"github.com/prometheus/prometheus/model/labels"
	gopsutil "github.com/shirou/gopsutil/v3/process"
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
	error      error
	dist       *asprof.Distribution
	jfrFile    string
	startTime  time.Time
	profiler   *asprof.Profiler
	sampleRate int
}

func newProfilingLoop(pid int, target discovery.Target, logger log.Logger, profiler *asprof.Profiler, output *pyroscope.Fanout, cfg ProfilingConfig) *profilingLoop {
	ctx, cancel := context.WithCancel(context.Background())
	dist, err := profiler.DistributionForProcess(pid)
	p := &profilingLoop{
		logger:   log.With(logger, "pid", pid),
		output:   output,
		pid:      pid,
		target:   target,
		cancel:   cancel,
		dist:     dist,
		jfrFile:  fmt.Sprintf("/tmp/asprof-%d-%d.jfr", os.Getpid(), pid),
		cfg:      cfg,
		profiler: profiler,
	}
	_ = level.Debug(p.logger).Log("msg", "new process", "target", fmt.Sprintf("%+v", target))

	if err != nil {
		p.onError(fmt.Errorf("failed to select dist for pid %d: %w", pid, err))
		return p
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.loop(ctx)
	}()
	return p
}

func (p *profilingLoop) loop(ctx context.Context) {
	if err := p.profiler.CopyLib(p.dist, p.pid); err != nil {
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

func (p *profilingLoop) reset() error {
	jfrFile := asprof.ProcessPath(p.jfrFile, p.pid)
	startTime := p.startTime
	endTime := time.Now()
	sampleRate := p.sampleRate
	p.startTime = endTime
	defer func() {
		os.Remove(jfrFile)
	}()

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
	for _, req := range profiles.Profiles {
		metric := req.Metric
		sz := req.Profile.SizeVT()
		l := log.With(p.logger, "metric", metric, "sz", sz)
		ls := labels.NewBuilder(nil)
		for _, l := range jfrpprofPyroscope.Labels(target, profiles.JFREvent, req.Metric, "", spyName) {
			ls.Set(l.Name, l.Value)
		}
		if ls.Get(labelServiceName) == "" {
			ls.Set(labelServiceName, inferServiceName(target))
		}

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
	}
	return nil
}

func (p *profilingLoop) start() error {
	cfg := p.getConfig()
	p.startTime = time.Now()
	p.sampleRate = cfg.SampleRate
	argv := make([]string, 0, 14)
	// asprof cli reference: https://github.com/async-profiler/async-profiler?tab=readme-ov-file#profiler-options
	argv = append(argv,
		"-f", p.jfrFile,
		"-o", "jfr",
	)
	if cfg.CPU {
		argv = append(argv, "-e", cfg.Event)
		if cfg.PerThread {
			argv = append(argv, "-t")
		}
		profilingInterval := time.Second.Nanoseconds() / int64(cfg.SampleRate)
		argv = append(argv, "-i", strconv.FormatInt(profilingInterval, 10))
	}
	if cfg.Alloc != "" {
		argv = append(argv, "--alloc", cfg.Alloc)
	}
	if cfg.Lock != "" {
		argv = append(argv, "--lock", cfg.Lock)
	}
	argv = append(argv,
		"start",
		"--timeout", strconv.Itoa(int(p.interval().Seconds())),
		strconv.Itoa(p.pid),
	)

	_ = level.Debug(p.logger).Log("cmd", fmt.Sprintf("%s %s", p.dist.LauncherPath(), strings.Join(argv, " ")))
	stdout, stderr, err := p.profiler.Execute(p.dist, argv)
	if err != nil {
		return fmt.Errorf("asprof failed to run: %w %s %s", err, stdout, stderr)
	}
	return nil
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
	_ = level.Debug(p.logger).Log("msg", "asprof", "cmd", fmt.Sprintf("%s %s", p.dist.LauncherPath(), strings.Join(argv, " ")))
	stdout, stderr, err := p.profiler.Execute(p.dist, argv)
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
	return alive
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
