// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"
)

type analyzeCmd struct {
	store *modulestore.Store

	coredumpPath string
	casePath     string
	lwpFilter    string
	debugEbpf    bool
	debugLog     bool
	pid          int
}

func newAnalyzeCmd(store *modulestore.Store) *ffcli.Command {
	args := &analyzeCmd{store: store}

	set := flag.NewFlagSet("analyze", flag.ExitOnError)
	set.StringVar(&args.coredumpPath, "core", "", "Path of the coredump to analyze")
	set.StringVar(&args.casePath, "case", "", "Path of the test case to analyze")
	set.StringVar(&args.lwpFilter, "lwp", "", "Only unwind certain threads (comma separated)")
	set.BoolVar(&args.debugEbpf, "debug-ebpf", false, "Enable eBPF debug printing")
	set.BoolVar(&args.debugLog, "debug-log", false, "Enable HA debug logging")
	set.IntVar(&args.pid, "pid", 0, "PID to analyze")

	return &ffcli.Command{
		Name:       "analyze",
		Exec:       args.exec,
		ShortUsage: "analyze [flags]",
		ShortHelp:  "Analyze a coredump file",
		FlagSet:    set,
	}
}

func (cmd *analyzeCmd) exec(context.Context, []string) (err error) {
	// Validate arguments.
	sourceArgCount := 0
	if cmd.coredumpPath != "" {
		sourceArgCount++
	}
	if cmd.pid != 0 {
		sourceArgCount++
	}
	if cmd.casePath != "" {
		sourceArgCount++
	}
	if sourceArgCount != 1 {
		return errors.New("please specify either `-core`, `-case` or `-pid`")
	}

	lwpFilter := libpf.Set[libpf.PID]{}
	if cmd.lwpFilter != "" {
		for _, lwp := range strings.Split(cmd.lwpFilter, ",") {
			var parsed int64
			parsed, err = strconv.ParseInt(lwp, 10, 32)
			if err != nil {
				return fmt.Errorf("failed to parse LWP: %v", err)
			}
			lwpFilter[libpf.PID(parsed)] = libpf.Void{}
		}
	}

	if cmd.debugLog {
		log.SetLevel(log.DebugLevel)
	}

	var proc process.Process
	switch {
	case cmd.pid != 0:
		proc, err = process.NewPtrace(libpf.PID(cmd.pid))
		if err != nil {
			return fmt.Errorf("failed to open pid `%d`: %w", cmd.pid, err)
		}
	case cmd.casePath != "":
		var testCase *CoredumpTestCase
		testCase, err = readTestCase(cmd.casePath)
		if err != nil {
			return fmt.Errorf("failed to read test case: %w", err)
		}

		proc, err = OpenStoreCoredump(cmd.store, testCase.CoredumpRef, testCase.Modules)
		if err != nil {
			return fmt.Errorf("failed to open coredump: %w", err)
		}
	default:
		proc, err = process.OpenCoredump(cmd.coredumpPath)
		if err != nil {
			return fmt.Errorf("failed to open coredump `%s`: %w", cmd.coredumpPath, err)
		}
	}
	defer proc.Close()

	threads, err := ExtractTraces(context.Background(), proc, cmd.debugEbpf, lwpFilter)
	if err != nil {
		return fmt.Errorf("failed to extract traces: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(threads); err != nil {
		return fmt.Errorf("JSON Marshall failed: %w", err)
	}

	return nil
}
