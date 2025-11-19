// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/tar"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"
)

type exportCmd struct {
	store *modulestore.Store

	out string
}

func newExportCmd(store *modulestore.Store) *ffcli.Command {
	cmd := exportCmd{store: store}
	set := flag.NewFlagSet("export", flag.ExitOnError)
	set.StringVar(&cmd.out, "out", "", "Output file (tar) [required]")

	return &ffcli.Command{
		Name:       "export",
		ShortUsage: "export [flags] [testCaseJsonFiles...]",
		ShortHelp:  "Export one or more test case modules to tar",
		FlagSet:    set,
		Exec:       cmd.exec,
	}
}

func (cmd *exportCmd) exec(_ context.Context, testCases []string) error {
	if cmd.out == "" {
		return errors.New("missing required argument `-out`")
	}

	moduleIDs := make(libpf.Set[modulestore.ID])
	for _, testCase := range testCases {
		t, err := readTestCase(testCase)
		if err != nil {
			return fmt.Errorf("failed to open test case '%s': %w",
				testCase, err)
		}
		moduleIDs[t.CoredumpRef] = libpf.Void{}
		for _, m := range t.Modules {
			moduleIDs[m.Ref] = libpf.Void{}
		}
	}

	out, err := os.Create(cmd.out)
	if err != nil {
		return fmt.Errorf("failed to create '%s': %w", cmd.out, err)
	}
	defer out.Close()

	tw := tar.NewWriter(out)
	for id := range moduleIDs {
		if err := cmd.store.ExportModule(id, tw); err != nil {
			return fmt.Errorf("failed to export '%s': %w",
				id, err)
		}
	}
	return tw.Close()
}
