//go:build unix

package reporter

import (
	"bytes"
	"testing"

	"github.com/google/pprof/profile"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/symb/irsymcache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/reporter/samples"
	"go.opentelemetry.io/ebpf-profiler/support"
)

func singleFrameTrace(ty libpf.FrameType, mappingFile libpf.FrameMappingFile, lineno libpf.AddressOrLineno, funcName, sourceFile string, sourceLine libpf.SourceLineno) libpf.Frames {
	frames := make(libpf.Frames, 0, 1)
	var mapping libpf.FrameMapping
	if mappingFile != (libpf.FrameMappingFile{}) {
		mapping = libpf.NewFrameMapping(libpf.FrameMappingData{
			File: mappingFile,
		})
	}
	frames.Append(&libpf.Frame{
		Type:            ty,
		AddressOrLineno: lineno,
		FunctionName:    libpf.Intern(funcName),
		SourceFile:      libpf.Intern(sourceFile),
		SourceLine:      sourceLine,
		Mapping:         mapping,
	})
	return frames
}

func newReporter() *PPROFReporter {
	tp := discovery.NewTargetProducer(discovery.TargetsOptions{
		Targets: []discovery.DiscoveredTarget{
			{
				"__process_pid__": "123",
				"service_name":    "service_a",
			},
		},
	})
	return NewPPROF(
		nil,
		&Config{
			SamplesPerSecond: 97,
		},
		tp,
		nil,
		nil,
	)
}

func TestPPROFReporter_StringAndFunctionTablePopulation(t *testing.T) {
	rep := newReporter()

	funcName := "myfunc"
	filePath := "/bin/bar"
	mappingFile := libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
		FileID:   libpf.NewFileID(7, 8),
		FileName: libpf.Intern(filePath),
	})

	traceKey := samples.TraceAndMetaKey{
		Pid: 123,
	}
	events := samples.KeyToEventMapping{
		traceKey: &samples.TraceEvents{
			Frames: singleFrameTrace(libpf.PythonFrame, mappingFile, 0x30,
				funcName, filePath, 1234),
			Timestamps: []uint64{42},
		},
	}

	profiles := rep.createProfile(
		libpf.Intern(""),
		support.TraceOriginSampling,
		events,
	)
	require.Len(t, profiles, 1)
	assert.Equal(t, "service_a", profiles[0].Labels.Get("service_name"))

	p, err := profile.Parse(bytes.NewReader(profiles[0].Raw))
	require.NoError(t, err)

	p.TimeNanos = 0
	expected := `PeriodType: cpu nanoseconds
Period: 10309278
Samples:
cpu/nanoseconds
   10309278: 1 
Locations
     1: 0x30 M=2 myfunc /bin/bar:1234:0 s=0()
Mappings
1: 0x0/0x0/0x0   
2: 0x0/0x0/0x0 /bin/bar  [FN][LN]
`
	assert.Equal(t, expected, p.String())
}

func singleFrameNative(mappingFile libpf.FrameMappingFile, lineno libpf.AddressOrLineno, mappingStart, mappingEnd libpf.Address, mappingFileOffset uint64) libpf.Frames {
	frames := make(libpf.Frames, 0, 2)
	frames.Append(&libpf.Frame{
		Type:            libpf.NativeFrame,
		AddressOrLineno: lineno,
		Mapping: libpf.NewFrameMapping(libpf.FrameMappingData{
			File:       mappingFile,
			Start:      mappingStart,
			End:        mappingEnd,
			FileOffset: mappingFileOffset,
		}),
	})
	return frames
}

func TestPPROFReporter_NativeFrame(t *testing.T) {
	rep := newReporter()

	filePath := "/usr/lib/libexample.so"
	mappingFile := libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
		FileID:   libpf.NewFileID(9, 10),
		FileName: libpf.Intern(filePath),
	})

	traceKey := samples.TraceAndMetaKey{
		Pid: 123,
	}
	events := samples.KeyToEventMapping{
		traceKey: &samples.TraceEvents{
			Frames:     singleFrameNative(mappingFile, 0x1000, 0x1000, 0x2000, 0x100),
			Timestamps: []uint64{789},
		},
	}
	profiles := rep.createProfile(
		libpf.Intern(""),
		support.TraceOriginSampling,
		events,
	)
	require.Len(t, profiles, 1)
	assert.Equal(t, "service_a", profiles[0].Labels.Get("service_name"))
	p, err := profile.Parse(bytes.NewReader(profiles[0].Raw))
	require.NoError(t, err)

	p.TimeNanos = 0
	expected := `PeriodType: cpu nanoseconds
Period: 10309278
Samples:
cpu/nanoseconds
   10309278: 1 
Locations
     1: 0x1000 M=2 
Mappings
1: 0x0/0x0/0x0   
2: 0x1000/0x2000/0x100 /usr/lib/libexample.so  
`
	assert.Equal(t, expected, p.String())
}

