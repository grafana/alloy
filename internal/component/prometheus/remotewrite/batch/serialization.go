package batch

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type Header uint

func (h Header) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("HEADER")
	}
	bb.addUint(uint32(h))
}

func HeaderDeserialize(bb *buffer) Header {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "HEADER")
	}

	return Header(bb.readUint())
}

type Timestamp int64

func (t Timestamp) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("TIMESTAMP")
	}

	bb.addInt64(int64(t))
}

func TimestampDeserialize(bb *buffer) Timestamp {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "TIMESTAMP")

	}

	return Timestamp(bb.readInt64())
}

type StringArray []string

func (sd StringArray) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("STRING_ARRAY")
	}
	bb.addUint(uint32(len(sd)))
	for _, s := range sd {
		bb.addUint(uint32(len(s)))
		bb.WriteString(s)
	}
}

func newStringArray(dict map[int]string) StringArray {
	arr := make([]string, len(dict)+1)
	arr[none_index] = "NONE"
	for i := 1; i <= len(dict); i++ {
		arr[i] = dict[i]
	}
	return arr
}

func StringArrayDeserialize(bb *buffer) StringArray {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "STRING_ARRAY")
	}
	// Get length of the dictionary
	total := int(bb.readUint())
	dict := make([]string, total)
	for i := 0; i < total; i++ {
		dict[i] = bb.readString()
	}
	return dict
}

type TimestampCount uint32

func (tc TimestampCount) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("TIMESTAMP_COUNT")
	}

	bb.addUint(uint32(tc))
}

func TimestampCountDeserialize(bb *buffer) TimestampCount {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "TIMESTAMP_COUNT")
	}

	return TimestampCount(bb.readUint())
}

type MetricCount uint32

func (mc MetricCount) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("METRIC_COUNT")
	}

	bb.addUint(uint32(mc))
}

func MetricCountDeserialize(bb *buffer) MetricCount {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "METRIC_COUNT")
	}

	return MetricCount(bb.readUint())
}

type SignalType TelemetryType

func (st SignalType) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("SIGNAL_TYPE")
	}
	bb.addUint(uint32(st))
}

func SignalTypeDeserialize(bb *buffer) SignalType {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "SIGNAL_TYPE")

	}
	return SignalType(bb.readUint())
}

type Value uint64

func (v Value) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("VALUE")
	}
	bb.addUInt64(uint64(v))
}

func ValueDeserialize(bb *buffer) Value {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "VALUE")

	}
	return Value(bb.readUint64())
}

type LabelCount uint32

func (lc LabelCount) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("LABEL_COUNT")
	}
	bb.addUint(uint32(lc))
}

func LabelCountDeserialize(bb *buffer) LabelCount {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "LABEL_COUNT")
	}
	return LabelCount(bb.readUint())
}

type ExemplarLabelCount uint32

func (lc ExemplarLabelCount) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("EXEMPLAR_LABEL_COUNT")
	}
	bb.addUint(uint32(lc))
}

func ExemplarLabelCountDeserialize(bb *buffer) ExemplarLabelCount {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "EXEMPLAR_LABEL_COUNT")
	}
	return ExemplarLabelCount(bb.readUint())
}

type LabelValueID uint32

func (li LabelValueID) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("LABEL_VALUE_ID")
	}
	bb.addUint(uint32(li))
}

func LabelValueIDDeserialize(bb *buffer) LabelValueID {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "LABEL_VALUE_ID")
	}
	return LabelValueID(bb.readUint())
}

type LabelNameID uint32

func (li LabelNameID) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("LABEL_NAME_ID")
	}
	bb.addUint(uint32(li))
}

func LabelNameIDDeserialize(bb *buffer) LabelNameID {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "LABEL_NAME_ID")
	}
	return LabelNameID(bb.readUint())
}

func checkVal(actual, expected string) {
	if actual != expected {
		panic(fmt.Sprintf("Expected %s, got %s", expected, actual))
	}
}

type buffer struct {
	*bytes.Buffer
	tb           []byte
	tb64         []byte
	stringbuffer []byte
	debug        bool
}

func (bb *buffer) addUint(num uint32) {
	binary.LittleEndian.PutUint32(bb.tb, num)
	bb.Write(bb.tb)
}

func (bb *buffer) readUint() uint32 {
	_, _ = bb.Read(bb.tb)
	return binary.LittleEndian.Uint32(bb.tb)
}

func (bb *buffer) readUint64() uint64 {
	_, _ = bb.Read(bb.tb64)
	return binary.LittleEndian.Uint64(bb.tb64)
}

func (bb *buffer) readInt64() int64 {
	_, _ = bb.Read(bb.tb64)
	return int64(binary.LittleEndian.Uint64(bb.tb64))
}

func (bb *buffer) writeString(s string) {
	bb.addUint(uint32(len(s)))
	_, _ = bb.WriteString(s)
}

func (bb *buffer) addInt64(num int64) {
	binary.LittleEndian.PutUint64(bb.tb64, uint64(num))
	bb.Write(bb.tb64)
}

func (bb *buffer) addUInt64(num uint64) {
	binary.LittleEndian.PutUint64(bb.tb64, num)
	bb.Write(bb.tb64)
}

func (bb *buffer) readString() string {
	length := bb.readUint()
	if cap(bb.stringbuffer) < int(length) {
		bb.stringbuffer = make([]byte, length)
	} else {
		bb.stringbuffer = bb.stringbuffer[:int(length)]
	}
	_, _ = bb.Read(bb.stringbuffer)
	return string(bb.stringbuffer)
}
