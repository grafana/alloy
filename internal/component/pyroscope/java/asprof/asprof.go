//go:build (linux || darwin) && (amd64 || arm64)

package asprof

import (
	"bytes"
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var fsMutex sync.Mutex

type Distribution struct {
	extractedDir string
}

func NewExtractedDistribution(extractedDir string) (Distribution, error) {
	d := Distribution{extractedDir: extractedDir}
	if _, err := os.Stat(d.LauncherPath()); err != nil {
		return d, fmt.Errorf("asprof launcher not found: %w", err)
	}
	if _, err := os.Stat(d.LibPath()); err != nil {
		return d, fmt.Errorf("asprof lib not found: %w", err)
	}
	return d, nil
}

func (d Distribution) LauncherPath() string {
	return filepath.Join(d.extractedDir, "bin/asprof")
}

type Archive struct {
	data   []byte
	format int
}

func (a *Archive) sha1() string {
	sum := sha1.Sum(a.data)
	return hex.EncodeToString(sum[:])
}

func (a *Archive) DistName() string {
	return fmt.Sprintf("alloy-asprof-%s", a.sha1())
}

const (
	ArchiveFormatTarGz = iota
	ArchiveFormatZip
)

func (d Distribution) Execute(argv []string) (string, string, error) {
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	exe := d.LauncherPath()
	cmd := exec.Command(exe, argv...)

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Start()
	if err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("asprof failed to start %s: %w", exe, err)
	}
	err = cmd.Wait()
	if err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("asprof failed to run %s: %w", exe, err)
	}
	return stdout.String(), stderr.String(), nil
}

func ExtractDistribution(a Archive, tmpDir, distName string) (Distribution, error) {
	d := Distribution{}
	fsMutex.Lock()
	defer fsMutex.Unlock()

	var launcher, lib []byte
	err := readArchive(a.data, a.format, func(name string, fi fs.FileInfo, data []byte) error {
		if strings.HasSuffix(name, "/asprof") {
			launcher = data
		}
		if strings.Contains(name, "libasyncProfiler") {
			lib = data
		}
		return nil
	})
	if err != nil {
		return d, err
	}
	if launcher == nil || lib == nil {
		return d, fmt.Errorf("failed to find libasyncProfiler in archive %s", distName)
	}

	fileMap := map[string][]byte{}
	fileMap[filepath.Join(distName, d.LauncherPath())] = launcher
	fileMap[filepath.Join(distName, d.LibPath())] = lib
	tmpDirFile, err := os.Open(tmpDir)
	if err != nil {
		return d, fmt.Errorf("failed to open tmp dir %s: %w", tmpDir, err)
	}
	defer tmpDirFile.Close()

	if err = checkTempDirPermissions(tmpDirFile); err != nil {
		return d, err
	}

	for path, data := range fileMap {
		if err = writeFile(tmpDirFile, path, data, true); err != nil {
			return d, err
		}
	}
	d.extractedDir = filepath.Join(tmpDir, distName)
	return d, nil
}
