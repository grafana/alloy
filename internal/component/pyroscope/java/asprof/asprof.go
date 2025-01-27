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
	version      int
}

func (d *Distribution) LauncherPath() string {
	return filepath.Join(d.extractedDir, "bin/asprof")
}

type Profiler struct {
	tmpDir       string
	extractOnce  sync.Once
	dist         *Distribution
	extractError error
	tmpDirMarker any
	archiveHash  string
	archive      Archive
}

type Archive struct {
	data    []byte
	version int
	format  int
}

const (
	ArchiveFormatTarGz = iota
	ArchiveFormatZip
)

func NewProfiler(tmpDir string, archive Archive) *Profiler {
	res := &Profiler{tmpDir: tmpDir, dist: new(Distribution), tmpDirMarker: "alloy-asprof"}
	sum := sha1.Sum(archive.data)
	hexSum := hex.EncodeToString(sum[:])
	res.archiveHash = hexSum
	res.dist.version = archive.version
	res.archive = archive
	return res
}

func (p *Profiler) Execute(dist *Distribution, argv []string) (string, string, error) {
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	exe := dist.LauncherPath()
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

func (p *Profiler) Distribution() *Distribution {
	return p.dist
}

func (p *Profiler) ExtractDistributions() error {
	p.extractOnce.Do(func() {
		p.extractError = p.extractDistributions()
	})
	return p.extractError
}

func (p *Profiler) extractDistributions() error {
	fsMutex.Lock()
	defer fsMutex.Unlock()
	distName := p.getDistName()

	var launcher, dist []byte
	err := readArchive(p.archive.data, p.archive.format, func(name string, fi fs.FileInfo, data []byte) error {
		if strings.Contains(name, "asprof") {
			launcher = data
		}
		if strings.Contains(name, "libasyncProfiler") {
			dist = data
		}
		return nil
	})
	if err != nil {
		return err
	}
	if launcher == nil || dist == nil {
		return fmt.Errorf("failed to find libasyncProfiler in archive %s", distName)
	}

	fileMap := map[string][]byte{}
	fileMap[filepath.Join(distName, p.dist.LauncherPath())] = launcher
	fileMap[filepath.Join(distName, p.dist.LibPath())] = dist
	tmpDirFile, err := os.Open(p.tmpDir)
	if err != nil {
		return fmt.Errorf("failed to open tmp dir %s: %w", p.tmpDir, err)
	}
	defer tmpDirFile.Close()

	if err = checkTempDirPermissions(tmpDirFile); err != nil {
		return err
	}

	for path, data := range fileMap {
		if err = writeFile(tmpDirFile, path, data, true); err != nil {
			return err
		}
	}
	p.dist.extractedDir = filepath.Join(p.tmpDir, distName)
	return nil
}

func (p *Profiler) getDistName() string {
	return fmt.Sprintf("%s-%s", p.tmpDirMarker, p.archiveHash)
}
