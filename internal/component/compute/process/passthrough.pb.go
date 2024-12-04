// Code generated by protoc-gen-go-lite. DO NOT EDIT.
// protoc-gen-go-lite version: v0.8.0
// source: passthrough.proto

package process

import (
	binary "encoding/binary"
	fmt "fmt"
	protobuf_go_lite "github.com/aperturerobotics/protobuf-go-lite"
	io "io"
	math "math"
)

type Passthrough struct {
	unknownFields []byte
	Metrics       [][]byte            `protobuf:"bytes,1,rep,name=metrics,proto3" json:"metrics,omitempty"`
	Traces        [][]byte            `protobuf:"bytes,2,rep,name=traces,proto3" json:"traces,omitempty"`
	Logs          [][]byte            `protobuf:"bytes,3,rep,name=logs,proto3" json:"logs,omitempty"`
	Config        map[string]string   `protobuf:"bytes,4,rep,name=config,proto3" json:"config,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Prommetrics   []*PrometheusMetric `protobuf:"bytes,5,rep,name=prommetrics,proto3" json:"prommetrics,omitempty"`
	Lokilogs      []*LokiLog          `protobuf:"bytes,6,rep,name=lokilogs,proto3" json:"lokilogs,omitempty"`
}

func (x *Passthrough) Reset() {
	*x = Passthrough{}
}

func (*Passthrough) ProtoMessage() {}

func (x *Passthrough) GetMetrics() [][]byte {
	if x != nil {
		return x.Metrics
	}
	return nil
}

func (x *Passthrough) GetTraces() [][]byte {
	if x != nil {
		return x.Traces
	}
	return nil
}

func (x *Passthrough) GetLogs() [][]byte {
	if x != nil {
		return x.Logs
	}
	return nil
}

func (x *Passthrough) GetConfig() map[string]string {
	if x != nil {
		return x.Config
	}
	return nil
}

func (x *Passthrough) GetPrommetrics() []*PrometheusMetric {
	if x != nil {
		return x.Prommetrics
	}
	return nil
}

func (x *Passthrough) GetLokilogs() []*LokiLog {
	if x != nil {
		return x.Lokilogs
	}
	return nil
}

type LokiLog struct {
	unknownFields []byte
	Labels        []*Label `protobuf:"bytes,1,rep,name=labels,proto3" json:"labels,omitempty"`
	Timestamp     int64    `protobuf:"varint,3,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	Line          string   `protobuf:"bytes,4,opt,name=line,proto3" json:"line,omitempty"`
	Metadata      []*Label `protobuf:"bytes,5,rep,name=metadata,proto3" json:"metadata,omitempty"`
	Parsed        []*Label `protobuf:"bytes,6,rep,name=parsed,proto3" json:"parsed,omitempty"`
}

func (x *LokiLog) Reset() {
	*x = LokiLog{}
}

func (*LokiLog) ProtoMessage() {}

func (x *LokiLog) GetLabels() []*Label {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *LokiLog) GetTimestamp() int64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}

func (x *LokiLog) GetLine() string {
	if x != nil {
		return x.Line
	}
	return ""
}

func (x *LokiLog) GetMetadata() []*Label {
	if x != nil {
		return x.Metadata
	}
	return nil
}

func (x *LokiLog) GetParsed() []*Label {
	if x != nil {
		return x.Parsed
	}
	return nil
}

type PrometheusMetric struct {
	unknownFields []byte
	Labels        []*Label `protobuf:"bytes,1,rep,name=Labels,proto3" json:"Labels,omitempty"`
	Value         float64  `protobuf:"fixed64,2,opt,name=value,proto3" json:"value,omitempty"`
	Timestampms   int64    `protobuf:"varint,3,opt,name=timestampms,proto3" json:"timestampms,omitempty"`
}

func (x *PrometheusMetric) Reset() {
	*x = PrometheusMetric{}
}

