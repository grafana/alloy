//go:build linux && amd64

package asprof

import (
	_ "embed"
)

//go:embed async-profiler-4.1-5930966-linux-x64.tar.gz
var embeddedArchiveData []byte

// bin/asprof
// lib/libasyncProfiler.so
