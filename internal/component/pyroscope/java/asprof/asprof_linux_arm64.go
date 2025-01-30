//go:build linux && arm64

package asprof

import (
	_ "embed"
)

//go:embed async-profiler-3.0-fa937db-linux-arm64.tar.gz
var embeddedArchiveData []byte

// bin/asprof
// lib/libasyncProfiler.so
