package util

import (
	"bytes"
	"io"

	"gopkg.in/yaml.v2"
)

// CompareYAML marshals a and b to YAML and ensures that their contents are
// equal. If either Marshal fails, CompareYAML returns false.
func CompareYAML(a, b interface{}) bool {
	aBytes, err := yaml.Marshal(a)
	if err != nil {
		return false
	}
	bBytes, err := yaml.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(aBytes, bBytes)
}

// CompareYAMLWithHook marshals both a and b to YAML and checks the results
// for equality, allowing for a hook to define custom marshaling behavior.
// If either Marshal fails, CompareYAMLWithHook returns false.
func CompareYAMLWithHook(a, b interface{}, hook func(in interface{}) (ok bool, out interface{}, err error)) bool {
	aBytes, err := marshalWithHook(a, hook)
	if err != nil {
		return false
	}
	bBytes, err := marshalWithHook(b, hook)
	if err != nil {
		return false
	}
	return bytes.Equal(aBytes, bBytes)
}

func marshalWithHook(i interface{}, hook func(in interface{}) (ok bool, out interface{}, err error)) ([]byte, error) {
	var buf bytes.Buffer
	err := marshalToWriterWithHook(i, &buf, hook)
	return buf.Bytes(), err
}

func marshalToWriterWithHook(i interface{}, w io.Writer, hook func(in interface{}) (ok bool, out interface{}, err error)) error {
	enc := yaml.NewEncoder(w)
	defer enc.Close()
	enc.SetHook(hook)
	return enc.Encode(i)
}
