package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUnmarshalYAMLMerged_CustomUnmarshal checks to see that
// UnmarshalYAMLMerged works with merging types that have custom unmarshal
// methods which do extra checks after calling unmarshal.
func TestUnmarshalYAMLMerged_CustomUnmarshal(t *testing.T) {
	in := `
  fieldA: foo
  fieldB: bar
  `

	var (
		val1 typeOne
		val2 typeTwo
	)

	err := UnmarshalYAMLMerged([]byte(in), &val1, &val2)
	require.NoError(t, err)

	require.Equal(t, "foo", val1.FieldA)
	require.Equal(t, "bar", val2.FieldB)
	require.True(t, val2.Unmarshaled)
}

type typeOne struct {
	FieldA string `yaml:"fieldA"`
}

type typeTwo struct {
	FieldB      string `yaml:"fieldB"`
	Unmarshaled bool   `yaml:"-"`
}

func (t *typeTwo) UnmarshalYAML(unmarshal func(any) error) error {
	type rawType typeTwo
	if err := unmarshal((*rawType)(t)); err != nil {
		return err
	}
	t.Unmarshaled = true
	return nil
}