func TestPPROFReporter_WithoutMapping(t *testing.T) {
	rep := newReporter()

	frames := make(libpf.Frames, 0, 1)
	frames.Append(&libpf.Frame{
		Type:            libpf.KernelFrame,
		AddressOrLineno: 0x2000,
	})

	traceKey := samples.TraceAndMetaKey{
		Pid: 123,
	}
	events := samples.KeyToEventMapping{
		traceKey: &samples.TraceEvents{
			Frames:     frames,
			Timestamps: []uint64{42},
		},
	}

	profiles := rep.createProfile(
		libpf.Intern(""),
		support.TraceOriginSampling,
		events,
	)
	require.Len(t, profiles, 1)
	assert.Equal(t, "service_a", profiles[0].Labels.Get("service_name"))

	p, err := profile.Parse(bytes.NewReader(profiles[0].Raw))
	require.NoError(t, err)

	p.TimeNanos = 0
	expected := `PeriodType: cpu nanoseconds
Period: 10309278
Samples:
cpu/nanoseconds
   10309278: 1 
Locations
     1: 0x2000 M=1 
Mappings
1: 0x0/0x0/0x0   
`
	assert.Equal(t, expected, p.String())
}

func TestPPROFReporter_Bug(t *testing.T) {
	rep := newReporter()

	frames := make(libpf.Frames, 0, 1)
	frames.Append(&libpf.Frame{
		Type:            libpf.KernelFrame,
		AddressOrLineno: 0x2000,
	})
	frames.Append(&libpf.Frame{
		Type:         libpf.PythonFrame,
		FunctionName: libpf.Intern("f1"),
		SourceLine:   42,
	})
	frames.Append(&libpf.Frame{
		Type:         libpf.PythonFrame,
		FunctionName: libpf.Intern("f2"),
		SourceLine:   239,
	})
	frames.Append(&libpf.Frame{
		Type:         libpf.PythonFrame,
		FunctionName: libpf.Intern("f2"),
		SourceLine:   240,
	})

	traceKey := samples.TraceAndMetaKey{
		Pid: 123,
	}
	events := samples.KeyToEventMapping{
		traceKey: &samples.TraceEvents{
			Frames:     frames,
			Timestamps: []uint64{42},
		},
	}

	profiles := rep.createProfile(
		libpf.Intern(""),
		support.TraceOriginSampling,
		events,
	)
	require.Len(t, profiles, 1)
	assert.Equal(t, "service_a", profiles[0].Labels.Get("service_name"))

	p, err := profile.Parse(bytes.NewReader(profiles[0].Raw))
	require.NoError(t, err)

	p.TimeNanos = 0
	expected := `PeriodType: cpu nanoseconds
Period: 10309278
Samples:
cpu/nanoseconds
   10309278: 1 2 3 4 
Locations
     1: 0x2000 M=1 
     2: 0x0 M=1 f1 :42:0 s=0()
     3: 0x0 M=1 f2 :239:0 s=0()
     4: 0x0 M=1 f2 :240:0 s=0()
Mappings
1: 0x0/0x0/0x0   [FN][LN]
`
	assert.Equal(t, expected, p.String())
}

