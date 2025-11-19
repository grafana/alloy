// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pdata // import "go.opentelemetry.io/ebpf-profiler/reporter/internal/pdata"

import (
	"fmt"
	"math"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/otel/attribute"

	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/reporter/internal/orderedset"
	"go.opentelemetry.io/ebpf-profiler/reporter/samples"
	"go.opentelemetry.io/ebpf-profiler/support"
)

const (
	ExecutableCacheLifetime = 1 * time.Hour
)

// uniqueMapping defines an unique mapping in a process.
type uniqueMapping struct {
	// mapping start in the ELF virtual address space
	Start libpf.Address
	// mapping file
	File libpf.FrameMappingFile
}

// Generate generates a pdata request out of internal profiles data, to be
// exported.
func (p *Pdata) Generate(tree samples.TraceEventsTree,
	agentName, agentVersion string,
) (pprofile.Profiles, error) {
	profiles := pprofile.NewProfiles()
	dic := profiles.Dictionary()

	// Temporary helpers that will build the various tables in ProfilesDictionary.
	stringSet := make(orderedset.OrderedSet[string], 64)
	funcSet := make(orderedset.OrderedSet[funcInfo], 64)
	mappingSet := make(orderedset.OrderedSet[uniqueMapping], 64)
	stackSet := make(orderedset.OrderedSet[stackInfo], 64)
	locationSet := make(orderedset.OrderedSet[locationInfo], 64)

	// By specification, the first element should be empty.
	stringSet.Add("")
	funcSet.Add(funcInfo{})
	mappingSet.Add(uniqueMapping{})
	stackSet.Add(stackInfo{})
	locationSet.Add(locationInfo{})

	dic.LinkTable().AppendEmpty()
	dic.MappingTable().AppendEmpty()
	dic.StackTable().AppendEmpty()
	dic.AttributeTable().AppendEmpty()
	dic.LocationTable().AppendEmpty()

	attrMgr := samples.NewAttrTableManager(stringSet, dic.AttributeTable())

	for containerID, originToEvents := range tree {
		if len(originToEvents) == 0 {
			continue
		}

		rp := profiles.ResourceProfiles().AppendEmpty()
		rp.Resource().Attributes().PutStr(string(semconv.ContainerIDKey),
			string(containerID))
		rp.SetSchemaUrl(semconv.SchemaURL)

		sp := rp.ScopeProfiles().AppendEmpty()
		sp.Scope().SetName(agentName)
		sp.Scope().SetVersion(agentVersion)
		sp.SetSchemaUrl(semconv.SchemaURL)

		for _, origin := range []libpf.Origin{
			support.TraceOriginSampling,
			support.TraceOriginOffCPU,
			support.TraceOriginUProbe,
		} {
			if len(originToEvents[origin]) == 0 {
				// Do not append empty profiles.
				continue
			}

			prof := sp.Profiles().AppendEmpty()
			if err := p.setProfile(dic,
				attrMgr, stringSet, funcSet, mappingSet, stackSet, locationSet,
				origin, originToEvents[origin], prof); err != nil {
				return profiles, err
			}
		}
	}

	// Populate the ProfilesDictionary tables.
	funcTable := dic.FunctionTable()
	funcTable.EnsureCapacity(len(funcSet))
	for range funcSet {
		funcTable.AppendEmpty()
	}
	for v, idx := range funcSet {
		f := funcTable.At(int(idx))
		f.SetNameStrindex(v.nameIdx)
		f.SetFilenameStrindex(v.fileNameIdx)
	}

	stringTable := dic.StringTable()
	stringTable.EnsureCapacity(len(stringSet))
	for _, val := range stringSet.ToSlice() {
		stringTable.Append(val)
	}

	return profiles, nil
}

