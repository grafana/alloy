// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os/exec"

	"github.com/peterbourgon/ff/v3/ffcli"

	"go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"
)

type rebaseCmd struct {
	store *modulestore.Store

	allowDirty bool
}

func newRebaseCmd(store *modulestore.Store) *ffcli.Command {
	args := &rebaseCmd{store: store}

	set := flag.NewFlagSet("rebase", flag.ExitOnError)
	set.BoolVar(&args.allowDirty, "allow-dirty", false, "Allow uncommitted changes in git")

	return &ffcli.Command{
		Name:       "rebase",
		Exec:       args.exec,
		ShortUsage: "rebase",
		ShortHelp:  "Update all test cases by running them and saving the current unwinding",
		FlagSet:    set,
	}
}

func (cmd *rebaseCmd) exec(context.Context, []string) (err error) {
	cases, err := findTestCases(true)
	if err != nil {
		return fmt.Errorf("failed to find test cases: %w", err)
	}

	if !cmd.allowDirty {
		if err = exec.Command("git", "diff", "--quiet").Run(); err != nil {
			return errors.New("refusing to work on a dirty source tree. " +
				"please commit your changes first or pass `-allow-dirty` to ignore")
		}
	}

	for _, testCasePath := range cases {
		var testCase *CoredumpTestCase
		testCase, err = readTestCase(testCasePath)
		if err != nil {
			return fmt.Errorf("failed to read test case: %w", err)
		}

		core, err := OpenStoreCoredump(cmd.store, testCase.CoredumpRef, testCase.Modules)
		if err != nil {
			return fmt.Errorf("failed to open coredump: %w", err)
		}

		testCase.Threads, err = ExtractTraces(context.Background(), core, false, nil)
		_ = core.Close()
		if err != nil {
			return fmt.Errorf("failed to extract traces: %w", err)
		}

		if err = writeTestCase(testCasePath, testCase, true); err != nil {
			return fmt.Errorf("failed to write test case: %w", err)
		}
	}

	return nil
}
