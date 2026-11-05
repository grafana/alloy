package rustcomponent

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"time"
)

// EncodeArgs lays out the fields of v tagged with `rust:"<type>"` the way C
// lays out a struct with those fields in the same order, so Rust can read it
// as its #[repr(C)] Raw struct. Untagged fields are skipped.
//
// Supported types are u8, u16, u32, u64, i8, i16, i32, i64 (any Go integer
// field), f32, f64 (float fields), bool and duration_ms (time.Duration as
// u64 milliseconds).
func EncodeArgs(v any) ([]byte, error) {
	rv := reflect.Indirect(reflect.ValueOf(v))
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected a struct, got %s", rv.Type())
	}

	var (
		buf      []byte
		maxAlign = 1
	)
	for i := range rv.NumField() {
		field := rv.Type().Field(i)
		typ, ok := field.Tag.Lookup("rust")
		if !ok {
			continue
		}

		b, err := encodeField(typ, rv.Field(i))
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}

		// Scalars are aligned to their size, as in C on 64-bit platforms.
		align := len(b)
		maxAlign = max(maxAlign, align)
		buf = pad(buf, align)
		buf = append(buf, b...)
	}
	return pad(buf, maxAlign), nil
}

func pad(buf []byte, align int) []byte {
	for len(buf)%align != 0 {
		buf = append(buf, 0)
	}
	return buf
}

func encodeField(typ string, v reflect.Value) ([]byte, error) {
	switch typ {
	case "u8":
		return encodeUint(v, 1)
	case "u16":
		return encodeUint(v, 2)
	case "u32":
		return encodeUint(v, 4)
	case "u64":
		return encodeUint(v, 8)
	case "i8":
		return encodeInt(v, 1)
	case "i16":
		return encodeInt(v, 2)
	case "i32":
		return encodeInt(v, 4)
	case "i64":
		return encodeInt(v, 8)
	case "f32":
		if !v.CanFloat() {
			return nil, fmt.Errorf("f32 needs a float field, got %s", v.Type())
		}
		return binary.NativeEndian.AppendUint32(nil, math.Float32bits(float32(v.Float()))), nil
	case "f64":
		if !v.CanFloat() {
			return nil, fmt.Errorf("f64 needs a float field, got %s", v.Type())
		}
		return binary.NativeEndian.AppendUint64(nil, math.Float64bits(v.Float())), nil
	case "bool":
		if v.Kind() != reflect.Bool {
			return nil, fmt.Errorf("bool needs a bool field, got %s", v.Type())
		}
		if v.Bool() {
			return []byte{1}, nil
		}
		return []byte{0}, nil
	case "duration_ms":
		d, ok := v.Interface().(time.Duration)
		if !ok || d < 0 {
			return nil, fmt.Errorf("duration_ms needs a non-negative time.Duration field, got %s", v.Type())
		}
		return binary.NativeEndian.AppendUint64(nil, uint64(d.Milliseconds())), nil
	default:
		return nil, fmt.Errorf("unsupported rust type %q", typ)
	}
}

func encodeUint(v reflect.Value, size int) ([]byte, error) {
	var n uint64
	switch {
	case v.CanUint():
		n = v.Uint()
	case v.CanInt() && v.Int() >= 0:
		n = uint64(v.Int())
	default:
		return nil, fmt.Errorf("u%d needs a non-negative integer field, got %v", size*8, v)
	}
	if size < 8 && n >= 1<<(size*8) {
		return nil, fmt.Errorf("%d overflows u%d", n, size*8)
	}
	return appendUint(n, size), nil
}

func encodeInt(v reflect.Value, size int) ([]byte, error) {
	var n int64
	switch {
	case v.CanInt():
		n = v.Int()
	case v.CanUint() && v.Uint() <= math.MaxInt64:
		n = int64(v.Uint())
	default:
		return nil, fmt.Errorf("i%d needs an integer field, got %v", size*8, v)
	}
	bits := size * 8
	if size < 8 && (n < -(1<<(bits-1)) || n >= 1<<(bits-1)) {
		return nil, fmt.Errorf("%d overflows i%d", n, bits)
	}
	return appendUint(uint64(n), size), nil
}

func appendUint(n uint64, size int) []byte {
	switch size {
	case 1:
		return []byte{byte(n)}
	case 2:
		return binary.NativeEndian.AppendUint16(nil, uint16(n))
	case 4:
		return binary.NativeEndian.AppendUint32(nil, uint32(n))
	default:
		return binary.NativeEndian.AppendUint64(nil, n)
	}
}
