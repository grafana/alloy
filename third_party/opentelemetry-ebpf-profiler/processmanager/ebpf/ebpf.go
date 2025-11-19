// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ebpf // import "go.opentelemetry.io/ebpf-profiler/processmanager/ebpf"

import (
	"context"
	"errors"
	"fmt"
	"math/bits"
	"reflect"
	"sync"
	"unsafe"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/features"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/constraints"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfunsafe"
	"go.opentelemetry.io/ebpf-profiler/lpm"
	"go.opentelemetry.io/ebpf-profiler/metrics"
	sdtypes "go.opentelemetry.io/ebpf-profiler/nativeunwind/stackdeltatypes"
	"go.opentelemetry.io/ebpf-profiler/processmanager/ebpfapi"
	"go.opentelemetry.io/ebpf-profiler/rlimit"
	"go.opentelemetry.io/ebpf-profiler/support"
	"go.opentelemetry.io/ebpf-profiler/util"
)

const (
	// updatePoolWorkers decides how many background workers we spawn to
	// process map-in-map updates.
	updatePoolWorkers = 16
	// updatePoolQueueCap decides the work queue capacity of each worker.
	updatePoolQueueCap = 8
)

type ebpfMapsImpl struct {
	// Interpreter related eBPF maps
	InterpreterOffsets *cebpf.Map `name:"interpreter_offsets"`
	DotnetProcs        *cebpf.Map `name:"dotnet_procs"`
	PerlProcs          *cebpf.Map `name:"perl_procs"`
	PyProcs            *cebpf.Map `name:"py_procs"`
	HotspotProcs       *cebpf.Map `name:"hotspot_procs"`
	PhpProcs           *cebpf.Map `name:"php_procs"`
	RubyProcs          *cebpf.Map `name:"ruby_procs"`
	V8Procs            *cebpf.Map `name:"v8_procs"`
	ApmIntProcs        *cebpf.Map `name:"apm_int_procs"`
	GoLabelsProcs      *cebpf.Map `name:"go_labels_procs"`

	// Stackdelta and process related eBPF maps
	ExeIDToStackDeltaMaps []*cebpf.Map
	StackDeltaPageToInfo  *cebpf.Map `name:"stack_delta_page_to_info"`
	PidPageToMappingInfo  *cebpf.Map `name:"pid_page_to_mapping_info"`
	UnwindInfoArray       *cebpf.Map `name:"unwind_info_array"`
	ReportedPIDs          *cebpf.Map `name:"reported_pids"`

	errCounterLock sync.Mutex
	errCounter     map[metrics.MetricID]int64

	hasGenericBatchOperations bool
	hasLPMTrieBatchOperations bool

	updateWorkers *asyncMapUpdaterPool
}

// Compile time check to make sure ebpfMapsImpl satisfies the interface .
var _ ebpfapi.EbpfHandler = &ebpfMapsImpl{}

// LoadMaps checks if the needed maps for the process manager are available
// and loads their references into a package-internal structure.
//
// It further spawns background workers for deferred map updates; the given
// context can be used to terminate them on shutdown.
func LoadMaps(ctx context.Context, maps map[string]*cebpf.Map) (ebpfapi.EbpfHandler, error) {
	impl := &ebpfMapsImpl{}
	impl.errCounter = make(map[metrics.MetricID]int64)

	implRefVal := reflect.ValueOf(impl).Elem()
	implRefType := reflect.TypeOf(impl).Elem()
	for i := 0; i < implRefType.NumField(); i++ {
		fieldType := implRefType.Field(i)
		nameTag, ok := fieldType.Tag.Lookup("name")
		if !ok {
			continue
		}
		mapVal, ok := maps[nameTag]
		if !ok {
			log.Fatalf("Map %v is not available", nameTag)
		}
		implRefVal.Field(i).Set(reflect.ValueOf(mapVal))
	}

	numBuckets := support.StackDeltaBucketLargest - support.StackDeltaBucketSmallest + 1
	impl.ExeIDToStackDeltaMaps = make([]*cebpf.Map, numBuckets)
	for i := support.StackDeltaBucketSmallest; i <= support.StackDeltaBucketLargest; i++ {
		deltasMapName := fmt.Sprintf("exe_id_to_%d_stack_deltas", i)
		deltasMap, ok := maps[deltasMapName]
		if !ok {
			log.Fatalf("Map %s is not available", deltasMapName)
		}
		impl.ExeIDToStackDeltaMaps[i-support.StackDeltaBucketSmallest] = deltasMap
	}

	if err := probeBatchOperations(cebpf.Hash); err == nil {
		log.Infof("Supports generic eBPF map batch operations")
		impl.hasGenericBatchOperations = true
	}

	if err := probeBatchOperations(cebpf.LPMTrie); err == nil {
		log.Infof("Supports LPM trie eBPF map batch operations")
		impl.hasLPMTrieBatchOperations = true
	}

	impl.updateWorkers = newAsyncMapUpdaterPool(ctx, updatePoolWorkers, updatePoolQueueCap)

	return impl, nil
}

