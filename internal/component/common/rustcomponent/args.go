package rustcomponent

import "github.com/fxamacker/cbor/v2"

// EncodeArgs encodes the generated arguments of a Rust component as CBOR,
// keyed by their `cbor` tags. Rust decodes them by field name and rejects
// unknown fields.
func EncodeArgs(v any) ([]byte, error) {
	return cbor.Marshal(v)
}
