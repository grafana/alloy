package pdata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/reporter/internal/orderedset"
	"go.opentelemetry.io/ebpf-profiler/reporter/samples"
	"go.opentelemetry.io/ebpf-profiler/support"
)

func TestGetDummyMappingIndex(t *testing.T) {
	fileID := libpf.NewFileID(12345678, 12345678)
	for _, tt := range []struct {
		name       string
		mappingSet orderedset.OrderedSet[libpf.FileID]
		stringSet  orderedset.OrderedSet[string]
		fileID     libpf.FileID

		wantIndex        int32
		wantMappingSet   orderedset.OrderedSet[libpf.FileID]
		wantMappingTable []int32
		wantStringSet    orderedset.OrderedSet[string]
	}{
		{
			name: "with an index already in the file id mapping",
			mappingSet: orderedset.OrderedSet[libpf.FileID]{
				fileID: 42,
			},
			fileID:    fileID,
			wantIndex: 42,
			wantMappingSet: orderedset.OrderedSet[libpf.FileID]{
				fileID: 42,
			},
		},
		{
			name:       "with an index not yet in the file id mapping",
			mappingSet: orderedset.OrderedSet[libpf.FileID]{},
			stringSet:  orderedset.OrderedSet[string]{},
			fileID:     fileID,

			wantIndex: 0,
			wantMappingSet: orderedset.OrderedSet[libpf.FileID]{
				fileID: 0,
			},
			wantMappingTable: []int32{0},
			wantStringSet:    orderedset.OrderedSet[string]{"": 0, "process.executable.build_id.htlhash": 1},
		},
		{
			name: "with an index not yet in the file id mapping and a filename in the string table",

			mappingSet: orderedset.OrderedSet[libpf.FileID]{},
			stringSet:  orderedset.OrderedSet[string]{"": 42},
			fileID:     fileID,

			wantIndex: 0,
			wantMappingSet: orderedset.OrderedSet[libpf.FileID]{
				fileID: 0,
			},
			wantMappingTable: []int32{42},
			wantStringSet:    orderedset.OrderedSet[string]{"": 42, "process.executable.build_id.htlhash": 1},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mappingSet := tt.mappingSet
			stringSet := tt.stringSet
			dic := pprofile.NewProfilesDictionary()
			mgr := samples.NewAttrTableManager(stringSet, dic.AttributeTable())

			idx, exists := mappingSet.AddWithCheck(tt.fileID)
			if !exists {
				mapping := dic.MappingTable().AppendEmpty()
				mapping.SetFilenameStrindex(stringSet.Add(""))
				mgr.AppendOptionalString(mapping.AttributeIndices(),
					semconv.ProcessExecutableBuildIDHtlhashKey,
					tt.fileID.StringNoQuotes())
			}

			assert.Equal(t, tt.wantIndex, idx)
			assert.Equal(t, tt.wantMappingSet, mappingSet)
			assert.Equal(t, tt.wantStringSet, stringSet)

			require.Equal(t, len(tt.wantMappingTable), dic.MappingTable().Len())
			for i, v := range tt.wantMappingTable {
				mapp := dic.MappingTable().At(i)
				assert.Equal(t, v, mapp.FilenameStrindex())
			}
		})
	}
}

func newTestFrames(extraFrame bool) libpf.Frames {
	mappingFile := libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
		FileID: libpf.NewFileID(2, 3),
	})
	frames := make(libpf.Frames, 0, 5)
	frames.Append(&libpf.Frame{
		Type:            libpf.KernelFrame,
		AddressOrLineno: 0xef,
		FunctionName:    libpf.Intern("func1"),
		MappingFile:     mappingFile,
	})
	frames.Append(&libpf.Frame{
		Type:            libpf.KernelFrame,
		AddressOrLineno: 0x1ef,
		FunctionName:    libpf.Intern("func2"),
		MappingFile:     mappingFile,
	})
	frames.Append(&libpf.Frame{
		Type:            libpf.KernelFrame,
		AddressOrLineno: 0x2ef,
		FunctionName:    libpf.Intern("func3"),
		MappingFile:     mappingFile,
	})
	frames.Append(&libpf.Frame{
		Type:            libpf.KernelFrame,
		AddressOrLineno: 0x3ef,
		FunctionName:    libpf.Intern("func4"),
		MappingFile:     mappingFile,
	})
	frames.Append(&libpf.Frame{
		Type:            libpf.KernelFrame,
		AddressOrLineno: 0x4ef,
		FunctionName:    libpf.Intern("func5"),
		MappingFile:     mappingFile,
	})

	if extraFrame {
		frames.Append(&libpf.Frame{
			Type:            libpf.KernelFrame,
			AddressOrLineno: 0x5ef,
			FunctionName:    libpf.Intern("func6"),
			MappingFile:     mappingFile,
		})
	}
	return frames
}

