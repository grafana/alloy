//go:build darwin

package asprof

import (
	_ "embed"
	"path/filepath"
)

//go:embed async-profiler-3.0-fa937db-macos.zip
var embeddedArchiveData []byte

// bin/asprof
// lib/libasyncProfiler.dylib

var embeddedArchiveVersion = 300

var EmbeddedArchive = Archive{data: embeddedArchiveData, version: embeddedArchiveVersion, format: ArchiveFormatZip}

func (d *Distribution) LibPath() string {
	return filepath.Join(d.extractedDir, "lib/libasyncProfiler.dylib")
}

func (p *Profiler) CopyLib(dist *Distribution, pid int) error {
	return nil
}

func ProcessPath(path string, pid int) string {
	return path
}
