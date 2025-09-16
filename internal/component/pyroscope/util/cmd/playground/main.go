package main

import (
	"context"
	"os"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
)

func main() {
	l := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	reg := prometheus.NewRegistry()
	args := ebpf.NewDefaultArguments()
	args.PyroscopeDynamicProfilingPolicy = false
	component, err := ebpf.New(l, reg, "playground", args)
	if err != nil {
		_ = l.Log("msg", "error creating ebpf component", "err", err)
		os.Exit(1)
	}
	g, ctx := errgroup.WithContext(context.Background())
	g.Go(func() error {
		return component.Run(ctx)
	})
	err = g.Wait()
	if err != nil {
		_ = l.Log("msg", "error running component", "err", err)
		os.Exit(1)
	}
}
