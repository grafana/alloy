// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"debug/elf"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"
)

type gdbCmd struct {
	store *modulestore.Store

	casePath    string
	keep        bool
	extractOnly bool
}

func newGdbCmd(store *modulestore.Store) *ffcli.Command {
	args := &gdbCmd{store: store}

	set := flag.NewFlagSet("gdb", flag.ExitOnError)
	set.StringVar(&args.casePath, "case", "", "Path of the test case to debug")
	set.BoolVar(&args.keep, "keep", false, "Keep sysroot after gdb exits")
	set.BoolVar(&args.extractOnly, "extract-only", false,
		"Don't invoke gdb, just create sysroot. Implies -keep.")

	return &ffcli.Command{
		Name:       "gdb",
		Exec:       args.exec,
		ShortUsage: "gdb [flags]",
		ShortHelp:  "Open coredump in gdb",
		FlagSet:    set,
	}
}

const sysrootBaseDir = "gdb-sysroot"

func (cmd *gdbCmd) exec(context.Context, []string) (err error) {
	// Validate arguments.
	if cmd.casePath == "" {
		return errors.New("please specify `-case`")
	}

	var test *CoredumpTestCase
	test, err = readTestCase(cmd.casePath)
	if err != nil {
		return fmt.Errorf("failed to read test case: %w", err)
	}

	sysroot := path.Join(sysrootBaseDir, path.Base(cmd.casePath))
	if err = os.MkdirAll(sysroot, 0o755); err != nil {
		return fmt.Errorf("failed to create sysroot directory: %v", err)
	}

	// Inject module info for the coredump itself.
	coreModule := ModuleInfo{LocalPath: "/core", Ref: test.CoredumpRef}
	files := append([]ModuleInfo{coreModule}, test.Modules...)

	soNameMap := make(map[string]string) // SONAME -> sysroot fs path
	for _, file := range files {
		dest := path.Join(sysroot, strings.TrimPrefix(path.Clean(file.LocalPath), "/"))

		// Unpack file if not already done previously.
		if _, err = os.Stat(dest); err != nil {
			log.Infof("Unpacking %v", dest)
			if err = os.MkdirAll(path.Dir(dest), 0o755); err != nil {
				return fmt.Errorf("failed to create directory for %v: %v", dest, err)
			}

			if err = cmd.store.UnpackModuleToPath(file.Ref, dest); err != nil {
				return fmt.Errorf("failed to unpack module %v: %v", dest, err)
			}
		}

		// Read SONAME for symlink creation.
		soName := readElfSoName(dest)
		if soName != "" {
			soNameMap[soName] = dest
		}
	}

	// Read main executable path from coredump.
	cd, err := process.OpenCoredump(path.Join(sysroot, "core"))
	if err != nil {
		return fmt.Errorf("failed to inspect coredump: %v", err)
	}
	defer cd.Close()
	executable := cd.MainExecutable()
	if executable == "" {
		return errors.New("failed to find main executable")
	}

	// Unfortunately gdb doesn't use the mapping path and instead reads DSO
	// names to load from ELF .dynamic section, then tries resolving things in
	// a way similar to LD. DSOs are often behind multiple levels of symlinks,
	// and we don't include those in our test cases. We thus have to recreate
	// them according to what .dynamic section specifies to allow gdb to find
	// everything that it needs.
	symlinkRoot := path.Join(sysroot, "mapped")
	if err = os.MkdirAll(symlinkRoot, 0o755); err != nil {
		return fmt.Errorf("failed to create symlink dir: %v", err)
	}
	for soName, dsoPath := range soNameMap {
		var dsoAbsPath string
		dsoAbsPath, err = filepath.Abs(dsoPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for DSO: %v", err)
		}

		linkPath := path.Join(symlinkRoot, soName)
		if _, err = os.Stat(linkPath); err == nil {
			continue // assume exists
		}

		log.Infof("Mapping DSO %v -> %v", soName, dsoPath)
		if err = os.Symlink(dsoAbsPath, linkPath); err != nil {
			return fmt.Errorf("failed to symlink: %v", err)
		}
	}

	if cmd.extractOnly {
		return nil
	}

	if len(test.Modules) == 0 {
		log.Warn("Test-case doesn't bundle modules: gdb might not work well")
	}

	gdbBin, err := exec.LookPath("gdb-multiarch")
	if err != nil {
		log.Warn("No gdb-multiarch installed. Falling back to regular gdb.")
		gdbBin = "gdb"
	}

	gdb := exec.Command(gdbBin,
		path.Join(sysroot, executable),
		"-c", path.Join(sysroot, "core"),
		"-iex", "set solib-search-path "+symlinkRoot,
		"-iex", "set sysroot "+sysroot)

	gdb.Stdin = os.Stdin
	gdb.Stdout = os.Stdout
	gdb.Stderr = os.Stderr

	err = gdb.Run()

	if !cmd.keep {
		if err2 := os.RemoveAll(sysroot); err2 != nil {
			log.Errorf("Failed to remove sysroot: %v", err)
		}

		// Only unlink the base directory if it's empty. os.Remove won't
		// delete non-empty directories and error out in that case.
		_ = os.Remove(sysrootBaseDir)
	}

	return err
}

// readElfSoName reads DT_SONAME from a given DSO on disk.
func readElfSoName(dsoPath string) string {
	ef, err := pfelf.Open(dsoPath)
	if err != nil {
		log.Warnf("Failed to open ELF %v: %v", dsoPath, err)
		return ""
	}
	defer ef.Close()

	if ef.Type != elf.ET_DYN {
		return ""
	}

	var soName []string
	soName, err = ef.DynString(elf.DT_SONAME)
	if err != nil {
		log.Warnf("Failed to read DT_SONAME from %v: %v", dsoPath, err)
		return ""
	}
	if len(soName) == 0 {
		log.Warnf("DSO at %v doesn't specify an SONAME", dsoPath)
		return ""
	}

	return soName[0]
}
