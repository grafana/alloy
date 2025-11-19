// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"
)

type cleanCmd struct {
	store *modulestore.Store

	// User-specified command line arguments.
	local, remote, temp, dry bool
	minAge                   uint64
}

func newCleanCmd(store *modulestore.Store) *ffcli.Command {
	cmd := cleanCmd{store: store}
	set := flag.NewFlagSet("clean", flag.ExitOnError)
	set.BoolVar(&cmd.temp, "temp", true, "Delete lingering temporary files in the local cache")
	set.BoolVar(&cmd.local, "local", true, "Clean the local cache")
	set.BoolVar(&cmd.remote, "remote", false, "Clean the remote storage")
	set.BoolVar(&cmd.dry, "dry-run", false, "Perform a dry-run (don't actually delete)")
	set.Uint64Var(&cmd.minAge, "min-age", 6*30,
		"Minimum module age to remove from remote, in days (default: 6 months)")
	return &ffcli.Command{
		Name:       "clean",
		ShortUsage: "clean [flags]",
		ShortHelp:  "Remove unreferenced files in the module store",
		FlagSet:    set,
		Exec:       cmd.exec,
	}
}

func (cmd *cleanCmd) exec(context.Context, []string) error {
	referenced, err := collectReferencedIDs()
	if err != nil {
		return errors.New("failed to collect referenced IDs")
	}

	for _, task := range []struct {
		enabled bool
		fn      func(libpf.Set[modulestore.ID]) error
	}{
		{cmd.temp, cmd.cleanTemp},
		{cmd.local, cmd.cleanLocal},
		{cmd.remote, cmd.cleanRemote},
	} {
		if task.enabled {
			if err := task.fn(referenced); err != nil {
				return err
			}
		}
	}

	return nil
}

func (cmd *cleanCmd) cleanTemp(libpf.Set[modulestore.ID]) error {
	if err := cmd.store.RemoveLocalTempFiles(); err != nil {
		return fmt.Errorf("failed to delete temp files: %w", err)
	}
	return nil
}

func (cmd *cleanCmd) cleanLocal(referenced libpf.Set[modulestore.ID]) error {
	localModules, err := cmd.store.ListLocalModules()
	if err != nil {
		return fmt.Errorf("failed to read local cache contents: %w", err)
	}

	for module := range localModules {
		if _, exists := referenced[module]; exists {
			continue
		}

		log.Infof("Removing local module `%s`", module.String())
		if !cmd.dry {
			if err := cmd.store.RemoveLocalModule(module); err != nil {
				return fmt.Errorf("failed to delete module: %w", err)
			}
		}
	}

	return nil
}

func (cmd *cleanCmd) cleanRemote(referenced libpf.Set[modulestore.ID]) error {
	remoteModules, err := cmd.store.ListRemoteModules()
	if err != nil {
		return fmt.Errorf("failed to receive remote module list: %w", err)
	}

	for module, lastChanged := range remoteModules {
		if _, exists := referenced[module]; exists {
			continue
		}
		if time.Since(lastChanged) < time.Duration(cmd.minAge)*24*time.Hour {
			// In order to prevent us from accidentally deleting modules uploaded for tests
			// proposed on other branches (but not yet merged with the current branch), we check
			// whether the module was recently uploaded before deleting it.
			log.Infof("Module `%s` is unreferenced, but was uploaded recently (%s). Skipping.",
				module.String(), lastChanged)
			continue
		}

		log.Infof("Deleting unreferenced module `%s` (uploaded: %s)", module.String(), lastChanged)
		if !cmd.dry {
			if err = cmd.store.RemoveRemoteModule(module); err != nil {
				return fmt.Errorf("failed to delete remote module: %w", err)
			}
		}
	}

	return nil
}

// collectReferencedIDs gathers a set of all modules referenced from all testcases.
func collectReferencedIDs() (libpf.Set[modulestore.ID], error) {
	cases, err := findTestCases(false)
	if err != nil {
		return nil, fmt.Errorf("failed to find test cases: %w", err)
	}

	referenced := libpf.Set[modulestore.ID]{}
	for _, path := range cases {
		test, err := readTestCase(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read test case: %w", err)
		}

		referenced[test.CoredumpRef] = libpf.Void{}
		for _, module := range test.Modules {
			referenced[module.Ref] = libpf.Void{}
		}
	}

	return referenced, nil
}
