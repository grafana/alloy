package pfunsafe // import "go.opentelemetry.io/ebpf-profiler/libpf/pfunsafe"

import "unsafe"

// FromPointer converts a Go struct pointer to []byte to read data into
// data must be a non-nil pointer to a struct
func FromPointer[T any](data *T) []byte {
	return unsafe.Slice(
		(*byte)(unsafe.Pointer(data)),
		int(unsafe.Sizeof(*data)),
	)
}

// FromSlice converts a Go slice to []byte to read data into
func FromSlice[T any](data []T) []byte {
	if len(data) == 0 {
		return nil
	}
	return unsafe.Slice(
		(*byte)(unsafe.Pointer(&data[0])),
		len(data)*int(unsafe.Sizeof(data[0])),
	)
}

// ToString converts a byte slice into a string without a heap allocation.
// Be aware that the byte slice and the string share the same memory - which makes
// the string mutable.
func ToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