func TestFunctionTableOrder(t *testing.T) {
	for _, tt := range []struct {
		name   string
		events map[libpf.Origin]samples.KeyToEventMapping

		wantFunctionTable        []string
		expectedResourceProfiles int
	}{
		{
			name:                     "no events",
			events:                   map[libpf.Origin]samples.KeyToEventMapping{},
			wantFunctionTable:        []string{""},
			expectedResourceProfiles: 0,
		}, {
			name:                     "single executable",
			expectedResourceProfiles: 1,
			events: map[libpf.Origin]samples.KeyToEventMapping{
				support.TraceOriginSampling: map[samples.TraceAndMetaKey]*samples.TraceEvents{
					{Pid: 1}: {
						Frames:     newTestFrames(false),
						Timestamps: []uint64{1, 2, 3, 4, 5},
					},
					// Test Function deduplication
					{Pid: 2}: {
						Frames:     newTestFrames(true),
						Timestamps: []uint64{6, 7, 8, 9, 10, 11},
					},
				},
			},
			wantFunctionTable: []string{
				"", "func1", "func2", "func3", "func4", "func5", "func6",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d, err := New(100, nil)
			require.NoError(t, err)
			tree := make(samples.TraceEventsTree)
			tree[""] = tt.events
			res, _ := d.Generate(tree, tt.name, "version")
			require.Equal(t, tt.expectedResourceProfiles, res.ResourceProfiles().Len())
			if tt.expectedResourceProfiles == 0 {
				// Do not check elements of ResourceProfile if there is no expected
				// ResourceProfile.
				return
			}
			require.Equal(t, 1, res.ResourceProfiles().At(0).ScopeProfiles().Len())
			expectedProfiles := len(tt.events)
			require.Equal(t, expectedProfiles, res.ResourceProfiles().
				At(0).ScopeProfiles().
				At(0).Profiles().Len())
			if expectedProfiles == 0 {
				return
			}
			dic := res.Dictionary()
			require.Equal(t, len(tt.wantFunctionTable), dic.FunctionTable().Len())
			for i := 0; i < dic.FunctionTable().Len(); i++ {
				funcName := dic.StringTable().At(int(dic.FunctionTable().At(i).NameStrindex()))
				assert.Equal(t, tt.wantFunctionTable[i], funcName)
			}
		})
	}
}

