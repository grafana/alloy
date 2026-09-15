package gather

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/pprof"
)

// PprofSnapshot collects point-in-time runtime profiles.
type PprofSnapshot struct{}

func (PprofSnapshot) Name() string { return "pprof-snapshot" }

func (PprofSnapshot) Gather(_ context.Context, _ Options) ([]File, error) {
	var files []File

	heap, err := lookupProfile("heap")
	if err != nil {
		return files, err
	}
	files = append(files, File{Path: "pprof/heap.pprof", Content: heap})

	goroutine, err := lookupProfile("goroutine")
	if err != nil {
		return files, err
	}
	files = append(files, File{Path: "pprof/goroutine.pprof", Content: goroutine})

	return files, nil
}

// mutexProfileRate is the sample rate the extension sets for the mutex profile
// during the window. The runtime returns the previous rate, so it is restored.
const mutexProfileRate = 5

// PprofWindow collects the CPU, mutex, and block profiles. It enables mutex
// sampling for the window and restores the previous rate at finish. It does NOT
// change the block profile rate: the runtime cannot report the current rate, so
// the extension cannot restore it. The block profile therefore reflects whatever
// block profiling the operator configured (for example through the
// pprofextension). The CPU profile runs only when the window is non-zero.
type PprofWindow struct{}

func (PprofWindow) Name() string { return "pprof-window" }

func (PprofWindow) Start(_ context.Context, opts Options) (FinishFunc, error) {
	// Enable mutex sampling for the window. Save the previous rate to restore it.
	prevMutex := runtime.SetMutexProfileFraction(mutexProfileRate)

	// The CPU profile needs a window. Skip it for a zero window. If CPU profiling
	// is already in use elsewhere (for example the pprof extension), StartCPUProfile
	// fails; record the error but still collect the mutex and block profiles.
	var cpuBuf bytes.Buffer
	var cpuErr error
	cpuStarted := false
	if opts.Duration > 0 {
		if err := pprof.StartCPUProfile(&cpuBuf); err != nil {
			cpuErr = fmt.Errorf("cpu profile: %w", err)
		} else {
			cpuStarted = true
		}
	}

	finish := func(_ context.Context) ([]File, error) {
		var files []File

		if cpuStarted {
			// StopCPUProfile flushes the samples to the buffer. Call it first.
			pprof.StopCPUProfile()
			files = append(files, File{Path: "pprof/cpu.pprof", Content: cpuBuf.Bytes()})
		}

		// Restore the mutex sampling rate.
		runtime.SetMutexProfileFraction(prevMutex)

		errs := []error{cpuErr}

		mutex, err := lookupProfile("mutex")
		if err != nil {
			errs = append(errs, err)
		} else {
			files = append(files, File{Path: "pprof/mutex.pprof", Content: mutex})
		}

		// Read the block profile as-is. The extension does not change the block
		// rate, so this reflects the operator's configuration.
		block, err := lookupProfile("block")
		if err != nil {
			errs = append(errs, err)
		} else {
			files = append(files, File{Path: "pprof/block.pprof", Content: block})
		}

		return files, errors.Join(errs...)
	}

	return finish, nil
}

// lookupProfile writes a named profile to a buffer.
func lookupProfile(name string) ([]byte, error) {
	var buf bytes.Buffer
	p := pprof.Lookup(name)
	if err := p.WriteTo(&buf, 0); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
