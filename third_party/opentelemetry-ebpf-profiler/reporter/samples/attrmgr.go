// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package samples // import "go.opentelemetry.io/ebpf-profiler/reporter/samples"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/otel/attribute"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/reporter/internal/orderedset"
)

// SampleAttrProducer provides a hook point to:
//
// - inspect each trace and its meta when it is enqueued in the reporter
// - produce extra meta info
// - attach extra attributes to the trace
type SampleAttrProducer interface {
	// CollectExtraSampleMeta gathers extra sample meta-info and returns it as
	// a pointer to a **hashable** struct.
	CollectExtraSampleMeta(trace *libpf.Trace, meta *TraceEventMeta) any

	// ExtraSampleAttrs is called when the reporter populates the Sample struct
	// before sending it out. Attributes returned from this function are added
	// as Sample attributes. `meta` receives the pointer that was returned from
	// CollectExtraSampleMeta.
	ExtraSampleAttrs(attrMgr *AttrTableManager, meta any) []int32
}

// AttrTableManager maintains index allocation and deduplication for attribute tables.
type AttrTableManager struct {
	// indices maps compound keys to the indices in the attribute table.
	indices map[string]int32

	// strSet to read and write strings from the string table
	strSet orderedset.OrderedSet[string]

	// attrTable being populated.
	attrTable pprofile.KeyValueAndUnitSlice
}

func NewAttrTableManager(strSet orderedset.OrderedSet[string], attrTable pprofile.KeyValueAndUnitSlice) *AttrTableManager {
	return &AttrTableManager{
		indices:   make(map[string]int32),
		strSet:    strSet,
		attrTable: attrTable,
	}
}

// AppendInt adds the index for the given integer attribute to an attribute index slice.
func (m *AttrTableManager) AppendInt(
	attrs pcommon.Int32Slice, key attribute.Key, value int64,
) {
	compound := fmt.Sprintf("%v_%d", key, value)
	m.appendAny(attrs, key, compound, value)
}

// AppendOptionalString adds the index for the given string attribute to an
// attribute index slice if it is non-empty.
func (m *AttrTableManager) AppendOptionalString(
	attrs pcommon.Int32Slice, key attribute.Key, value string,
) {
	if value == "" {
		return
	}

	compound := fmt.Sprintf("%v_%s", key, value)
	m.appendAny(attrs, key, compound, value)
}

func (m *AttrTableManager) appendAny(
	attrs pcommon.Int32Slice,
	key attribute.Key,
	compoundKey string,
	value any,
) {
	if attributeIndex, exists := m.indices[compoundKey]; exists {
		attrs.Append(attributeIndex)
		return
	}

	newIndex := int32(m.attrTable.Len())

	a := m.attrTable.AppendEmpty()
	a.SetKeyStrindex(m.strSet.Add(string(key)))

	switch v := value.(type) {
	case int64:
		a.Value().SetInt(v)
	case string:
		a.Value().SetStr(v)
	}
	m.indices[compoundKey] = newIndex
	attrs.Append(newIndex)
}
