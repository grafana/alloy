//go:build linux && (amd64 || arm64)

package asprof

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var EmbeddedArchive = Archive{data: embeddedArchiveData, format: ArchiveFormatTarGz}

func (d Distribution) LibPath() string {
	return filepath.Join(d.extractedDir, "lib/libasyncProfiler.so")
}

func (d Distribution) CopyLib(pid int) error {
	fsMutex.Lock()
	defer fsMutex.Unlock()
	libData, err := os.ReadFile(d.LibPath())
	if err != nil {
		return err
	}
	launcherData, err := os.ReadFile(d.LauncherPath())
	if err != nil {
		return err
	}
	procRoot := ProcessPath("/", pid)
	procRootFile, err := os.Open(procRoot)
	if err != nil {
		return fmt.Errorf("failed to open proc root %s: %w", procRoot, err)
	}
	defer procRootFile.Close()
	dstLibPath := strings.TrimPrefix(d.LibPath(), "/")
	dstLauncherPath := strings.TrimPrefix(d.LauncherPath(), "/")
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
	return filepath.Join("/proc", strconv.Itoa(pid), "root", path)
}
