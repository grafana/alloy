//go:build linux && arm64

package asprof

import (
	_ "embed"
)

//go:embed async-profiler-4.0-87b7b42-linux-arm64.tar.gz
var embeddedArchiveData []byte

// bin/asprof
// lib/libasyncProfiler.so
