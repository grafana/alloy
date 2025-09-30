//go:build linux && (arm64 || amd64)

package main

import (
	"context"
	"os"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf"
	"github.com/grafana/alloy/internal/component/pyroscope/write"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace/noop"
)

var (
	l   = log.With(log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout)), "ts", log.DefaultTimestampUTC)
	reg = prometheus.NewRegistry()
)

func newWrite() pyroscope.Appendable {
	var receiver pyroscope.Appendable
	e := write.GetDefaultEndpointOptions()
	e.URL = "http://localhost:4040"
	_, err := write.New(
		log.With(l, "component", "write"),
		noop.Tracer{},
		reg,
		func(exports write.Exports) {
			receiver = exports.Receiver
		},
		"playground",
		"",
		write.Arguments{Endpoints: []*write.EndpointOptions{&e}},
	)
	if err != nil {
		_ = l.Log("msg", "error creating write component", "err", err)
		os.Exit(1)
	}
	return receiver
}

func newEbpf(forward pyroscope.Appendable) *ebpf.Component {
	args := ebpf.NewDefaultArguments()
	args.PyroscopeDynamicProfilingPolicy = false
	args.ForwardTo = []pyroscope.Appendable{forward}
	args.ReporterUnsymbolizedStubs = true
	args.Demangle = "full"
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
	w := newWrite()
	e := newEbpf(w)

	g := run.Group{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g.Add(func() error {
		return e.Run(ctx)
	}, func(err error) {
		cancel()
	})
	if err := g.Run(); err != nil {
		_ = l.Log("msg", "error running component", "err", err)
		os.Exit(1)
	}
}