// UpdateInterpreterOffsets adds the given moduleRanges to the eBPF map interpreterOffsets.
func (impl *ebpfMapsImpl) UpdateInterpreterOffsets(ebpfProgIndex uint16, fileID host.FileID,
	offsetRanges []util.Range) error {
	key, value, err := ebpfapi.InterpreterOffsetKeyValue(ebpfProgIndex, fileID, offsetRanges)
	if err != nil {
		return err
	}
	if err := impl.InterpreterOffsets.Update(unsafe.Pointer(&key), unsafe.Pointer(&value),
		cebpf.UpdateAny); err != nil {
		log.Fatalf("Failed to place interpreter range in map: %v", err)
	}

	return nil
}

// getInterpreterTypeMap returns the eBPF map for the given typ
// or an error if typ is not supported.
func (impl *ebpfMapsImpl) getInterpreterTypeMap(typ libpf.InterpreterType) (*cebpf.Map, error) {
	switch typ {
	case libpf.Dotnet:
		return impl.DotnetProcs, nil
	case libpf.Perl:
		return impl.PerlProcs, nil
	case libpf.Python:
		return impl.PyProcs, nil
	case libpf.HotSpot:
		return impl.HotspotProcs, nil
	case libpf.PHP:
		return impl.PhpProcs, nil
	case libpf.Ruby:
		return impl.RubyProcs, nil
	case libpf.V8:
		return impl.V8Procs, nil
	case libpf.APMInt:
		return impl.ApmIntProcs, nil
	case libpf.GoLabels:
		return impl.GoLabelsProcs, nil
	default:
		return nil, fmt.Errorf("type %d is not (yet) supported", typ)
	}
}

// UpdateProcData adds the given PID specific data to the specified interpreter data eBPF map.
func (impl *ebpfMapsImpl) UpdateProcData(typ libpf.InterpreterType, pid libpf.PID,
	data unsafe.Pointer) error {
	log.Debugf("Loading symbol addresses into eBPF map for PID %d type %d",
		pid, typ)
	ebpfMap, err := impl.getInterpreterTypeMap(typ)
	if err != nil {
		return err
	}

	pid32 := uint32(pid)
	if err := ebpfMap.Update(unsafe.Pointer(&pid32), data, cebpf.UpdateAny); err != nil {
		return fmt.Errorf("failed to add %v info: %s", typ, err)
	}
	return nil
}

// DeleteProcData removes the given PID specific data of the specified interpreter data eBPF map.
func (impl *ebpfMapsImpl) DeleteProcData(typ libpf.InterpreterType, pid libpf.PID) error {
	log.Debugf("Removing symbol addresses from eBPF map for PID %d type %d",
		pid, typ)
	ebpfMap, err := impl.getInterpreterTypeMap(typ)
	if err != nil {
		return err
	}

	pid32 := uint32(pid)
	if err := ebpfMap.Delete(unsafe.Pointer(&pid32)); err != nil {
		return fmt.Errorf("failed to remove info: %v", err)
	}
	return nil
}

// getPIDPage initializes a PIDPage instance.
func getPIDPage(pid libpf.PID, key uint64, length uint32) support.PIDPage {
	// pid_page_to_mapping_info is an LPM trie and expects the pid and page
	// to be in big endian format.
	return support.PIDPage{
		Pid:       bits.ReverseBytes32(uint32(pid)),
		Page:      bits.ReverseBytes64(key),
		PrefixLen: support.BitWidthPID + length,
	}
}

