//go:build linux && (arm64 || amd64)

package reporter

import (
	"bytes"
	"testing"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/reporter/samples"
	"go.opentelemetry.io/ebpf-profiler/support"

	discovery "go.opentelemetry.io/ebpf-profiler/pyroscope/discovery"
)

func singleFrameTrace(ty libpf.FrameType, mappingFile libpf.FrameMappingFile, lineno libpf.AddressOrLineno, funcName, sourceFile string, sourceLine libpf.SourceLineno) libpf.Frames {
	frames := make(libpf.Frames, 0, 1)
	frames.Append(&libpf.Frame{
		Type:            ty,
		AddressOrLineno: lineno,
		FunctionName:    libpf.Intern(funcName),
		SourceFile:      libpf.Intern(sourceFile),
		SourceLine:      sourceLine,
		MappingFile:     mappingFile,
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
		Type:              libpf.NativeFrame,
		AddressOrLineno:   lineno,
		MappingStart:      mappingStart,
		MappingEnd:        mappingEnd,
		MappingFileOffset: mappingFileOffset,
		MappingFile:       mappingFile,
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
