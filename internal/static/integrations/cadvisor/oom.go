//go:build linux

package cadvisor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"regexp"
	"sync"
	"time"

	"github.com/euank/go-kmsg-parser/kmsgparser"
	info "github.com/google/cadvisor/lib/model"
)

// Since cAdvisor v0.60 the library manager no longer watches the kernel log
// for OOM kills and always reports zero for ContainerStats.OOMEvents. The
// cAdvisor binary does the watching itself (cmd/internal/api/oom.go), which
// can't be imported. This file does the equivalent for the integration, so
// that container_oom_events_total is populated again.

// oomEvent is a single OOM kill reported by the kernel.
type oomEvent struct {
	// ContainerName is the cgroup of the killed task (task_memcg). This matches
	// the container name the cAdvisor manager uses, and what cAdvisor counted
	// OOM events against before v0.60.
	ContainerName string
	Timestamp     time.Time
}

// oomSource streams OOM events to emit until ctx is done. It must return once
// ctx is done, and must not call emit after it returns.
type oomSource func(ctx context.Context, emit func(oomEvent)) error

// oomCounter holds the cumulative number of OOM kills per container.
type oomCounter struct {
	mu          sync.Mutex
	byContainer map[string]uint64
}

func newOOMCounter() *oomCounter {
	return &oomCounter{byContainer: map[string]uint64{}}
}

func (c *oomCounter) inc(containerName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byContainer[containerName]++
}

func (c *oomCounter) get(containerName string) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.byContainer[containerName]
}

// watchOOMs runs src until ctx is done and counts the events it reports.
// Watching is best-effort: if src fails, OOM events stay at their current
// value and a warning is logged, but the rest of the integration keeps working.
func watchOOMs(ctx context.Context, l *slog.Logger, src oomSource, counter *oomCounter) {
	err := src(ctx, func(ev oomEvent) {
		l.Debug("observed OOM kill", "container", ev.ContainerName, "timestamp", ev.Timestamp)
		counter.inc(ev.ContainerName)
	})
	if err != nil && ctx.Err() == nil {
		l.Warn("OOM event watching stopped, container_oom_events_total will not increase", "err", err)
	}
}

// runWithOOMWatcher runs src until ctx is done, waits for it to exit, and then
// calls stop. A nil src disables OOM watching.
func runWithOOMWatcher(ctx context.Context, l *slog.Logger, src oomSource, counter *oomCounter, stop func() error) error {
	var wg sync.WaitGroup
	if src != nil {
		wg.Go(func() { watchOOMs(ctx, l, src, counter) })
	}

	<-ctx.Done()
	wg.Wait()
	return stop()
}

// containerInfoProvider is the set of manager methods used by
// metrics.NewPrometheusCollector.
type containerInfoProvider interface {
	GetRequestedContainersInfo(containerName string, options info.RequestOptions) (map[string]*info.ContainerInfo, error)
	GetVersionInfo() (*info.VersionInfo, error)
	GetMachineInfo() (*info.MachineInfo, error)
}

// oomInfoProvider wraps a containerInfoProvider and fills in
// ContainerStats.OOMEvents from an oomCounter.
type oomInfoProvider struct {
	containerInfoProvider
	counter *oomCounter
}

func (p *oomInfoProvider) GetRequestedContainersInfo(containerName string, options info.RequestOptions) (map[string]*info.ContainerInfo, error) {
	infos, err := p.containerInfoProvider.GetRequestedContainersInfo(containerName, options)
	for _, ci := range infos {
		if ci == nil {
			continue
		}
		count := p.counter.get(ci.Name)
		if count == 0 {
			continue
		}
		for i, stats := range ci.Stats {
			if stats == nil {
				continue
			}
			// The stats point into the manager's in-memory cache, which other
			// scrapes may read at the same time. Copy before setting the count.
			s := *stats
			s.OOMEvents = count
			ci.Stats[i] = &s
		}
	}
	return infos, err
}