func TestPPROFReporter_Demangle(t *testing.T) {
	fid := libpf.NewFileID(7, 13)
	key := symbolizerKey{
		fid:  fid,
		addr: 0xcafe00de,
	}
	rep := newReporter()
	rep.symbols = &symbolizer{
		symbols: map[symbolizerKey]irsymcache.SourceInfo{
			key: {
				LineNumber:   9,
				FunctionName: libpf.Intern("_ZN15PlatformMonitor4waitEm"),
			},
		},
	}
	rep.cfg.Demangle = "full"

	frames := make(libpf.Frames, 0, 1)
	frames.Append(&libpf.Frame{
		Type:            libpf.KernelFrame,
		AddressOrLineno: 0x2000,
	})
	frames.Append(&libpf.Frame{ // a native frame without a valid mapping should not be symbolized
		Type:            libpf.NativeFrame,
		AddressOrLineno: 0xface000,
	})
	frames.Append(&libpf.Frame{ // a native frame with a mapping, already symbolized, should not be symbolized again
		Type:            libpf.NativeFrame,
		FunctionName:    libpf.Intern("_ZN18ConcurrentGCThread3runEv"),
		AddressOrLineno: 0xcafe00ef,
		Mapping: libpf.NewFrameMapping(libpf.FrameMappingData{
			File: libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
				FileID:   fid,
				FileName: libpf.Intern("libfoo.so"),
			}),
			Start: 0xcafe0000,
			End:   0xcafe1000,
		}),
	})
	frames.Append(&libpf.Frame{ // a native frame with a mapping should be symbolized
		Type:            libpf.NativeFrame,
		FunctionName:    libpf.NullString,
		AddressOrLineno: 0xcafe00de,
		Mapping: libpf.NewFrameMapping(libpf.FrameMappingData{
			File: libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
				FileID:   fid,
				FileName: libpf.Intern("libfoo.so"),
			}),
			Start: 0xcafe0000,
			End:   0xcafe1000,
		}),
	})

	traceKey := samples.TraceAndMetaKey{
		Pid: 123,
	}
	events := samples.KeyToEventMapping{
		traceKey: &samples.TraceEvents{
			Frames:     frames,
			Timestamps: []uint64{42},
		},
	}

	profiles := rep.createProfile(
		libpf.Intern(""),
		support.TraceOriginSampling,
		events,
	)
	require.Len(t, profiles, 1)
	assert.Equal(t, "service_a", profiles[0].Labels.Get("service_name"))

	p, err := profile.Parse(bytes.NewReader(profiles[0].Raw))
	require.NoError(t, err)

	p.TimeNanos = 0
	expected := `PeriodType: cpu nanoseconds
Period: 10309278
Samples:
cpu/nanoseconds
   10309278: 1 2 3 4 
Locations
     1: 0x2000 M=1 
     2: 0xface000 M=1 
     3: 0xcafe00ef M=2 ConcurrentGCThread::run() :0:0 s=0()
     4: 0xcafe00de M=2 PlatformMonitor::wait(unsigned long) :0:0 s=0()
Mappings
1: 0x0/0x0/0x0   
2: 0xcafe0000/0xcafe1000/0x0 libfoo.so  [FN]
`
	assert.Equal(t, expected, p.String())
}

func TestPPROFReporter_UnsymbolizedStub(t *testing.T) {
	rep := newReporter()
	rep.symbols = &symbolizer{}
	rep.cfg.ReporterUnsymbolizedStubs = true

	frames := make(libpf.Frames, 0, 1)
	frames.Append(&libpf.Frame{
		Type:            libpf.KernelFrame,
		AddressOrLineno: 0x2000,
	})
	frames.Append(&libpf.Frame{
		Type:            libpf.NativeFrame,
		AddressOrLineno: 0xface000,
	})
	frames.Append(&libpf.Frame{
		Type:            libpf.NativeFrame,
		AddressOrLineno: 0xcafe00ef,
		Mapping: libpf.NewFrameMapping(libpf.FrameMappingData{
			File: libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
				FileID:   libpf.NewFileID(7, 13),
				FileName: libpf.Intern("libfoo.so"),
			}),
			Start: 0xcafe0000,
			End:   0xcafe1000,
		}),
	})

	traceKey := samples.TraceAndMetaKey{
		Pid: 123,
	}
	events := samples.KeyToEventMapping{
		traceKey: &samples.TraceEvents{
			Frames:     frames,
			Timestamps: []uint64{42},
		},
	}

	profiles := rep.createProfile(
		libpf.Intern(""),
		support.TraceOriginSampling,
		events,
	)
	require.Len(t, profiles, 1)
	assert.Equal(t, "service_a", profiles[0].Labels.Get("service_name"))

	p, err := profile.Parse(bytes.NewReader(profiles[0].Raw))
	require.NoError(t, err)

	p.TimeNanos = 0
	expected := `PeriodType: cpu nanoseconds
Period: 10309278
Samples:
cpu/nanoseconds
   10309278: 1 2 3 
Locations
     1: 0x2000 M=1 
     2: 0xface000 M=1 
     3: 0xcafe00ef M=2 $ libfoo.so + 0xcafe00ef :0:0 s=0()
Mappings
1: 0x0/0x0/0x0   
2: 0xcafe0000/0xcafe1000/0x0 libfoo.so  [FN]
`
	assert.Equal(t, expected, p.String())
}

type symbolizer struct {
	symbols map[symbolizerKey]irsymcache.SourceInfo
}

type symbolizerKey struct {
	fid  libpf.FileID
	addr uint64
}

func (s symbolizer) ExecutableKnown(id libpf.FileID) bool {
	return true
}

func (s symbolizer) ObserveExecutable(id libpf.FileID, ref *pfelf.Reference) error {
	return nil
}

func (s symbolizer) ResolveAddress(file libpf.FileID, addr uint64) (irsymcache.SourceInfo, error) {
	return s.symbols[symbolizerKey{fid: file, addr: addr}], nil
}

func (s symbolizer) Cleanup() {

}
