//go:build linux && (arm64 || amd64)

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf"
	"github.com/grafana/alloy/internal/component/pyroscope/java"
	"github.com/grafana/alloy/internal/component/pyroscope/testutil"
	"github.com/grafana/alloy/internal/component/pyroscope/write"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	l   = log.With(log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout)), "ts", log.DefaultTimestampUTC)
	reg = prometheus.NewRegistry()
)

type config struct {
	ebpfEnabled bool
	javaPids    pids
	uprobeLinks stringSlice
}

func parseConfig() *config {
	c := &config{}
	flag.BoolVar(&c.ebpfEnabled, "ebpf", true, "enable ebpf")
	flag.Var(&c.javaPids, "java", "java process id")
	flag.Var(&c.uprobeLinks, "uprobe", "uprobe link (can be specified multiple times)")
	flag.Parse()
	return c
}

func newWrite() (pyroscope.Appendable, *write.Component) {
	receiver, c, err := testutil.CreateWriteComponent(l, reg, "http://localhost:4040")
	if err != nil {
		_ = l.Log("msg", "error creating write component", "err", err)
		os.Exit(1)
	}
	return receiver, c
}

func newEbpf(forward pyroscope.Appendable, uprobeLinks []string) *ebpf.Component {
	args := ebpf.NewDefaultArguments()
	args.PyroscopeDynamicProfilingPolicy = false
	args.ForwardTo = []pyroscope.Appendable{forward}
	args.ReporterUnsymbolizedStubs = true
	args.Demangle = "full"
	args.UProbeLinks = uprobeLinks
	// args.DebugInfoOptions.UploadEnabled = true
	e, err := ebpf.New(
		log.With(l, "component", "ebpf"),
		reg,
		"playground",
		args,
	)
	if err != nil {
		_ = l.Log("msg", "error creating ebpf component", "err", err)
		os.Exit(1)
	}
	return e
}

func main() {
	g := run.Group{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel2 := func(err error) {
		cancel()
	}

	cfg := parseConfig()
	w, wc := newWrite()
	g.Add(func() error {
		return wc.Run(ctx)
	}, cancel2)

	if cfg.ebpfEnabled {
		e := newEbpf(w, cfg.uprobeLinks)
		g.Add(func() error {
			return e.Run(ctx)
		}, cancel2)
	}
	if len(cfg.javaPids) > 0 {
		j := newJava(cfg.javaPids, w)
		g.Add(func() error {
			return j.Run(ctx)
		}, cancel2)
	}

	if err := g.Run(); err != nil {
		_ = l.Log("msg", "error running component", "err", err)
		os.Exit(1)
	}
}

func newJava(ps pids, w pyroscope.Appendable) *java.Component {
	args := java.DefaultArguments()
	args.ForwardTo = []pyroscope.Appendable{w}
	for _, pid := range ps {
		exe, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		cwd, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cwd", pid))
		t := discovery.NewTargetFromMap(map[string]string{
			java.LabelProcessID: strconv.Itoa(pid),
			"exe":               exe,
			"cwd":               string(cwd),
		})
		args.Targets = append(args.Targets, t)
	}

	j, err := java.New(l, reg, "java", args)
	if err != nil {
		_ = l.Log("msg", "error creating java component", "err", err)
		os.Exit(1)
	}
	return j
}

type pids []int

func (p *pids) String() string {
	return fmt.Sprintf("%+v", *p)
}

func (p *pids) Set(value string) error {
	pid, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	*p = append(*p, pid)
	return nil
}

type stringSlice []string

func (s *stringSlice) String() string {
	return fmt.Sprintf("%+v", *s)
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}