// getPIDPageFromPrefix initializes a PIDPage instance from a PID and lpm.Prefix.
func getPIDPageFromPrefix(pid libpf.PID, prefix lpm.Prefix) support.PIDPage {
	return getPIDPage(pid, prefix.Key, prefix.Length)
}

// UpdatePidInterpreterMapping updates the eBPF map pidPageToMappingInfo with the
// data required to call the correct interpreter unwinder for that memory region.
func (impl *ebpfMapsImpl) UpdatePidInterpreterMapping(pid libpf.PID, prefix lpm.Prefix,
	interpreterProgram uint8, fileID host.FileID, bias uint64) error {
	cKey := getPIDPageFromPrefix(pid, prefix)
	biasAndUnwindProgram, err := support.EncodeBiasAndUnwindProgram(bias, interpreterProgram)
	if err != nil {
		return err
	}

	cValue := support.PIDPageMappingInfo{
		File_id:                 uint64(fileID),
		Bias_and_unwind_program: biasAndUnwindProgram,
	}

	return impl.PidPageToMappingInfo.Update(unsafe.Pointer(&cKey), unsafe.Pointer(&cValue),
		cebpf.UpdateNoExist)
}

// DeletePidInterpreterMapping removes the element specified by pid, prefix and a corresponding
// mapping size from the eBPF map pidPageToMappingInfo. It is normally used when an
// interpreter process dies or a region that formerly required interpreter-based unwinding is no
// longer needed.
func (impl *ebpfMapsImpl) DeletePidInterpreterMapping(pid libpf.PID, prefix lpm.Prefix) error {
	cKey := getPIDPageFromPrefix(pid, prefix)
	return impl.PidPageToMappingInfo.Delete(unsafe.Pointer(&cKey))
}

// trackMapError is a wrapper to report issues with changes to eBPF maps.
func (impl *ebpfMapsImpl) trackMapError(id metrics.MetricID, err error) error {
	if err != nil {
		impl.errCounterLock.Lock()
		impl.errCounter[id]++
		impl.errCounterLock.Unlock()
	}
	return err
}

// CollectMetrics returns gathered errors for changes to eBPF maps.
func (impl *ebpfMapsImpl) CollectMetrics() []metrics.Metric {
	impl.errCounterLock.Lock()
	defer impl.errCounterLock.Unlock()

	counts := make([]metrics.Metric, 0, 7)
	for id, value := range impl.errCounter {
		counts = append(counts, metrics.Metric{
			ID:    id,
			Value: metrics.MetricValue(value),
		})
		// As we don't want to report metrics with zero values on the next call,
		// we delete the entries from the map instead of just resetting them.
		delete(impl.errCounter, id)
	}

	return counts
}

// poolPIDPage caches reusable heap-allocated PIDPage instances
// to avoid excessive heap allocations.
var poolPIDPage = sync.Pool{
	New: func() any {
		return new(support.PIDPage)
	},
}

// getPIDPagePooled returns a heap-allocated and initialized PIDPage instance.
// After usage, put the instance back into the pool with poolPIDPage.Put().
func getPIDPagePooled(pid libpf.PID, prefix lpm.Prefix) *support.PIDPage {
	cPIDPage := poolPIDPage.Get().(*support.PIDPage)
	*cPIDPage = getPIDPageFromPrefix(pid, prefix)
	return cPIDPage
}

// poolPIDPageMappingInfo caches reusable heap-allocated PIDPageMappingInfo instances
// to avoid excessive heap allocations.
var poolPIDPageMappingInfo = sync.Pool{
	New: func() any {
		return new(support.PIDPageMappingInfo)
	},
}

// getPIDPageMappingInfo returns a heap-allocated and initialized PIDPageMappingInfo instance.
// After usage, put the instance back into the pool with poolPIDPageMappingInfo.Put().
func getPIDPageMappingInfo(fileID, biasAndUnwindProgram uint64) *support.PIDPageMappingInfo {
	cInfo := poolPIDPageMappingInfo.Get().(*support.PIDPageMappingInfo)
	cInfo.File_id = fileID
	cInfo.Bias_and_unwind_program = biasAndUnwindProgram

	return cInfo
}