func TestProfileDuration(t *testing.T) {
	for _, tt := range []struct {
		name   string
		events map[libpf.Origin]samples.KeyToEventMapping
	}{
		{
			name: "profile duration",
			events: map[libpf.Origin]samples.KeyToEventMapping{
				support.TraceOriginSampling: map[samples.TraceAndMetaKey]*samples.TraceEvents{
					{Pid: 1}: {
						Timestamps: []uint64{2, 1, 3, 4, 7},
					},
					{Pid: 2}: {
						Timestamps: []uint64{8},
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d, err := New(100, nil)
			require.NoError(t, err)

			tree := make(samples.TraceEventsTree)
			tree[""] = tt.events
			res, err := d.Generate(tree, tt.name, "version")
			require.NoError(t, err)

			profile := res.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0)
			require.Equal(t, pcommon.Timestamp(7), profile.Duration())
			require.Equal(t, pcommon.Timestamp(1), profile.Time())
		})
	}
}

func TestGenerate_EmptyTree(t *testing.T) {
	d, err := New(100, nil)
	require.NoError(t, err)

	tree := make(samples.TraceEventsTree)
	profiles, err := d.Generate(tree, "agent", "v1")
	require.NoError(t, err)
	assert.Equal(t, 0, profiles.ResourceProfiles().Len())
}

func singleFrameTrace(ty libpf.FrameType, mappingFile libpf.FrameMappingFile,
	lineno libpf.AddressOrLineno, funcName, sourceFile string,
	sourceLine libpf.SourceLineno,
) libpf.Frames {
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

func TestGenerate_SingleContainerSingleOrigin(t *testing.T) {
	d, err := New(100, nil)
	require.NoError(t, err)

	funcName := "main"
	filePath := "/bin/test"
	mappingFile := libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
		FileID:   libpf.NewFileID(1, 2),
		FileName: libpf.Intern(filePath),
	})

	traceKey := samples.TraceAndMetaKey{
		ExecutablePath: filePath,
		Comm:           "testproc",
		Pid:            123,
		Tid:            456,
		ApmServiceName: "svc",
	}
	events := map[libpf.Origin]samples.KeyToEventMapping{
		support.TraceOriginSampling: {
			traceKey: &samples.TraceEvents{
				Frames: singleFrameTrace(libpf.GoFrame, mappingFile,
					0x10, funcName, filePath, 42),
				Timestamps: []uint64{100},
				EnvVars:    map[string]string{"FOO": "BAR"},
			},
		},
	}
	tree := samples.TraceEventsTree{
		"container1": events,
	}

	profiles, err := d.Generate(tree, "agent", "v1")
	require.NoError(t, err)
	require.Equal(t, 1, profiles.ResourceProfiles().Len())
	rp := profiles.ResourceProfiles().At(0)
	val, _ := rp.Resource().Attributes().Get(string(semconv.ContainerIDKey))
	assert.Equal(t, "container1", val.Str())
	assert.Equal(t, semconv.SchemaURL, rp.SchemaUrl())
	require.Equal(t, 1, rp.ScopeProfiles().Len())
	sp := rp.ScopeProfiles().At(0)
	assert.Equal(t, "agent", sp.Scope().Name())
	assert.Equal(t, "v1", sp.Scope().Version())
	assert.Equal(t, semconv.SchemaURL, sp.SchemaUrl())
	require.Equal(t, 1, sp.Profiles().Len())
	prof := sp.Profiles().At(0)
	assert.Equal(t, pcommon.Timestamp(100), prof.Time())
	assert.Equal(t, pcommon.Timestamp(0), prof.Duration())

	t.Run("Check environment variable attribute", func(t *testing.T) {
		foundFOOKey := false
		foundBarValue := false

		dic := profiles.Dictionary()
		for _, attr := range dic.AttributeTable().All() {
			key := dic.StringTable().At(int(attr.KeyStrindex()))
			value := attr.Value()
			// Check if this is an environment variable attribute
			if key == "process.environment_variable.FOO" {
				foundFOOKey = true
				if value.Type() == pcommon.ValueTypeStr && value.Str() == "BAR" {
					foundBarValue = true
				}
			}
		}
		assert.True(t, foundFOOKey,
			"Attribute 'process.environment_variable.FOO' should be in the attribute table")
		assert.True(t, foundBarValue,
			"Environment variable value 'bar' should be in the attribute table")
	})
}

func TestGenerate_MultipleOriginsAndContainers(t *testing.T) {
	d, err := New(100, nil)
	require.NoError(t, err)

	mappingFile := libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
		FileID:   libpf.NewFileID(5, 6),
		FileName: libpf.Intern("/bin/foo"),
	})
	traceKey := samples.TraceAndMetaKey{ExecutablePath: "/bin/foo"}
	frames := singleFrameTrace(libpf.PythonFrame, mappingFile, 0x20, "f", "/bin/foo", 1)

	events1 := map[libpf.Origin]samples.KeyToEventMapping{
		support.TraceOriginSampling: {
			traceKey: &samples.TraceEvents{
				Frames:     frames,
				Timestamps: []uint64{1, 2},
			},
		},
		support.TraceOriginOffCPU: {
			traceKey: &samples.TraceEvents{
				Frames:     frames,
				Timestamps: []uint64{3, 4},
				OffTimes:   []int64{10, 20},
			},
		},
	}
	events2 := map[libpf.Origin]samples.KeyToEventMapping{
		support.TraceOriginSampling: {
			traceKey: &samples.TraceEvents{
				Frames:     frames,
				Timestamps: []uint64{5},
			},
		},
	}
	tree := samples.TraceEventsTree{
		"c1": events1,
		"c2": events2,
	}

	profiles, err := d.Generate(tree, "agent", "v2")
	require.NoError(t, err)
	require.Equal(t, 2, profiles.ResourceProfiles().Len())

	// Since map iteration order is not guaranteed, we need to check containers by their ID
	containerProfileCounts := make(map[string]int)
	for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
		rp := profiles.ResourceProfiles().At(i)
		val, exists := rp.Resource().Attributes().Get(string(semconv.ContainerIDKey))
		require.True(t, exists)
		containerID := val.Str()
		profileCount := rp.ScopeProfiles().At(0).Profiles().Len()
		containerProfileCounts[containerID] = profileCount
	}

	// c1 has both origins, so 2 profiles
	assert.Equal(t, 2, containerProfileCounts["c1"])
	// c2 has only sampling, so 1 profile
	assert.Equal(t, 1, containerProfileCounts["c2"])
}

