package value

// RawFunction allows creating function implementations using raw River values.
// This is useful for functions which wish to operate over dynamic types while
// avoiding decoding to interface{} for performance reasons.
//
// The func value itself is provided as an argument so error types can be
// filled.
type RawFunction func(funcValue Value, args ...Value) (Value, error)
