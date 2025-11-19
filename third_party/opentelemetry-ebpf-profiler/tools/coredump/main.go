// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// coredump provides a tool for extracting stack traces from coredumps.
// It also includes a test suite to unit test profiling agent components against
// a set of coredumps to validate stack extraction code.

package main

import (
	"context"
	"errors"
	"flag"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/ebpf-profiler/tools/coredump/cloudstore"
	"go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"
)

func main() {
	log.SetReportCaller(false)
	log.SetFormatter(&log.TextFormatter{})

	cloudClient, err := cloudstore.Client()
	if err != nil {
		log.Fatalf("%v", err)
	}
	store, err := modulestore.New(cloudClient,
		cloudstore.PublicReadURL(), cloudstore.ModulestoreS3Bucket(), "modulecache")
	if err != nil {
		log.Fatalf("%v", err)
	}

	root := ffcli.Command{
		Name:       "coredump",
		ShortUsage: "coredump <subcommand> [flags]",
		ShortHelp:  "Tool for creating and managing coredump test cases",
		Subcommands: []*ffcli.Command{
			newAnalyzeCmd(store),
			newCleanCmd(store),
			newExportCmd(store),
			newExportModuleCmd(store),
			newNewCmd(store),
			newRebaseCmd(store),
			newUploadCmd(store),
			newGdbCmd(store),
		},
		Exec: func(context.Context, []string) error {
			return flag.ErrHelp
		},
	}

	if err := root.ParseAndRun(context.Background(), os.Args[1:]); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			log.Fatalf("%v", err)
		}
	}
}
