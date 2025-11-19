package golang

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/util"
)

func BenchmarkGolang(b *testing.B) {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		b.Fatal("Failed to get PC from runtime")
	}
	fn := runtime.FuncForPC(pc)
	exec, err := os.Executable()
	if err != nil {
		b.Fatalf("Failed to get the executable: %v", err)
	}

	libpfPID := libpf.PID(os.Getpid())
	pid := process.New(libpfPID, libpfPID)

	elfRef := pfelf.NewReference(exec, pid)
	hostFileID, err := host.FileIDFromBytes([]byte{0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55})
	if err != nil {
		b.Fatalf("Failed to create hostID: %v", err)
	}
	loaderInfo := interpreter.NewLoaderInfo(hostFileID, elfRef, []util.Range{})
	rm := remotememory.NewProcessVirtualMemory(libpfPID)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		gD, err := Loader(nil, loaderInfo)
		if err != nil {
			b.Fatalf("Failed to create loader: %v", err)
		}

		gI, err := gD.Attach(nil, libpfPID, 0x0, rm)
		if err != nil {
			b.Fatalf("Failed to create instance: %v", err)
		}

		frames := libpf.Frames{}

		if err := gI.Symbolize(&host.Frame{
			File:   hostFileID,
			Lineno: libpf.AddressOrLineno(pc),
			Type:   libpf.FrameType(libpf.Native),
		}, &frames); err != nil {
			b.Fatalf("Failed to symbolize 0x%x: %v", pc, err)
		}

		if len(frames) != 1 {
			b.Fatalf("Expected a single entry but got %d", len(frames))
		}
		for _, uniqueFrame := range frames {
			f := uniqueFrame.Value()
			// The returned anonymous function has the suffic 'func1'.
			// Therefore check only for a matching prefix.
			if !strings.HasPrefix(f.FunctionName.String(), fn.Name()) {
				b.Fatalf("Expected '%s()' but got '%s()'", fn, f.FunctionName)
			}
		}
	}
}
