// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processmanager

// See also tools/coredump/coredump_test.go for core dump based testing.

import (
	"context"
	"debug/elf"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"testing"
	"time"
	"unsafe"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/lpm"
	"go.opentelemetry.io/ebpf-profiler/metrics"
	"go.opentelemetry.io/ebpf-profiler/nativeunwind"
	sdtypes "go.opentelemetry.io/ebpf-profiler/nativeunwind/stackdeltatypes"
	"go.opentelemetry.io/ebpf-profiler/process"
	pmebpf "go.opentelemetry.io/ebpf-profiler/processmanager/ebpfapi"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/dynamicprofiling"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/support"
	tracertypes "go.opentelemetry.io/ebpf-profiler/tracer/types"
	"go.opentelemetry.io/ebpf-profiler/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dummyProcess implements pfelf.Process for testing purposes
type dummyProcess struct {
	pid           libpf.PID
	mappings      []process.Mapping
	mappingsError error
}

func (d *dummyProcess) PID() libpf.PID {
	return d.pid
}

func (d *dummyProcess) GetMachineData() process.MachineData {
	return process.MachineData{}
}

func (d *dummyProcess) GetProcessMeta(_ process.MetaConfig) process.ProcessMeta {
	return process.ProcessMeta{}
}

func (d *dummyProcess) GetExe() (string, error) {
	return "", errors.New("not implemented")
}

func (d *dummyProcess) GetMappings() ([]process.Mapping, uint32, error) {
	return d.mappings, 0, d.mappingsError
}

func (d *dummyProcess) GetThreads() ([]process.ThreadInfo, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyProcess) GetRemoteMemory() remotememory.RemoteMemory {
	return remotememory.RemoteMemory{}
}

func (d *dummyProcess) GetMappingFileLastModified(_ *process.Mapping) int64 {
	return 0
}

func (d *dummyProcess) CalculateMappingFileID(m *process.Mapping) (libpf.FileID, error) {
	return libpf.FileIDFromExecutableFile(m.Path.String())
}

func (d *dummyProcess) OpenMappingFile(m *process.Mapping) (process.ReadAtCloser, error) {
	return os.Open(m.Path.String())
}

func (d *dummyProcess) OpenELF(name string) (*pfelf.File, error) {
	return pfelf.Open(name)
}

func (d *dummyProcess) ExtractAsFile(name string) (string, error) {
	return name, nil
}

func (d *dummyProcess) Close() error {
	return nil
}

func newTestProcess(pid libpf.PID) *dummyProcess {
	return &dummyProcess{pid: pid, mappingsError: errors.New("not implemented")}
}

// dummyStackDeltaProvider is an implementation of nativeunwind.StackDeltaProvider.
// It is intended to be used only within this test.
type dummyStackDeltaProvider struct{}

// GetIntervalStructuresForFile fills in the expected data structure with semi random data.
func (d *dummyStackDeltaProvider) GetIntervalStructuresForFile(_ *pfelf.Reference,
	result *sdtypes.IntervalData) error {
	r := rand.New(rand.NewPCG(42, 42)) //nolint:gosec
	addr := 0x10
	for i := 0; i < r.IntN(42); i++ {
		addr += r.IntN(42 * 42)
		data := int32(8 * r.IntN(42))
		result.Deltas.Add(sdtypes.StackDelta{
			Address: uint64(addr),
			Info:    sdtypes.UnwindInfo{Opcode: support.UnwindOpcodeBaseSP, Param: data},
		})
	}
	return nil
}

// GetAndResetStatistics satisfies the interface and does not return values.
func (d *dummyStackDeltaProvider) GetAndResetStatistics() nativeunwind.Statistics {
	return nativeunwind.Statistics{}
}

// Compile time check that the dummyStackDeltaProvider implements its interface correctly.
var _ nativeunwind.StackDeltaProvider = (*dummyStackDeltaProvider)(nil)

// generateDummyFiles creates num temporary files. The caller is responsible to delete
// these files afterwards.
func generateDummyFiles(t *testing.T, num int) []string {
	t.Helper()
	files := make([]string, 0, num)

	for i := range num {
		name := fmt.Sprintf("dummy%d", i)
		tmpfile, err := os.CreateTemp(t.TempDir(), "*"+name)
		require.NoError(t, err)

		// The generated fileID is based on the content of the file.
		// So we write the pseudo random name to the file as content.
		content := []byte(tmpfile.Name())
		_, err = tmpfile.Write(content)
		require.NoError(t, err)
		_ = tmpfile.Close()
		require.NoError(t, err)
		files = append(files, tmpfile.Name())
	}
	return files
}

// mappingArgs provides a structured way for the arguments to NewMapping()
// for the tests.
type mappingArgs struct {
	// pid represents the simulated process ID.
	pid libpf.PID
	// vaddr represents the simulated start of the mapped memory.
	vaddr uint64
	// bias is the load bias to simulate and verify.
	bias uint64
}

// ebpfMapsMockup implements the ebpf interface as test mockup
type ebpfMapsMockup struct {
	updateProcCount, deleteProcCount uint8

	stackDeltaMemory []pmebpf.StackDeltaEBPF
	// deleteStackDeltaRangesCount reflects the number of times
	// the deleteStackDeltaRanges to update the eBPF map was called.
	deleteStackDeltaRangesCount uint8
	// deleteStackDeltaPage reflects the number of times
	// the DeleteStackDeltaPage to update the eBPF map was called.
	deleteStackDeltaPage uint8
	// deletePidPageMappingCount reflects the number of times
	// the deletePidPageMapping to update the eBPF map was called.
	deletePidPageMappingCount uint8
	// expectedBias value for updatedPidPageToExeIDOffset calls
	expectedBias uint64
}

var _ interpreter.EbpfHandler = &ebpfMapsMockup{}

func (mockup *ebpfMapsMockup) RemoveReportedPID(libpf.PID) {
}

func (mockup *ebpfMapsMockup) UpdateInterpreterOffsets(uint16, host.FileID,
	[]util.Range) error {
	return nil
}

func (mockup *ebpfMapsMockup) UpdateProcData(libpf.InterpreterType, libpf.PID,
	unsafe.Pointer) error {
	mockup.updateProcCount++
	return nil
}

func (mockup *ebpfMapsMockup) DeleteProcData(libpf.InterpreterType, libpf.PID) error {
	mockup.deleteProcCount++
	return nil
}

func (mockup *ebpfMapsMockup) UpdatePidInterpreterMapping(libpf.PID,
	lpm.Prefix, uint8, host.FileID, uint64) error {
	return nil
}

func (mockup *ebpfMapsMockup) DeletePidInterpreterMapping(libpf.PID, lpm.Prefix) error {
	return nil
}

func (mockup *ebpfMapsMockup) UpdateUnwindInfo(uint16, sdtypes.UnwindInfo) error { return nil }

func (mockup *ebpfMapsMockup) UpdateExeIDToStackDeltas(fileID host.FileID,
	deltaArrays []pmebpf.StackDeltaEBPF) (uint16, error) {
	mockup.stackDeltaMemory = append(mockup.stackDeltaMemory, deltaArrays...)
	// execinfomanager expects a mapID >0. So to fake this behavior, we return
	// parts of the fileID.
	return uint16(fileID), nil
}

func (mockup *ebpfMapsMockup) DeleteExeIDToStackDeltas(host.FileID, uint16) error {
	mockup.deleteStackDeltaRangesCount++
	return nil
}

func (mockup *ebpfMapsMockup) UpdateStackDeltaPages(host.FileID, []uint16,
	uint16, uint64) error {
	return nil
}

func (mockup *ebpfMapsMockup) DeleteStackDeltaPage(host.FileID, uint64) error {
	mockup.deleteStackDeltaPage++
	return nil
}

func (mockup *ebpfMapsMockup) UpdatePidPageMappingInfo(pid libpf.PID, prefix lpm.Prefix,
	fileID uint64, bias uint64) error {
	if prefix.Key == 0 && fileID == 0 && bias == 0 {
		// If all provided values are 0 the hook was called to create
		// a dummy entry.
		return nil
	}
	if bias != mockup.expectedBias {
		return fmt.Errorf("expected bias 0x%x for PID %d but got 0x%x",
			mockup.expectedBias, pid, bias)
	}
	return nil
}

func (mockup *ebpfMapsMockup) setExpectedBias(expected uint64) {
	mockup.expectedBias = expected
}

func (mockup *ebpfMapsMockup) DeletePidPageMappingInfo(_ libpf.PID, prefixes []lpm.Prefix) (int,
	error) {
	mockup.deletePidPageMappingCount += uint8(len(prefixes))
	return len(prefixes), nil
}

func (mockup *ebpfMapsMockup) CollectMetrics() []metrics.Metric     { return []metrics.Metric{} }
func (mockup *ebpfMapsMockup) SupportsGenericBatchOperations() bool { return false }
func (mockup *ebpfMapsMockup) SupportsLPMTrieBatchOperations() bool { return false }

func TestNewMapping(t *testing.T) {
	tests := map[string]struct {
		// newMapping holds the arguments that are passed to NewMapping() in the test.
		// For each mappingArgs{} a temporary file will be created.
		newMapping []mappingArgs
		// duplicate indicates if for each {}mappingArgs the generated dummy file
		// should be loaded twice to simulate a duplicate loading.
		duplicate bool
		// expectedStackDeltas holds the number of stack deltas that are
		// expected after loading all temporary files with the arguments from newMapping.
		expectedStackDeltas int
	}{
		"regular load": {newMapping: []mappingArgs{
			{pid: 1, vaddr: 0x10000, bias: 0x0000},
			{pid: 2, vaddr: 0x40000, bias: 0x2000},
			{pid: 3, vaddr: 0x60000, bias: 0x3000},
			{pid: 4, vaddr: 0x40000, bias: 0x4000}},
			expectedStackDeltas: 36},
		"duplicate load": {newMapping: []mappingArgs{
			{pid: 123, vaddr: 0x0F000, bias: 0x1000},
			{pid: 456, vaddr: 0x50000, bias: 0x4000},
			{pid: 789, vaddr: 0x40000, bias: 0}},
			duplicate:           true,
			expectedStackDeltas: 27},
	}

	for name, testcase := range tests {
		t.Run(name, func(t *testing.T) {
			// The generated dummy files do not contain valid stack deltas,
			// so we replace the stack delta provider.
			dummyProvider := dummyStackDeltaProvider{}
			ebpfMockup := &ebpfMapsMockup{}

			// For this test do not include interpreters.
			noInterpreters, _ := tracertypes.Parse("")

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			manager, err := New(ctx,
				noInterpreters,
				1*time.Second,
				ebpfMockup,
				NewMapFileIDMapper(),
				nil,
				nil,
				&dummyProvider,
				true,
				dynamicprofiling.AlwaysOnPolicy{},
				libpf.Set[string]{},
			)
			require.NoError(t, err)

			// Replace the internal hooks for the tests. These hooks catch the
			// updates of the eBPF maps and let us compare the results.
			manager.metricsAddSlice = func(m []metrics.Metric) {
				for id, value := range m {
					t.Logf("Added +%d to metric %d\n", value, id)
				}
			}

			execs := generateDummyFiles(t, len(testcase.newMapping))

			if testcase.duplicate {
				execs = append(execs, execs...)
			}

			// For the duplicate test case we have more test files than provided
			// arguments to NewMapping(). modulo makes sure we don't exceed the
			// index of the provided arguments to NewMapping().
			modulo := len(testcase.newMapping)

			// Simulate new memory mappings
			for index, exec := range execs {
				pid := testcase.newMapping[index%modulo].pid
				vaddr := libpf.Address(testcase.newMapping[index%modulo].vaddr)
				bias := testcase.newMapping[index%modulo].bias
				ebpfMockup.setExpectedBias(bias)

				pr := newTestProcess(pid)
				elfRef := pfelf.NewReference(exec, pr)
				err := manager.handleNewMapping(
					pr, &Mapping{
						FileID: host.FileID(index % modulo),
						Vaddr:  vaddr,
						Bias:   bias,
						Length: 0x10,
					}, elfRef)
				elfRef.Close()
				require.NoError(t, err)
			}

			assert.Len(t, ebpfMockup.stackDeltaMemory, testcase.expectedStackDeltas)
		})
	}
}

// populateManager fills the internal maps of the process manager with some dummy information.
func populateManager(t *testing.T, pm *ProcessManager) {
	t.Helper()

	data := []struct {
		pid libpf.PID

		mapping Mapping
	}{
		{
			pid: 1,
			mapping: Mapping{
				FileID: host.FileID(127),
				Vaddr:  libpf.Address(0x1000),
				Bias:   0x1000,
				Length: 127,
			},
		}, {
			pid: 2,
			mapping: Mapping{
				FileID: host.FileID(128),
				Vaddr:  libpf.Address(0x1000),
				Bias:   0x1000,
				Length: 128,
			},
		}, {
			pid: 2,
			mapping: Mapping{
				FileID: host.FileID(129),
				Vaddr:  libpf.Address(0x1000),
				Bias:   0x1000,
				Length: 129,
			},
		}, {
			pid: 920,
			mapping: Mapping{
				FileID: host.FileID(128),
				Vaddr:  libpf.Address(0x1000),
				Bias:   0x1000,
				Length: 128,
			},
		}, {
			pid: 921,
			mapping: Mapping{
				FileID: host.FileID(129),
				Vaddr:  libpf.Address(0x2000),
				Bias:   0x2000,
				Length: 129,
			},
		}, {
			pid: 3,
			mapping: Mapping{
				FileID: host.FileID(130),
				Vaddr:  libpf.Address(0x3000),
				Bias:   0x3000,
				Length: 130,
			},
		}, {
			pid: 3,
			mapping: Mapping{
				FileID: host.FileID(131),
				Vaddr:  libpf.Address(0x4000),
				Bias:   0x4000,
				Length: 131,
			},
		},
	}

	mockup := pm.ebpf.(*ebpfMapsMockup)

	for _, d := range data {
		c := d
		mockup.setExpectedBias(c.mapping.Bias)
		pr := newTestProcess(c.pid)
		elfRef := pfelf.NewReference("", pr)
		err := pm.handleNewMapping(pr, &c.mapping, elfRef)
		require.NoError(t, err)
	}
}

func TestProcExit(t *testing.T) {
	tests := map[string]struct {
		// pid represents the ID of a process.
		pid libpf.PID
		// deletePidPageMappingCount reflects the number of times
		// the deletePidPageMappingHook to update the eBPF map was called.
		deletePidPageMappingCount uint8
		// deleteExeIDToIndicesCount reflects the number of times
		// the deleteExeIDToIndicesHook to update the eBPF map was called.
		deleteExeIDToIndicesCount uint8
		// deleteStackDeltaRangesCount reflects the number of times
		// the deleteStackDeltaRangesHook to update the eBPF map was called.
		deleteStackDeltaRangesCount uint8
	}{
		// unknown process simulates a test case where the process manager is
		// informed about the process exit of a process it is not aware of.
		"unknown process": {pid: 512},
		// process with single mapping simulates a test case where a process with a single
		// memory mapping exits and this was the last mapping for the loaded executables.
		"process with single mapping": {pid: 1,
			deletePidPageMappingCount:   8,
			deleteExeIDToIndicesCount:   1,
			deleteStackDeltaRangesCount: 1},
		// process with multiple mapped mappings simulates a test case where a process with
		// multiple memory mappings exits but the mappings are still referenced else where.
		"process with multiple mapped mappings": {pid: 2,
			deletePidPageMappingCount:   3,
			deleteExeIDToIndicesCount:   0,
			deleteStackDeltaRangesCount: 0},
		// process with multiple one-time mappings simulates a test case where a process with
		// multiple one-time memory mappings exits and these mappings need to be removed.
		"process with multiple one-time mappings": {pid: 3,
			deletePidPageMappingCount:   6,
			deleteExeIDToIndicesCount:   2,
			deleteStackDeltaRangesCount: 2},
	}

	for name, testcase := range tests {
		t.Run(name, func(t *testing.T) {
			// The generated dummy files do not contain valid stack deltas,
			// so we replace the stack delta provider.
			dummyProvider := dummyStackDeltaProvider{}
			ebpfMockup := &ebpfMapsMockup{}

			// For this test do not include interpreters.
			noInterpreters, _ := tracertypes.Parse("")

			ctx, cancel := context.WithCancel(t.Context())

			manager, err := New(ctx,
				noInterpreters,
				1*time.Second,
				ebpfMockup,
				NewMapFileIDMapper(),
				nil,
				nil,
				&dummyProvider,
				true,
				dynamicprofiling.AlwaysOnPolicy{},
				libpf.Set[string]{},
			)
			require.NoError(t, err)
			defer cancel()

			// Replace the internal hooks for the tests. These hooks catch the
			// updates of the eBPF maps and let us compare the results.
			manager.metricsAddSlice = func(m []metrics.Metric) {
				for id, value := range m {
					t.Logf("Added +%d to metric %d\n", value, id)
				}
			}

			populateManager(t, manager)

			manager.processPIDExit(testcase.pid)
			assert.Equal(t, testcase.deletePidPageMappingCount,
				ebpfMockup.deletePidPageMappingCount)
			assert.Equal(t, testcase.deleteStackDeltaRangesCount,
				ebpfMockup.deleteStackDeltaPage)
			assert.Equal(t, testcase.deleteStackDeltaRangesCount,
				ebpfMockup.deleteStackDeltaRangesCount)
		})
	}
}

type testPolicy struct {
	enabled bool
}

func (t *testPolicy) ProfilingEnabled(_ process.Process, _ string) bool {
	return t.enabled
}

func TestDynamicProfilingPolicy(t *testing.T) {
	dummyProvider := dummyStackDeltaProvider{}
	ebpfMockup := &ebpfMapsMockup{}

	noInterpreters, _ := tracertypes.Parse("")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	policy := &testPolicy{}
	manager, err := New(ctx,
		noInterpreters,
		1*time.Second,
		ebpfMockup,
		NewMapFileIDMapper(),
		nil,
		nil,
		&dummyProvider,
		true,
		policy, nil)
	require.NoError(t, err)
	defer manager.Close()
	manager.metricsAddSlice = func([]metrics.Metric) {}

	pid := libpf.PID(os.Getpid())
	pr := newTestProcess(pid)
	pr.mappings, _, pr.mappingsError = process.New(pid, pid).GetMappings()
	exe, _ := os.Readlink("/proc/self/exe")
	for _, m := range pr.mappings {
		if m.Path == libpf.Intern(exe) && m.Flags&elf.PF_X != 0 {
			pr.mappings = []process.Mapping{m}
			break
		}
	}
	require.Len(t, pr.mappings, 1)

	manager.SynchronizeProcess(pr)
	manager.mu.RLock()
	info := manager.pidToProcessInfo[pid]
	manager.mu.RUnlock()
	require.Nil(t, info)

	policy.enabled = true

	manager.SynchronizeProcess(pr)
	manager.mu.RLock()
	info = manager.pidToProcessInfo[pid]
	manager.mu.RUnlock()
	require.NotNil(t, info)

	policy.enabled = false

	manager.SynchronizeProcess(pr)
	manager.mu.RLock()
	info = manager.pidToProcessInfo[pid]
	manager.mu.RUnlock()
	require.NotNil(t, info)
}