func (*PrometheusMetric) ProtoMessage() {}

func (x *PrometheusMetric) GetLabels() []*Label {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *PrometheusMetric) GetValue() float64 {
	if x != nil {
		return x.Value
	}
	return 0
}

func (x *PrometheusMetric) GetTimestampms() int64 {
	if x != nil {
		return x.Timestampms
	}
	return 0
}

type Label struct {
	unknownFields []byte
	Name          string `protobuf:"bytes,1,opt,name=Name,proto3" json:"Name,omitempty"`
	Value         string `protobuf:"bytes,2,opt,name=Value,proto3" json:"Value,omitempty"`
}

func (x *Label) Reset() {
	*x = Label{}
}

func (*Label) ProtoMessage() {}

func (x *Label) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Label) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

type Passthrough_ConfigEntry struct {
	unknownFields []byte
	Key           string `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Value         string `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *Passthrough_ConfigEntry) Reset() {
	*x = Passthrough_ConfigEntry{}
}

func (*Passthrough_ConfigEntry) ProtoMessage() {}

func (x *Passthrough_ConfigEntry) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *Passthrough_ConfigEntry) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

func (m *Passthrough) CloneVT() *Passthrough {
	if m == nil {
		return (*Passthrough)(nil)
	}
	r := new(Passthrough)
	if rhs := m.Metrics; rhs != nil {
		tmpContainer := make([][]byte, len(rhs))
		for k, v := range rhs {
			tmpBytes := make([]byte, len(v))
			copy(tmpBytes, v)
			tmpContainer[k] = tmpBytes
		}
		r.Metrics = tmpContainer
	}
	if rhs := m.Traces; rhs != nil {
		tmpContainer := make([][]byte, len(rhs))
		for k, v := range rhs {
			tmpBytes := make([]byte, len(v))
			copy(tmpBytes, v)
			tmpContainer[k] = tmpBytes
		}
		r.Traces = tmpContainer
	}
	if rhs := m.Logs; rhs != nil {
		tmpContainer := make([][]byte, len(rhs))
		for k, v := range rhs {
			tmpBytes := make([]byte, len(v))
			copy(tmpBytes, v)
			tmpContainer[k] = tmpBytes
		}
		r.Logs = tmpContainer
	}
	if rhs := m.Config; rhs != nil {
		tmpContainer := make(map[string]string, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v
		}
		r.Config = tmpContainer
	}
	if rhs := m.Prommetrics; rhs != nil {
		tmpContainer := make([]*PrometheusMetric, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Prommetrics = tmpContainer
	}
	if rhs := m.Lokilogs; rhs != nil {
		tmpContainer := make([]*LokiLog, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Lokilogs = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *Passthrough) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *LokiLog) CloneVT() *LokiLog {
	if m == nil {
		return (*LokiLog)(nil)
	}
	r := new(LokiLog)
	r.Timestamp = m.Timestamp
	r.Line = m.Line
	if rhs := m.Labels; rhs != nil {
		tmpContainer := make([]*Label, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Labels = tmpContainer
	}
	if rhs := m.Metadata; rhs != nil {
		tmpContainer := make([]*Label, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Metadata = tmpContainer
	}
	if rhs := m.Parsed; rhs != nil {
		tmpContainer := make([]*Label, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Parsed = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *LokiLog) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *PrometheusMetric) CloneVT() *PrometheusMetric {
	if m == nil {
		return (*PrometheusMetric)(nil)
	}
	r := new(PrometheusMetric)
	r.Value = m.Value
	r.Timestampms = m.Timestampms
	if rhs := m.Labels; rhs != nil {
		tmpContainer := make([]*Label, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Labels = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *PrometheusMetric) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *Label) CloneVT() *Label {
	if m == nil {
		return (*Label)(nil)
	}
	r := new(Label)
	r.Name = m.Name
	r.Value = m.Value
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *Label) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (this *Passthrough) EqualVT(that *Passthrough) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if len(this.Metrics) != len(that.Metrics) {
		return false
	}
	for i, vx := range this.Metrics {
		vy := that.Metrics[i]
		if string(vx) != string(vy) {
			return false
		}
	}
	if len(this.Traces) != len(that.Traces) {
		return false
	}
	for i, vx := range this.Traces {
		vy := that.Traces[i]
		if string(vx) != string(vy) {
			return false
		}
	}
	if len(this.Logs) != len(that.Logs) {
		return false
	}
	for i, vx := range this.Logs {
		vy := that.Logs[i]
		if string(vx) != string(vy) {
			return false
		}
	}
	if len(this.Config) != len(that.Config) {
		return false
	}
	for i, vx := range this.Config {
		vy, ok := that.Config[i]
		if !ok {
			return false
		}
		if vx != vy {
			return false
		}
	}
	if len(this.Prommetrics) != len(that.Prommetrics) {
		return false
	}
	for i, vx := range this.Prommetrics {
		vy := that.Prommetrics[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &PrometheusMetric{}
			}
			if q == nil {
				q = &PrometheusMetric{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if len(this.Lokilogs) != len(that.Lokilogs) {
		return false
	}
	for i, vx := range this.Lokilogs {
		vy := that.Lokilogs[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &LokiLog{}
			}
			if q == nil {
				q = &LokiLog{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *Passthrough) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*Passthrough)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *LokiLog) EqualVT(that *LokiLog) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if len(this.Labels) != len(that.Labels) {
		return false
	}
	for i, vx := range this.Labels {
		vy := that.Labels[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &Label{}
			}
			if q == nil {
				q = &Label{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if this.Timestamp != that.Timestamp {
		return false
	}
	if this.Line != that.Line {
		return false
	}
	if len(this.Metadata) != len(that.Metadata) {
		return false
	}
	for i, vx := range this.Metadata {
		vy := that.Metadata[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &Label{}
			}
			if q == nil {
				q = &Label{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if len(this.Parsed) != len(that.Parsed) {
		return false
	}
	for i, vx := range this.Parsed {
		vy := that.Parsed[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &Label{}
			}
			if q == nil {
				q = &Label{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *LokiLog) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*LokiLog)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *PrometheusMetric) EqualVT(that *PrometheusMetric) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if len(this.Labels) != len(that.Labels) {
		return false
	}
	for i, vx := range this.Labels {
		vy := that.Labels[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &Label{}
			}
			if q == nil {
				q = &Label{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if this.Value != that.Value {
		return false
	}
	if this.Timestampms != that.Timestampms {
		return false
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *PrometheusMetric) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*PrometheusMetric)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *Label) EqualVT(that *Label) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if this.Name != that.Name {
		return false
	}
	if this.Value != that.Value {
		return false
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *Label) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*Label)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (m *Passthrough) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Passthrough) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Passthrough) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if len(m.Lokilogs) > 0 {
		for iNdEx := len(m.Lokilogs) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Lokilogs[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x32
		}
	}
	if len(m.Prommetrics) > 0 {
		for iNdEx := len(m.Prommetrics) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Prommetrics[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x2a
		}
	}
	if len(m.Config) > 0 {
		for k := range m.Config {
			v := m.Config[k]
			baseI := i
			i -= len(v)
			copy(dAtA[i:], v)
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(v)))
			i--
			dAtA[i] = 0x12
			i -= len(k)
			copy(dAtA[i:], k)
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(k)))
			i--
			dAtA[i] = 0xa
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(baseI-i))
			i--
			dAtA[i] = 0x22
		}
	}
	if len(m.Logs) > 0 {
		for iNdEx := len(m.Logs) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.Logs[iNdEx])
			copy(dAtA[i:], m.Logs[iNdEx])
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.Logs[iNdEx])))
			i--
			dAtA[i] = 0x1a
		}
	}
	if len(m.Traces) > 0 {
		for iNdEx := len(m.Traces) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.Traces[iNdEx])
			copy(dAtA[i:], m.Traces[iNdEx])
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.Traces[iNdEx])))
			i--
			dAtA[i] = 0x12
		}
	}
	if len(m.Metrics) > 0 {
		for iNdEx := len(m.Metrics) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.Metrics[iNdEx])
			copy(dAtA[i:], m.Metrics[iNdEx])
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.Metrics[iNdEx])))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *LokiLog) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *LokiLog) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *LokiLog) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if len(m.Parsed) > 0 {
		for iNdEx := len(m.Parsed) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Parsed[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x32
		}
	}
	if len(m.Metadata) > 0 {
		for iNdEx := len(m.Metadata) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Metadata[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x2a
		}
	}
	if len(m.Line) > 0 {
		i -= len(m.Line)
		copy(dAtA[i:], m.Line)
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.Line)))
		i--
		dAtA[i] = 0x22
	}
	if m.Timestamp != 0 {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(m.Timestamp))
		i--
		dAtA[i] = 0x18
	}
	if len(m.Labels) > 0 {
		for iNdEx := len(m.Labels) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Labels[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *PrometheusMetric) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PrometheusMetric) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *PrometheusMetric) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if m.Timestampms != 0 {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(m.Timestampms))
		i--
		dAtA[i] = 0x18
	}
	if m.Value != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.Value))))
		i--
		dAtA[i] = 0x11
	}
	if len(m.Labels) > 0 {
		for iNdEx := len(m.Labels) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Labels[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *Label) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Label) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Label) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if len(m.Value) > 0 {
		i -= len(m.Value)
		copy(dAtA[i:], m.Value)
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.Value)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.Name) > 0 {
		i -= len(m.Name)
		copy(dAtA[i:], m.Name)
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.Name)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *Passthrough) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Metrics) > 0 {
		for _, b := range m.Metrics {
			l = len(b)
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if len(m.Traces) > 0 {
		for _, b := range m.Traces {
			l = len(b)
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if len(m.Logs) > 0 {
		for _, b := range m.Logs {
			l = len(b)
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if len(m.Config) > 0 {
		for k, v := range m.Config {
			_ = k
			_ = v
			mapEntrySize := 1 + len(k) + protobuf_go_lite.SizeOfVarint(uint64(len(k))) + 1 + len(v) + protobuf_go_lite.SizeOfVarint(uint64(len(v)))
			n += mapEntrySize + 1 + protobuf_go_lite.SizeOfVarint(uint64(mapEntrySize))
		}
	}
	if len(m.Prommetrics) > 0 {
		for _, e := range m.Prommetrics {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if len(m.Lokilogs) > 0 {
		for _, e := range m.Lokilogs {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	n += len(m.unknownFields)
	return n
}

func (m *LokiLog) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Labels) > 0 {
		for _, e := range m.Labels {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if m.Timestamp != 0 {
		n += 1 + protobuf_go_lite.SizeOfVarint(uint64(m.Timestamp))
	}
	l = len(m.Line)
	if l > 0 {
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	if len(m.Metadata) > 0 {
		for _, e := range m.Metadata {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if len(m.Parsed) > 0 {
		for _, e := range m.Parsed {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	n += len(m.unknownFields)
	return n
}

func (m *PrometheusMetric) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Labels) > 0 {
		for _, e := range m.Labels {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if m.Value != 0 {
		n += 9
	}
	if m.Timestampms != 0 {
		n += 1 + protobuf_go_lite.SizeOfVarint(uint64(m.Timestampms))
	}
	n += len(m.unknownFields)
	return n
}

func (m *Label) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Name)
	if l > 0 {
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	l = len(m.Value)
	if l > 0 {
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	n += len(m.unknownFields)
	return n
}

func (m *Passthrough) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Passthrough: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Passthrough: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Metrics", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Metrics = append(m.Metrics, make([]byte, postIndex-iNdEx))
			copy(m.Metrics[len(m.Metrics)-1], dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Traces", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Traces = append(m.Traces, make([]byte, postIndex-iNdEx))
			copy(m.Traces[len(m.Traces)-1], dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Logs", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Logs = append(m.Logs, make([]byte, postIndex-iNdEx))
			copy(m.Logs[len(m.Logs)-1], dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Config", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Config == nil {
				m.Config = make(map[string]string)
			}
			var mapkey string
			var mapvalue string
			for iNdEx < postIndex {
				entryPreIndex := iNdEx
				var wire uint64
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return protobuf_go_lite.ErrIntOverflow
					}
					if iNdEx >= l {
						return io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					wire |= uint64(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				fieldNum := int32(wire >> 3)
				if fieldNum == 1 {
					var stringLenmapkey uint64
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return protobuf_go_lite.ErrIntOverflow
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						stringLenmapkey |= uint64(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					intStringLenmapkey := int(stringLenmapkey)
					if intStringLenmapkey < 0 {
						return protobuf_go_lite.ErrInvalidLength
					}
					postStringIndexmapkey := iNdEx + intStringLenmapkey
					if postStringIndexmapkey < 0 {
						return protobuf_go_lite.ErrInvalidLength
					}
					if postStringIndexmapkey > l {
						return io.ErrUnexpectedEOF
					}
					mapkey = string(dAtA[iNdEx:postStringIndexmapkey])
					iNdEx = postStringIndexmapkey
				} else if fieldNum == 2 {
					var stringLenmapvalue uint64
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return protobuf_go_lite.ErrIntOverflow
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						stringLenmapvalue |= uint64(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					intStringLenmapvalue := int(stringLenmapvalue)
					if intStringLenmapvalue < 0 {
						return protobuf_go_lite.ErrInvalidLength
					}
					postStringIndexmapvalue := iNdEx + intStringLenmapvalue
					if postStringIndexmapvalue < 0 {
						return protobuf_go_lite.ErrInvalidLength
					}
					if postStringIndexmapvalue > l {
						return io.ErrUnexpectedEOF
					}
					mapvalue = string(dAtA[iNdEx:postStringIndexmapvalue])
					iNdEx = postStringIndexmapvalue
				} else {
					iNdEx = entryPreIndex
					skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
					if err != nil {
						return err
					}
					if (skippy < 0) || (iNdEx+skippy) < 0 {
						return protobuf_go_lite.ErrInvalidLength
					}
					if (iNdEx + skippy) > postIndex {
						return io.ErrUnexpectedEOF
					}
					iNdEx += skippy
				}
			}
			m.Config[mapkey] = mapvalue
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Prommetrics", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Prommetrics = append(m.Prommetrics, &PrometheusMetric{})
			if err := m.Prommetrics[len(m.Prommetrics)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Lokilogs", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Lokilogs = append(m.Lokilogs, &LokiLog{})
			if err := m.Lokilogs[len(m.Lokilogs)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *LokiLog) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: LokiLog: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: LokiLog: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Labels", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Labels = append(m.Labels, &Label{})
			if err := m.Labels[len(m.Labels)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Timestamp", wireType)
			}
			m.Timestamp = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Timestamp |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Line", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Line = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Metadata", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Metadata = append(m.Metadata, &Label{})
			if err := m.Metadata[len(m.Metadata)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Parsed", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Parsed = append(m.Parsed, &Label{})
			if err := m.Parsed[len(m.Parsed)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *PrometheusMetric) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: PrometheusMetric: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: PrometheusMetric: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Labels", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Labels = append(m.Labels, &Label{})
			if err := m.Labels[len(m.Labels)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Value", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Value = float64(math.Float64frombits(v))
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Timestampms", wireType)
			}
			m.Timestampms = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Timestampms |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Label) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Label: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Label: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Name", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Name = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Value", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Value = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
