//go:build linux && (amd64 || arm64)

package asprof

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"path/filepath"
)

var embeddedArchiveVersion = 300

var EmbeddedArchive = Archive{data: embeddedArchiveData, version: embeddedArchiveVersion, format: ArchiveFormatTarGz}

func (d *Distribution) LibPath() string {
	return filepath.Join(d.extractedDir, "lib/libasyncProfiler.so")
}

func (p *Profiler) CopyLib(dist *Distribution, pid int) error {
	fsMutex.Lock()
	defer fsMutex.Unlock()
	libData, err := os.ReadFile(dist.LibPath())
	if err != nil {
		return err
	}
	launcherData, err := os.ReadFile(dist.LauncherPath())
	if err != nil {
		return err
	}
	procRoot := ProcessPath("/", pid)
	procRootFile, err := os.Open(procRoot)
	if err != nil {
		return fmt.Errorf("failed to open proc root %s: %w", procRoot, err)
	}
	defer procRootFile.Close()
	dstLibPath := strings.TrimPrefix(dist.LibPath(), "/")
	dstLauncherPath := strings.TrimPrefix(dist.LauncherPath(), "/")
	if err = writeFile(procRootFile, dstLibPath, libData, false); err != nil {
		return err
	}
	// this is to create a bin directory, we don't actually need to write anything there, and we don't execute the launcher there
	if err = writeFile(procRootFile, dstLauncherPath, launcherData, false); err != nil {
		return err
	}
	return nil
}

func ProcessPath(path string, pid int) string {
	f := procFile{path, pid}
	return f.procRootPath()
}

type procFile struct {
	path string
	pid  int
}

func (f *procFile) procRootPath() string {
	return filepath.Join("/proc", strconv.Itoa(f.pid), "root", f.path)
}
