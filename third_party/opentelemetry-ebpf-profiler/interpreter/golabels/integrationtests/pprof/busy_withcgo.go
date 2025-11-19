//go:build withcgo && linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprof // import "go.opentelemetry.io/ebpf-profiler/interpreter/golabels/integrationtests/pprof"

/*
#include <stdio.h>

void cgofunc() {
	volatile int counter = 0;
	while (counter < 1000000) {
		counter++;
	}
}
*/
import "C"

//go:noinline
func busyFunc() {
	C.cgofunc()
}
