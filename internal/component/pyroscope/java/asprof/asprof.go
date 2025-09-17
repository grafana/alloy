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

func NewExtractedDistribution(extractedDir string) *Distribution {
	return &Distribution{extractedDir: extractedDir}
}

func (d *Distribution) LauncherPath() string {
	return filepath.Join(d.extractedDir, "bin/asprof")
}

type Profiler struct {
	//tmpDir      string
	//extractOnce sync.Once
	dist *Distribution
	//extractError error
	//tmpDirMarker string
	//archiveHash  string
	//archive      Archive
}

type Archive struct {
	data   []byte
	format int
}

const (
	ArchiveFormatTarGz = iota
	ArchiveFormatZip
)

func NewProfiler(d *Distribution) *Profiler {
	return &Profiler{dist: d}
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

//func (p *Profiler) ExtractDistributions() error {
//	p.extractOnce.Do(func() {
//		p.extractError = p.extractDistributions()
//	})
//	return p.extractError
//}

const tmpDirMarker = "alloy-asprof"

func ExtractDistribution(a Archive, tmpDir string) (*Distribution, error) {
	sum := sha1.Sum(a.data)
	archiveHash := hex.EncodeToString(sum[:])
	distName := fmt.Sprintf("%s-%s", tmpDirMarker, archiveHash)

	d := new(Distribution)
	fsMutex.Lock()
	defer fsMutex.Unlock()

	var launcher, lib []byte
	err := readArchive(a.data, a.format, func(name string, fi fs.FileInfo, data []byte) error {
		if strings.Contains(name, "asprof") {
			launcher = data
		}
		if strings.Contains(name, "libasyncProfiler") {
			lib = data
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if launcher == nil || lib == nil {
		return nil, fmt.Errorf("failed to find libasyncProfiler in archive %s", distName)
	}

	fileMap := map[string][]byte{}
	fileMap[filepath.Join(distName, d.LauncherPath())] = launcher
	fileMap[filepath.Join(distName, d.LibPath())] = lib
	tmpDirFile, err := os.Open(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open tmp dir %s: %w", tmpDir, err)
	}
	defer tmpDirFile.Close()

	if err = checkTempDirPermissions(tmpDirFile); err != nil {
		return nil, err
	}

	for path, data := range fileMap {
		if err = writeFile(tmpDirFile, path, data, true); err != nil {
			return nil, err
		}
	}
	d.extractedDir = filepath.Join(tmpDir, distName)
	return d, nil
}