var (
	// oomStartRegexp matches the first line of a kernel OOM report.
	oomStartRegexp = regexp.MustCompile(`invoked oom-killer:`)
	// oomKillRegexp matches the summary line printed by Linux 5.0 and later.
	oomKillRegexp = regexp.MustCompile(`oom-kill:constraint=(.*),nodemask=(.*),cpuset=(.*),mems_allowed=(.*),oom_memcg=(.*),task_memcg=(.*),task=(.*),pid=(.*),uid=(.*)`)
	// oomLegacyContainerRegexp matches the container line printed by kernels before 5.0.
	oomLegacyContainerRegexp = regexp.MustCompile(`Task in (.*) killed as a result of limit of (.*)`)
	// oomKilledProcessRegexp matches the last line of a kernel OOM report.
	oomKilledProcessRegexp = regexp.MustCompile(`Killed process ([0-9]+) \((.+)\)`)
)

// parseOOMs reads kernel log messages until msgs is closed and calls emit for
// every complete OOM kill report. It follows the parsing of cAdvisor's
// utils/oomparser package.
func parseOOMs(msgs <-chan kmsgparser.Message, emit func(oomEvent)) {
	for msg := range msgs {
		if !oomStartRegexp.MatchString(msg.Message) {
			continue
		}

		ev := oomEvent{ContainerName: "/", Timestamp: msg.Timestamp}
		finished := false
		for msg := range msgs {
			if finished = parseOOMLine(msg.Message, &ev); finished {
				ev.Timestamp = msg.Timestamp
				break
			}
		}
		if !finished {
			// msgs closed in the middle of a report.
			return
		}
		emit(ev)
	}
}

// parseOOMLine updates ev from one line of an OOM report. It returns true
// once the report is complete.
func parseOOMLine(line string, ev *oomEvent) bool {
	if m := oomKillRegexp.FindStringSubmatch(line); m != nil {
		ev.ContainerName = m[6]
		return true
	}
	if m := oomLegacyContainerRegexp.FindStringSubmatch(line); m != nil {
		ev.ContainerName = path.Join("/", m[1])
		return false
	}
	return oomKilledProcessRegexp.MatchString(line)
}

// kmsgOOMSource returns an oomSource that reads OOM kills from /dev/kmsg.
//
// cAdvisor's utils/oomparser is not used because it can't be stopped: it
// keeps /dev/kmsg open and its goroutines running for the life of the
// process. Here the reader is closed when ctx is done, which ends the parser
// goroutine.
func kmsgOOMSource(l *slog.Logger) oomSource {
	return func(ctx context.Context, emit func(oomEvent)) error {
		parser, err := kmsgparser.NewParser()
		if err != nil {
			return fmt.Errorf("failed to open kernel log: %w", err)
		}
		parser.SetLogger(kmsgLogger{ctx: ctx, l: l})

		msgs := parser.Parse()
		stop := context.AfterFunc(ctx, func() { _ = parser.Close() })

		// Read until the channel is closed, even after ctx is done, so the
		// parser goroutine never blocks on a send and always exits.
		parseOOMs(msgs, emit)
		for range msgs {
		}

		if stop() {
			// The stream ended before ctx was done.
			_ = parser.Close()
			return errors.New("kernel log stream ended")
		}
		return nil
	}
}

// kmsgLogger adapts slog to kmsgparser.Logger. Read errors after ctx is done
// are expected (they come from closing the reader), so they are logged at
// debug level.
type kmsgLogger struct {
	ctx context.Context
	l   *slog.Logger
}

var _ kmsgparser.Logger = kmsgLogger{}

func (k kmsgLogger) Infof(format string, args ...any) {
	k.l.Debug(fmt.Sprintf(format, args...))
}

func (k kmsgLogger) Warningf(format string, args ...any) {
	k.l.Debug(fmt.Sprintf(format, args...))
}

func (k kmsgLogger) Errorf(format string, args ...any) {
	if k.ctx.Err() != nil {
		k.l.Debug(fmt.Sprintf(format, args...))
		return
	}
	k.l.Warn(fmt.Sprintf(format, args...))
}