func TestGenerate_StringAndFunctionTablePopulation(t *testing.T) {
	d, err := New(100, nil)
	require.NoError(t, err)

	funcName := "myfunc"
	filePath := "/bin/bar"
	mappingFile := libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
		FileID:   libpf.NewFileID(7, 8),
		FileName: libpf.Intern(filePath),
	})

	traceKey := samples.TraceAndMetaKey{ExecutablePath: filePath}
	events := map[libpf.Origin]samples.KeyToEventMapping{
		support.TraceOriginSampling: {
			traceKey: &samples.TraceEvents{
				Frames: singleFrameTrace(libpf.PythonFrame, mappingFile, 0x30,
					funcName, filePath, 123),
				Timestamps: []uint64{42},
			},
		},
	}
	tree := samples.TraceEventsTree{
		"c": events,
	}

	profiles, err := d.Generate(tree, "agent", "v3")
	require.NoError(t, err)
	dic := profiles.Dictionary()
	// The string table should contain "" as first element, then function name and file path
	strs := dic.StringTable().At(0)
	assert.Contains(t, strs, "")
	// Convert StringSlice to a Go slice for assertion
	var stringTableSlice []string
	for i := 0; i < dic.StringTable().Len(); i++ {
		stringTableSlice = append(stringTableSlice, dic.StringTable().At(i))
	}
	assert.Contains(t, stringTableSlice, funcName)
	assert.Contains(t, stringTableSlice, filePath)
	// The function table should have the function name and file path indices set
	require.Equal(t, 2, dic.FunctionTable().Len())
	fn := dic.FunctionTable().At(1)
	assert.Equal(t, funcName, dic.StringTable().At(int(fn.NameStrindex())))
	assert.Equal(t, filePath, dic.StringTable().At(int(fn.FilenameStrindex())))
}

func singleFrameNative(mappingFile libpf.FrameMappingFile, lineno libpf.AddressOrLineno,
	mappingStart, mappingEnd libpf.Address, mappingFileOffset uint64,
) libpf.Frames {
	frames := make(libpf.Frames, 0, 1)
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

func TestGenerate_NativeFrame(t *testing.T) {
	d, err := New(100, nil)
	require.NoError(t, err)

	filePath := "/usr/lib/libexample.so"
	mappingFile := libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
		FileID:   libpf.NewFileID(9, 10),
		FileName: libpf.Intern(filePath),
	})

	traceKey := samples.TraceAndMetaKey{
		ExecutablePath: filePath,
		Comm:           "native_app",
		Pid:            789,
		Tid:            1011,
	}
	events := map[libpf.Origin]samples.KeyToEventMapping{
		support.TraceOriginSampling: {
			traceKey: &samples.TraceEvents{
				Frames:     singleFrameNative(mappingFile, 0x1000, 0x1000, 0x2000, 0x100),
				Timestamps: []uint64{123, 456, 789},
			},
		},
	}
	tree := samples.TraceEventsTree{
		"native_container": events,
	}

	profiles, err := d.Generate(tree, "agent", "v1")
	require.NoError(t, err)
	require.Equal(t, 1, profiles.ResourceProfiles().Len())

	// Check resource profile attributes
	rp := profiles.ResourceProfiles().At(0)
	val, exists := rp.Resource().Attributes().Get(string(semconv.ContainerIDKey))
	require.True(t, exists)
	assert.Equal(t, "native_container", val.Str())

	// Check scope profile
	require.Equal(t, 1, rp.ScopeProfiles().Len())
	sp := rp.ScopeProfiles().At(0)
	assert.Equal(t, "agent", sp.Scope().Name())
	assert.Equal(t, "v1", sp.Scope().Version())

	// Check profile
	require.Equal(t, 1, sp.Profiles().Len())
	prof := sp.Profiles().At(0)
	assert.Equal(t, pcommon.Timestamp(123), prof.Time())
	assert.Equal(t, pcommon.Timestamp(666), prof.Duration())

	// Verify profile contains one sample
	assert.Equal(t, 1, prof.Samples().Len())
	sample := prof.Samples().At(0)
	assert.Len(t, sample.Values().AsRaw(), 0)
	assert.Len(t, sample.TimestampsUnixNano().AsRaw(), 3)

	// Check that the mapping table contains our native frame mapping
	// (plus the dummy mapping at index 0)
	dic := profiles.Dictionary()
	assert.GreaterOrEqual(t, dic.MappingTable().Len(), 2,
		"Mapping table should have dummy mapping + native frame mapping")

	// Find the mapping for our native frame (not the dummy one at index 0)
	var nativeMapping pprofile.Mapping
	found := false
	for i := 1; i < dic.MappingTable().Len(); i++ { // Skip dummy mapping at index 0
		mapping := dic.MappingTable().At(i)
		if mapping.MemoryStart() == uint64(0x1000) {
			nativeMapping = mapping
			found = true
			break
		}
	}
	require.True(t, found, "Should find mapping for native frame")

	// Verify mapping details
	assert.Equal(t, uint64(0x1000), nativeMapping.MemoryStart())
	assert.Equal(t, uint64(0x2000), nativeMapping.MemoryLimit())
	assert.Equal(t, uint64(0x100), nativeMapping.FileOffset())

	// Verify the filename is correctly set in the mapping
	filenameStrIndex := nativeMapping.FilenameStrindex()
	filename := dic.StringTable().At(int(filenameStrIndex))
	assert.Equal(t, filePath, filename)

	// For native frames, function information is not populated in the function table
	// since it's resolved by the backend. The function table should be empty.
	assert.Equal(t, 1, dic.FunctionTable().Len(),
		"Function table should be empty for native frames")
}

