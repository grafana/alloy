//go:build darwin

package asprof

import (
	_ "embed"
	"path/filepath"
)

//go:embed async-profiler-4.3-macos.zip
var embeddedArchiveData []byte

// bin/asprof
// lib/libasyncProfiler.dylib

var EmbeddedArchive = Archive{data: embeddedArchiveData, format: ArchiveFormatZip}

func (d Distribution) LibPath() string {
	return filepath.Join(d.extractedDir, "lib/libasyncProfiler.dylib")
}

func (d Distribution) CopyLib(pid int) error {
	return nil
}

func ProcessPath(path string, pid int) string {
	return path
}