// probeBatchOperations tests if the BPF syscall accepts batch operations. It
// returns nil if batch operations are supported for mapType or an error otherwise.
func probeBatchOperations(mapType cebpf.MapType) error {
	restoreRlimit, err := rlimit.MaximizeMemlock()
	if err != nil {
		// In environment like github action runners, we can not adjust rlimit.
		// Therefore we just return false here and do not use batch operations.
		return fmt.Errorf("failed to adjust rlimit: %w", err)
	}
	defer restoreRlimit()

	updates := 5
	mapSpec := &cebpf.MapSpec{
		Type:       mapType,
		KeySize:    8,
		ValueSize:  8,
		MaxEntries: uint32(updates),
		Flags:      features.BPF_F_NO_PREALLOC,
	}

	var keys any
	switch mapType {
	case cebpf.Array:
		// KeySize for Array maps always needs to be 4.
		mapSpec.KeySize = 4
		// Array maps are always preallocated.
		mapSpec.Flags = 0
		keys = generateSlice[uint32](updates)
	default:
		keys = generateSlice[uint64](updates)
	}

	probeMap, err := cebpf.NewMap(mapSpec)
	if err != nil {
		return fmt.Errorf("failed to create %s map for batch probing: %v",
			mapType, err)
	}
	defer probeMap.Close()

	values := generateSlice[uint64](updates)

	n, err := probeMap.BatchUpdate(keys, values, nil)
	if err != nil {
		// Older kernel do not support batch operations on maps.
		// This is just fine and we return here.
		return err
	}
	if n != updates {
		return fmt.Errorf("unexpected batch update return: expected %d but got %d",
			updates, n)
	}

	// Remove the probe entries from the map.
	m, err := probeMap.BatchDelete(keys, nil)
	if err != nil {
		return err
	}
	if m != updates {
		return fmt.Errorf("unexpected batch delete return: expected %d but got %d",
			updates, m)
	}
	return nil
}

// getMapID returns the mapID number to use for given number of stack deltas.
func getMapID(numDeltas uint32) (uint16, error) {
	significantBits := 32 - bits.LeadingZeros32(numDeltas)
	if significantBits <= support.StackDeltaBucketSmallest {
		return support.StackDeltaBucketSmallest, nil
	}
	if significantBits > support.StackDeltaBucketLargest {
		return 0, fmt.Errorf("no map available for %d stack deltas", numDeltas)
	}
	return uint16(significantBits), nil
}

// getOuterMap is a helper function to select the correct outer map for
// storing the stack deltas based on the mapID.
func (impl *ebpfMapsImpl) getOuterMap(mapID uint16) *cebpf.Map {
	if mapID < support.StackDeltaBucketSmallest ||
		mapID > support.StackDeltaBucketLargest {
		return nil
	}
	return impl.ExeIDToStackDeltaMaps[mapID-support.StackDeltaBucketSmallest]
}

// RemoveReportedPID removes a PID from the reported_pids eBPF map. The kernel component will
// place a PID in this map before it reports it to Go for further processing.
func (impl *ebpfMapsImpl) RemoveReportedPID(pid libpf.PID) {
	key := uint32(pid)
	_ = impl.ReportedPIDs.Delete(unsafe.Pointer(&key))
}

// UpdateUnwindInfo writes UnwindInfo into the unwind info array at the given index
func (impl *ebpfMapsImpl) UpdateUnwindInfo(index uint16, info sdtypes.UnwindInfo) error {
	if uint32(index) >= impl.UnwindInfoArray.MaxEntries() {
		return fmt.Errorf("unwind info array full (%d/%d items)",
			index, impl.UnwindInfoArray.MaxEntries())
	}

	key := uint32(index)
	value := support.UnwindInfo{
		Opcode:      info.Opcode,
		FpOpcode:    info.FPOpcode,
		MergeOpcode: info.MergeOpcode,
		Param:       info.Param,
		FpParam:     info.FPParam,
	}
	return impl.trackMapError(metrics.IDUnwindInfoArrayUpdate,
		impl.UnwindInfoArray.Update(unsafe.Pointer(&key), unsafe.Pointer(&value),
			cebpf.UpdateAny))
}

