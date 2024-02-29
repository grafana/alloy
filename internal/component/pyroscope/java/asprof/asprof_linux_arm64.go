//go:build linux && arm64

package asprof

import (
	_ "embed"
)

//go:embed async-profiler-3.0-linux-arm64.tar.gz
var embededArchiveData []byte

// asprof
// glibc / libasyncProfiler.so
// musl / libasyncProfiler.so

var embededArchiveVersion = 300

var EmbeddedArchive = Archive{data: embededArchiveData, version: embededArchiveVersion}
