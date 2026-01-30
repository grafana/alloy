// Package zapadapter allows github.com/go-kit/log to be used as a Zap core.
package zapadapter

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New returns a new zap.Logger instance which will forward logs to the
// provided log.Logger. The github.com/go-kit/log/level package will be used
// for specifying log levels.
func New(l log.Logger) *zap.Logger {
	return zap.New(&loggerCore{inner: l})
}

// loggerCore is a zap.Core implementation which forwards logs to a log.Logger
// instance.
type loggerCore struct {
	inner log.Logger
}

var _ zapcore.Core = (*loggerCore)(nil)

// Enabled implements zapcore.Core and returns whether logs at a specific level
// should be reported.
func (lc *loggerCore) Enabled(zapcore.Level) bool {
	// An instance of log.Logger has no way of knowing if logs will be filtered
	// out, so we always have to return true here.
	return true
}

// With implements zapcore.Core, returning a new logger core with ff appended
// to the list of fields.
func (lc *loggerCore) With(ff []zapcore.Field) zapcore.Core {
	// Encode all the fields so that they're go-kit compatible and create a
	// new logger from it.
	enc := newFieldEncoder()
	defer func() { _ = enc.Close() }()

	for _, f := range ff {
		f.AddTo(enc)
	}

	return &loggerCore{
		inner: log.With(lc.inner, enc.fields...),
	}
}

// Check implements zapcore.Core. lc will always add itself along with the
// provided entry to the CheckedEntry.
func (lc *loggerCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(e, lc)
}

// Write serializes e with the provided list of fields, writing them to the
// underlying github.com/go-kit/log.Logger instance.
func (lc *loggerCore) Write(e zapcore.Entry, ff []zapcore.Field) error {
	enc := newFieldEncoder()
	defer func() { _ = enc.Close() }()

	enc.fields = append(enc.fields, "msg", e.Message)

	for _, f := range ff {
		f.AddTo(enc)
	}

	switch e.Level {
	case zapcore.DebugLevel:
		return level.Debug(lc.inner).Log(enc.fields...)
	case zapcore.InfoLevel:
		return level.Info(lc.inner).Log(enc.fields...)
	case zapcore.WarnLevel:
		return level.Warn(lc.inner).Log(enc.fields...)
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		// We ignore panics/fatals here because we really don't want components to
		// be able to do that.
		return level.Error(lc.inner).Log(enc.fields...)
	default:
		return lc.inner.Log(enc.fields...)
	}
}

func (lc *loggerCore) Sync() error {
	return nil
}

// fieldEncoder implements zapcore.ObjectEncoder. It enables converting a
// zapcore.Field into a value which will be written as a github.com/go-kit/log
// keypair.
type fieldEncoder struct {
	// fields are the list of fields that will be passed to log.Logger.Log.
	fields []any

	// namespace is used to prefix keys before appending to fields. When a
	// zap.Namespace field is logged, the OpenNamespace method of the
	// fieldEncoder will be invoked, appending to the namespace slice.
	//
	// It is not possible to pop a namespace from the list; once a zap.Namespace
	// field is logged, all further fields in that entry are scoped within that
	// namespace.
	namespace []string
}

var _ zapcore.ObjectEncoder = (*fieldEncoder)(nil)

var encPool = sync.Pool{
	New: func() any {
		return &fieldEncoder{}
	},
}

// newFieldEncoder creates a ready-to-use fieldEncoder. Call Close once the
// fieldEncoder is no longer needed.
func newFieldEncoder() *fieldEncoder {
	fe := encPool.Get().(*fieldEncoder)
	fe.fields = fe.fields[:0]
	fe.namespace = fe.namespace[:0]
	return fe
}

func (fe *fieldEncoder) Close() error {
	encPool.Put(fe)
	return nil
}

func (fe *fieldEncoder) AddArray(key string, marshaler zapcore.ArrayMarshaler) error {
	fe.fields = append(fe.fields, fe.keyName(key), lazyStringer{f: func() string {
		enc := newArrayFieldEncoder()
		err := marshaler.MarshalLogArray(enc)
		if err != nil {
			return err.Error()
		}
		b, err := enc.jsonMarshal()
		if err != nil {
			return err.Error()
		}
		return string(b)
	}})
	return nil
}

func (fe *fieldEncoder) AddObject(key string, marshaler zapcore.ObjectMarshaler) error {
	fe.fields = append(fe.fields, fe.keyName(key), lazyStringer{f: func() string {
		enc := newObjectFieldEncoder()
		err := marshaler.MarshalLogObject(enc)
		if err != nil {
			return err.Error()
		}
		b, err := enc.jsonMarshal()
		if err != nil {
			return err.Error()
		}
		return string(b)
	}})
	return nil
}

func (fe *fieldEncoder) AddBinary(key string, value []byte) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddByteString(key string, value []byte) {
	fe.fields = append(fe.fields, fe.keyName(key), string(value))
}

func (fe *fieldEncoder) AddBool(key string, value bool) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddComplex128(key string, value complex128) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddComplex64(key string, value complex64) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddDuration(key string, value time.Duration) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddFloat64(key string, value float64) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddFloat32(key string, value float32) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddInt(key string, value int) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddInt64(key string, value int64) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddInt32(key string, value int32) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddInt16(key string, value int16) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddInt8(key string, value int8) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddString(key, value string) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddTime(key string, value time.Time) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddUint(key string, value uint) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddUint64(key string, value uint64) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddUint32(key string, value uint32) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddUint16(key string, value uint16) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddUint8(key string, value uint8) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddUintptr(key string, value uintptr) {
	fe.fields = append(fe.fields, fe.keyName(key), value)
}