// UpdateExeIDToStackDeltas creates a nested map for fileID in the eBPF map exeIDTostack_deltas
// and inserts the elements of the deltas array in this nested map. Returns mapID or error.
func (impl *ebpfMapsImpl) UpdateExeIDToStackDeltas(fileID host.FileID,
	deltas []ebpfapi.StackDeltaEBPF) (
	uint16, error) {
	numDeltas := len(deltas)
	mapID, err := getMapID(uint32(numDeltas))
	if err != nil {
		return 0, err
	}
	outerMap := impl.getOuterMap(mapID)

	restoreRlimit, err := rlimit.MaximizeMemlock()
	if err != nil {
		return 0, fmt.Errorf("failed to increase rlimit: %v", err)
	}
	defer restoreRlimit()
	innerMap, err := cebpf.NewMap(&cebpf.MapSpec{
		Type:       cebpf.Array,
		KeySize:    4,
		ValueSize:  support.Sizeof_StackDelta,
		MaxEntries: 1 << mapID,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create inner map: %v", err)
	}
	defer func() {
		if err = innerMap.Close(); err != nil {
			log.Errorf("Failed to close FD of inner map for 0x%x: %v", fileID, err)
		}
	}()

	// We continue updating the inner map after enqueueing the update to the
	// outer map. Both the async update pool and our code below need an open
	// file descriptor to work, and we don't know which will complete first.
	// We thus clone the FD, transfer ownership of the clone to the update
	// pool and continue using our original FD whose lifetime is now no longer
	// tied to the FD used in the updater pool.
	innerMapCloned, err := innerMap.Clone()
	if err != nil {
		return 0, fmt.Errorf("failed to clone inner map: %v", err)
	}

	impl.updateWorkers.EnqueueUpdate(outerMap, fileID, innerMapCloned)

	if impl.hasGenericBatchOperations {
		innerKeys := make([]uint32, numDeltas)
		stackDeltas := make([]support.StackDelta, numDeltas)

		// Prepare values for batch update.
		for index, delta := range deltas {
			innerKeys[index] = uint32(index)
			stackDeltas[index].AddrLow = delta.AddressLow
			stackDeltas[index].UnwindInfo = delta.UnwindInfo
		}

		_, err := innerMap.BatchUpdate(
			ptrCastMarshaler[uint32](innerKeys),
			ptrCastMarshaler[support.StackDelta](stackDeltas),
			&cebpf.BatchOptions{Flags: uint64(cebpf.UpdateAny)})
		if err != nil {
			return 0, impl.trackMapError(metrics.IDExeIDToStackDeltasBatchUpdate,
				fmt.Errorf("failed to batch insert %d elements for 0x%x "+
					"into exeIDTostack_deltas: %v",
					numDeltas, fileID, err))
		}
		return mapID, nil
	}

	innerKey := uint32(0)
	stackDelta := support.StackDelta{}
	for index, delta := range deltas {
		stackDelta.AddrLow = delta.AddressLow
		stackDelta.UnwindInfo = delta.UnwindInfo
		innerKey = uint32(index)
		if err := innerMap.Update(unsafe.Pointer(&innerKey), unsafe.Pointer(&stackDelta),
			cebpf.UpdateAny); err != nil {
			return 0, impl.trackMapError(metrics.IDExeIDToStackDeltasUpdate, fmt.Errorf(
				"failed to insert element %d for 0x%x into exeIDTostack_deltas: %v",
				index, fileID, err))
		}
	}

	return mapID, nil
}

// DeleteExeIDToStackDeltas removes all eBPF stack delta entries for given fileID and mapID number.
func (impl *ebpfMapsImpl) DeleteExeIDToStackDeltas(fileID host.FileID, mapID uint16) error {
	outerMap := impl.getOuterMap(mapID)
	if outerMap == nil {
		return fmt.Errorf("invalid mapID %d", mapID)
	}

	// Deleting the entry from the outer maps deletes also the entries of the inner
	// map associated with this outer key.
	impl.updateWorkers.EnqueueUpdate(outerMap, fileID, nil)

	return nil
}

// UpdateStackDeltaPages adds fileID/page with given information to eBPF map. If the entry exists,
// it will return an error. Otherwise the key/value pairs will be appended to the hash.
func (impl *ebpfMapsImpl) UpdateStackDeltaPages(fileID host.FileID, numDeltasPerPage []uint16,
	mapID uint16, firstPageAddr uint64) error {
	firstDelta := uint32(0)
	keys := make([]support.StackDeltaPageKey, len(numDeltasPerPage))
	values := make([]support.StackDeltaPageInfo, len(numDeltasPerPage))

	// Prepare the key/value combinations that will be loaded.
	for pageNumber, numDeltas := range numDeltasPerPage {
		pageAddr := firstPageAddr + uint64(pageNumber)<<support.StackDeltaPageBits
		keys[pageNumber] = support.StackDeltaPageKey{
			FileID: uint64(fileID),
			Page:   pageAddr,
		}
		values[pageNumber] = support.StackDeltaPageInfo{
			FirstDelta: firstDelta,
			NumDeltas:  numDeltas,
			MapID:      mapID,
		}
		firstDelta += uint32(numDeltas)
	}

	if impl.hasGenericBatchOperations {
		_, err := impl.StackDeltaPageToInfo.BatchUpdate(
			ptrCastMarshaler[support.StackDeltaPageKey](keys),
			ptrCastMarshaler[support.StackDeltaPageInfo](values),
			&cebpf.BatchOptions{Flags: uint64(cebpf.UpdateNoExist)})
		return impl.trackMapError(metrics.IDStackDeltaPageToInfoBatchUpdate, err)
	}

	for index := range keys {
		if err := impl.trackMapError(metrics.IDStackDeltaPageToInfoUpdate,
			impl.StackDeltaPageToInfo.Update(unsafe.Pointer(&keys[index]),
				unsafe.Pointer(&values[index]), cebpf.UpdateNoExist)); err != nil {
			return err
		}
	}
	return nil
}

// DeleteStackDeltaPage removes the entry specified by fileID and page from the eBPF map.
func (impl *ebpfMapsImpl) DeleteStackDeltaPage(fileID host.FileID, page uint64) error {
	key := support.StackDeltaPageKey{
		FileID: uint64(fileID),
		Page:   page,
	}
	return impl.trackMapError(metrics.IDStackDeltaPageToInfoDelete,
		impl.StackDeltaPageToInfo.Delete(unsafe.Pointer(&key)))
}

// UpdatePidPageMappingInfo adds the pid and page combination with a corresponding fileID and
// bias as value to the eBPF map pid_page_to_mapping_info.
// Given a PID and a virtual address, the native unwinder can perform one lookup and obtain both
// the fileID of the text section that is mapped at this virtual address, and the offset into the
// text section that this page can be found at on disk.
// If the key/value pair already exists it will return an error.
func (impl *ebpfMapsImpl) UpdatePidPageMappingInfo(pid libpf.PID, prefix lpm.Prefix,
	fileID, bias uint64) error {
	biasAndUnwindProgram, err := support.EncodeBiasAndUnwindProgram(bias, support.ProgUnwindNative)
	if err != nil {
		return err
	}

	cKey := getPIDPagePooled(pid, prefix)
	defer poolPIDPage.Put(cKey)

	cValue := getPIDPageMappingInfo(fileID, biasAndUnwindProgram)
	defer poolPIDPageMappingInfo.Put(cValue)

	return impl.trackMapError(metrics.IDPidPageToMappingInfoUpdate,
		impl.PidPageToMappingInfo.Update(unsafe.Pointer(cKey), unsafe.Pointer(cValue),
			cebpf.UpdateNoExist))
}

// DeletePidPageMappingInfo removes the elements specified by prefixes from eBPF map
// pid_page_to_mapping_info and returns the number of elements removed.
func (impl *ebpfMapsImpl) DeletePidPageMappingInfo(pid libpf.PID, prefixes []lpm.Prefix) (int,
	error) {
	if impl.hasLPMTrieBatchOperations {
		deleted, err := impl.DeletePidPageMappingInfoBatch(pid, prefixes)
		if err != nil {
			// BatchDelete may return early and not run to completion. If that happens,
			// fall back to a single Delete pass to avoid leaking map entries.
			deleted2, _ := impl.DeletePidPageMappingInfoSingle(pid, prefixes)
			return (deleted + deleted2), err
		}
		return deleted, nil
	}
	return impl.DeletePidPageMappingInfoSingle(pid, prefixes)
}

func (impl *ebpfMapsImpl) DeletePidPageMappingInfoSingle(pid libpf.PID, prefixes []lpm.Prefix) (int,
	error) {
	var cKey = &support.PIDPage{}
	var deleted int
	var combinedErrors error
	for _, prefix := range prefixes {
		*cKey = getPIDPageFromPrefix(pid, prefix)
		if err := impl.PidPageToMappingInfo.Delete(unsafe.Pointer(cKey)); err != nil {
			_ = impl.trackMapError(metrics.IDPidPageToMappingInfoDelete, err)
			combinedErrors = errors.Join(combinedErrors, err)
			continue
		}
		deleted++
	}
	return deleted, combinedErrors
}

func (impl *ebpfMapsImpl) DeletePidPageMappingInfoBatch(pid libpf.PID, prefixes []lpm.Prefix) (int,
	error) {
	// Prepare all keys based on the given prefixes.
	cKeys := make([]support.PIDPage, 0, len(prefixes))
	for _, prefix := range prefixes {
		cKeys = append(cKeys, getPIDPageFromPrefix(pid, prefix))
	}

	deleted, err := impl.PidPageToMappingInfo.BatchDelete(
		ptrCastMarshaler[support.PIDPage](cKeys), nil)
	return deleted, impl.trackMapError(metrics.IDPidPageToMappingInfoBatchDelete, err)
}

// LookupPidPageInformation returns the fileID and bias for a given pid and page combination from
// the eBPF map pid_page_to_mapping_info.
// So far this function is used only in tests.
func (impl *ebpfMapsImpl) LookupPidPageInformation(pid libpf.PID, page uint64) (host.FileID,
	uint64, error) {
	cKey := getPIDPage(pid, page, support.BitWidthPage)
	cValue := support.PIDPageMappingInfo{}
	if err := impl.PidPageToMappingInfo.Lookup(unsafe.Pointer(&cKey),
		unsafe.Pointer(&cValue)); err != nil {
		return host.FileID(0), 0, fmt.Errorf("failed to lookup page 0x%x for PID %d: %v",
			page, pid, err)
	}
	bias, _ := support.DecodeBiasAndUnwindProgram(cValue.Bias_and_unwind_program)
	return host.FileID(cValue.File_id), bias, nil
}

// SupportsGenericBatchOperations returns true if the kernel supports eBPF batch operations
// on hash and array maps.
func (impl *ebpfMapsImpl) SupportsGenericBatchOperations() bool {
	return impl.hasGenericBatchOperations
}

// SupportsLPMTrieBatchOperations returns true if the kernel supports eBPF batch operations
// on LPM trie maps.
func (impl *ebpfMapsImpl) SupportsLPMTrieBatchOperations() bool {
	return impl.hasLPMTrieBatchOperations
}

// ptrCastMarshaler is a small wrapper type intended to be used with cilium's BatchUpdate and
// BackDelete functions.
//
// Usually cilium will send any slice passed to these functions through the standard library's
// binary.Write function. This not only uses reflection to inspect every single item in the slice,
// but also results in an avoidable copy.
//
// However, before cilium does this, it checks whether the type defines custom marshaling logic
// using the BinaryMarshaler interface. This type implements that interface and simply does an
// unsafe pointer cast, avoiding the reflection and allocation overhead entirely.
//
// Other than binary.Write this type does *NOT* perform *ANY* sanity checks. Users need to ensure
// that their T only contains primitive types, aliases of primitive types, or structs of them.
// Using a T that contains high-level Go types like slices, maps or pointers is undefined behavior.
type ptrCastMarshaler[T any] []T

func (r ptrCastMarshaler[T]) MarshalBinary() (data []byte, err error) {
	return pfunsafe.FromSlice(r), nil
}

// generateSlice returns a slice of type T and populates every value with its index.
func generateSlice[T constraints.Unsigned](num int) ptrCastMarshaler[T] {
	keys := make([]T, num)
	for k := range keys {
		keys[k] = T(k)
	}
	return keys
}
