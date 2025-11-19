package python // import "go.opentelemetry.io/ebpf-profiler/interpreter/python"

import "sync/atomic"

var NoContinueWithNextUnwinder = atomic.Bool{}

func continueWithNextUnwinder() int {
	res := 1
	if NoContinueWithNextUnwinder.Load() {
		res = 0
	}
	return res
}