// setProfile sets the data an OTLP profile with all collected samples up to
// this moment.
func (p *Pdata) setProfile(
	dic pprofile.ProfilesDictionary,
	attrMgr *samples.AttrTableManager,
	stringSet orderedset.OrderedSet[string],
	funcSet orderedset.OrderedSet[funcInfo],
	mappingSet orderedset.OrderedSet[uniqueMapping],
	stackSet orderedset.OrderedSet[stackInfo],
	locationSet orderedset.OrderedSet[locationInfo],
	origin libpf.Origin,
	events map[samples.TraceAndMetaKey]*samples.TraceEvents,
	profile pprofile.Profile,
) error {
	st := profile.SampleType()
	switch origin {
	case support.TraceOriginSampling:
		profile.SetPeriod(1e9 / int64(p.samplesPerSecond))
		pt := profile.PeriodType()
		pt.SetTypeStrindex(stringSet.Add("cpu"))
		pt.SetUnitStrindex(stringSet.Add("nanoseconds"))

		st.SetTypeStrindex(stringSet.Add("samples"))
		st.SetUnitStrindex(stringSet.Add("count"))
	case support.TraceOriginOffCPU:
		st.SetTypeStrindex(stringSet.Add("off_cpu"))
		st.SetUnitStrindex(stringSet.Add("nanoseconds"))
	case support.TraceOriginUProbe:
		st.SetTypeStrindex(stringSet.Add("events"))
		st.SetUnitStrindex(stringSet.Add("count"))
	default:
		// Should never happen
		return fmt.Errorf("generating profile for unsupported origin %d", origin)
	}

	startTS, endTS := uint64(math.MaxUint64), uint64(0)
	for traceKey, traceInfo := range events {
		sample := profile.Samples().AppendEmpty()

		for _, ts := range traceInfo.Timestamps {
			startTS = min(startTS, ts)
			endTS = max(endTS, ts)
		}

		sample.TimestampsUnixNano().FromRaw(traceInfo.Timestamps)
		if origin == support.TraceOriginOffCPU {
			sample.Values().Append(traceInfo.OffTimes...)
		}

		locationIndices := make([]int32, 0, len(traceInfo.Frames))
		// Walk every frame of the trace.
		for _, uniqueFrame := range traceInfo.Frames {
			frame := uniqueFrame.Value()
			locInfo := locationInfo{
				address:   uint64(frame.AddressOrLineno),
				frameType: frame.Type.String(),
			}

			if frame.MappingFile.Valid() {
				index, ok := mappingSet.AddWithCheck(uniqueMapping{
					Start: frame.MappingStart,
					File:  frame.MappingFile,
				})
				if !ok {
					mf := frame.MappingFile.Value()
					mapping := dic.MappingTable().AppendEmpty()
					mapping.SetMemoryStart(uint64(frame.MappingStart))
					mapping.SetMemoryLimit(uint64(frame.MappingEnd))
					mapping.SetFileOffset(frame.MappingFileOffset)
					mapping.SetFilenameStrindex(stringSet.Add(mf.FileName.String()))

					attrMgr.AppendOptionalString(mapping.AttributeIndices(),
						semconv.ProcessExecutableBuildIDGNUKey,
						mf.GnuBuildID)
					attrMgr.AppendOptionalString(mapping.AttributeIndices(),
						semconv.ProcessExecutableBuildIDGoKey,
						mf.GoBuildID)
					attrMgr.AppendOptionalString(mapping.AttributeIndices(),
						semconv.ProcessExecutableBuildIDHtlhashKey,
						mf.FileID.StringNoQuotes())
				}
				locInfo.mappingIndex = index
			} else {
				locInfo.mappingIndex = 0
			}

			if frame.FunctionName != libpf.NullString || frame.SourceFile != libpf.NullString {
				// Store interpreted frame information as a Line message
				locInfo.hasLine = true
				locInfo.lineNumber = int64(frame.SourceLine)
				fi := funcInfo{
					nameIdx:     stringSet.Add(frame.FunctionName.String()),
					fileNameIdx: stringSet.Add(frame.SourceFile.String()),
				}
				locInfo.functionIndex = funcSet.Add(fi)
			}

			idx, exists := locationSet.AddWithCheck(locInfo)
			if !exists {
				// Add a new Location to the dictionary
				loc := dic.LocationTable().AppendEmpty()
				loc.SetAddress(locInfo.address)
				loc.SetMappingIndex(locInfo.mappingIndex)
				if locInfo.hasLine {
					line := loc.Lines().AppendEmpty()
					line.SetLine(locInfo.lineNumber)
					line.SetFunctionIndex(locInfo.functionIndex)
				}
				attrMgr.AppendOptionalString(loc.AttributeIndices(),
					semconv.ProfileFrameTypeKey, locInfo.frameType)
			}
			locationIndices = append(locationIndices, idx)
		} // End per-frame processing

		stackIdx, exists := stackSet.AddWithCheck(stackInfo{
			locationIndices: fmt.Sprintf("%v", locationIndices)})
		if !exists {
			// Add a new Stack to the dictionary
			stack := dic.StackTable().AppendEmpty()
			for _, locIdx := range locationIndices {
				stack.LocationIndices().Append(locIdx)
			}
		}
		sample.SetStackIndex(stackIdx)

		exeName := traceKey.ExecutablePath
		if exeName != "" {
			_, exeName = filepath.Split(exeName)
		}

		attrMgr.AppendOptionalString(sample.AttributeIndices(),
			semconv.ThreadNameKey, traceKey.Comm)

		attrMgr.AppendOptionalString(sample.AttributeIndices(),
			semconv.ProcessExecutableNameKey, exeName)
		attrMgr.AppendOptionalString(sample.AttributeIndices(),
			semconv.ProcessExecutablePathKey, traceKey.ExecutablePath)
		attrMgr.AppendInt(sample.AttributeIndices(),
			semconv.ProcessPIDKey, traceKey.Pid)
		attrMgr.AppendInt(sample.AttributeIndices(),
			semconv.ThreadIDKey, traceKey.Tid)

		for key, value := range traceInfo.EnvVars {
			env := semconv.ProcessEnvironmentVariable(key, value)
			attrMgr.AppendOptionalString(
				sample.AttributeIndices(),
				env.Key, env.Value.AsString())
		}
		for key, value := range traceInfo.Labels {
			// Once https://github.com/open-telemetry/semantic-conventions/issues/2561
			// reached an agreement, use the actual OTel SemConv attribute.
			attrMgr.AppendOptionalString(
				sample.AttributeIndices(),
				attribute.Key("process.context.label."+key),
				value)
		}

		if p.ExtraSampleAttrProd != nil {
			extra := p.ExtraSampleAttrProd.ExtraSampleAttrs(attrMgr, traceKey.ExtraMeta)
			sample.AttributeIndices().Append(extra...)
		}
	} // End sample processing

	log.Debugf("Reporting OTLP profile with %d samples", profile.Samples().Len())

	profile.SetDuration(pcommon.Timestamp(endTS - startTS))
	profile.SetTime(pcommon.Timestamp(startTS))

	return nil
}