func TestStackTableOrder(t *testing.T) {
	for _, tt := range []struct {
		name   string
		events map[libpf.Origin]samples.KeyToEventMapping

		wantStackTable           [][]int32
		expectedLocationTableLen int
	}{
		{
			name: "single stack",
			events: map[libpf.Origin]samples.KeyToEventMapping{
				support.TraceOriginSampling: map[samples.TraceAndMetaKey]*samples.TraceEvents{
					{}: {
						Frames:     newTestFrames(false),
						Timestamps: []uint64{1, 2, 3, 4, 5},
					},
				},
			},
			wantStackTable: [][]int32{
				nil, {1, 2, 3, 4, 5},
			},
			expectedLocationTableLen: 6,
		},
		{
			name: "multiple stacks",
			events: map[libpf.Origin]samples.KeyToEventMapping{
				support.TraceOriginSampling: map[samples.TraceAndMetaKey]*samples.TraceEvents{
					{Pid: 1}: {
						Frames:     newTestFrames(false),
						Timestamps: []uint64{1, 2, 3, 4, 5},
					},
				},
				// This test relies on an implementation detail for ordering of results:
				// it assumes that support.TraceOriginSampling events are processed first
				support.TraceOriginOffCPU: map[samples.TraceAndMetaKey]*samples.TraceEvents{
					{Pid: 2}: {
						Frames:     newTestFrames(true),
						Timestamps: []uint64{7, 8, 9, 10, 11, 12},
					},
					{Pid: 3}: {
						Frames:     newTestFrames(false),
						Timestamps: []uint64{13, 14, 15, 16, 17},
					},
				},
			},
			wantStackTable: [][]int32{
				nil,
				{1, 2, 3, 4, 5},
				{1, 2, 3, 4, 5, 6},
			},
			expectedLocationTableLen: 7,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d, err := New(100, nil)
			require.NoError(t, err)
			tree := make(samples.TraceEventsTree)
			tree[""] = tt.events
			res, _ := d.Generate(tree, tt.name, "version")

			dic := res.Dictionary()

			require.Equal(t, tt.expectedLocationTableLen, dic.LocationTable().Len())
			require.Equal(t, len(tt.wantStackTable), dic.StackTable().Len())
			for i := 0; i < dic.StackTable().Len(); i++ {
				locationIndices := dic.StackTable().At(i).LocationIndices().AsRaw()
				assert.Equal(t, tt.wantStackTable[i], locationIndices)
			}
		})
	}
}