func (fe *fieldEncoder) AddReflected(key string, value any) error {
	fe.fields = append(fe.fields, fe.keyName(key), value)
	return nil
}

func (fe *fieldEncoder) OpenNamespace(key string) {
	fe.namespace = append(fe.namespace, key)
}

// keyName returns the key to used for a named field. If the fieldEncoder isn't
// namespaced, then the key name is k. Otherwise, the key name the combined
// string of the namespace and key, delimiting each fragment by a period `.`.
func (fe *fieldEncoder) keyName(k string) any {
	if len(fe.namespace) == 0 {
		return k
	}
	return key(append(fe.namespace, k))
}

type key []string

var _ fmt.Stringer = (key)(nil)

func (k key) String() string {
	if len(k) == 1 {
		return k[0]
	}
	return strings.Join(k, ".")
}

var _ zapcore.ObjectEncoder = (*objectFieldEncoder)(nil)

type objectFieldEncoder struct {
	obj       map[string]any
	namespace []string
}

func newObjectFieldEncoder() *objectFieldEncoder {
	return &objectFieldEncoder{
		obj: make(map[string]any),
	}
}

func (fe *objectFieldEncoder) key(key string) string {
	if len(fe.namespace) == 0 {
		return key
	}
	return strings.Join(append(fe.namespace, key), ".")
}

func (fe *objectFieldEncoder) jsonMarshal() ([]byte, error) {
	return json.Marshal(fe.obj)
}

func (fe *objectFieldEncoder) AddArray(key string, marshaler zapcore.ArrayMarshaler) error {
	subFieldEncoder := newArrayFieldEncoder()
	err := marshaler.MarshalLogArray(subFieldEncoder)
	if err != nil {
		return err
	}
	fe.obj[key] = subFieldEncoder.arr
	return nil
}

func (fe *objectFieldEncoder) AddObject(key string, marshaler zapcore.ObjectMarshaler) error {
	subFieldEncoder := newObjectFieldEncoder()
	err := marshaler.MarshalLogObject(subFieldEncoder)
	if err != nil {
		return err
	}
	fe.obj[key] = subFieldEncoder.obj
	return nil
}

func (fe *objectFieldEncoder) AddBinary(key string, value []byte) {
	fe.obj[fe.key(key)] = string(value)
}
func (fe *objectFieldEncoder) AddByteString(key string, value []byte) {
	fe.obj[fe.key(key)] = string(value)
}
func (fe *objectFieldEncoder) AddBool(key string, value bool) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddComplex128(key string, value complex128) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddComplex64(key string, value complex64) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddDuration(key string, value time.Duration) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddFloat64(key string, value float64) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddFloat32(key string, value float32) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddInt(key string, value int) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddInt64(key string, value int64) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddInt32(key string, value int32) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddInt16(key string, value int16) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddInt8(key string, value int8) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddString(key, value string) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddTime(key string, value time.Time) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddUint(key string, value uint) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddUint64(key string, value uint64) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddUint32(key string, value uint32) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddUint16(key string, value uint16) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddUint8(key string, value uint8) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddUintptr(key string, value uintptr) {
	fe.obj[fe.key(key)] = value
}
func (fe *objectFieldEncoder) AddReflected(key string, value any) error {
	fe.obj[fe.key(key)] = value
	return nil
}
func (fe *objectFieldEncoder) OpenNamespace(key string) {
	fe.namespace = append(fe.namespace, key)
}

var _ zapcore.ArrayEncoder = (*arrayFieldEncoder)(nil)

type arrayFieldEncoder struct {
	arr []any
}

func newArrayFieldEncoder() *arrayFieldEncoder {
	return &arrayFieldEncoder{
		arr: make([]any, 0),
	}
}

func (fe *arrayFieldEncoder) jsonMarshal() ([]byte, error) {
	return json.Marshal(fe.arr)
}

func (fe *arrayFieldEncoder) AppendArray(marshaler zapcore.ArrayMarshaler) error {
	subFieldEncoder := newArrayFieldEncoder()
	err := marshaler.MarshalLogArray(subFieldEncoder)
	if err != nil {
		return err
	}
	fe.arr = append(fe.arr, subFieldEncoder.arr)
	return nil
}

func (fe *arrayFieldEncoder) AppendObject(marshaler zapcore.ObjectMarshaler) error {
	subFieldEncoder := newObjectFieldEncoder()
	err := marshaler.MarshalLogObject(subFieldEncoder)
	if err != nil {
		return err
	}
	fe.arr = append(fe.arr, subFieldEncoder.obj)
	return nil
}

func (fe *arrayFieldEncoder) AppendBool(value bool) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendByteString(value []byte) {
	fe.arr = append(fe.arr, string(value))
}
func (fe *arrayFieldEncoder) AppendComplex128(value complex128) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendComplex64(value complex64) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendDuration(value time.Duration) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendFloat64(value float64) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendFloat32(value float32) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendInt(value int) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendInt64(value int64) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendInt32(value int32) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendInt16(value int16) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendInt8(value int8) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendString(value string) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendTime(value time.Time) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendUint(value uint) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendUint64(value uint64) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendUint32(value uint32) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendUint16(value uint16) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendUint8(value uint8) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendUintptr(value uintptr) {
	fe.arr = append(fe.arr, value)
}
func (fe *arrayFieldEncoder) AppendReflected(value any) error {
	fe.arr = append(fe.arr, value)
	return nil
}

type lazyStringer struct {
	f func() string
}

func (l lazyStringer) String() string {
	return l.f()
}
