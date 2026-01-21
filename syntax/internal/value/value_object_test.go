package value_test

import (
	"testing"

	"github.com/grafana/alloy/syntax/internal/value"
	"github.com/stretchr/testify/require"
)

// TestBlockRepresentation ensures that the struct tags for blocks are
// represented correctly.
func TestBlockRepresentation(t *testing.T) {
	type UnlabledBlock struct {
		Value int `alloy:"value,attr"`
	}
	type LabeledBlock struct {
		Value int    `alloy:"value,attr"`
		Label string `alloy:",label"`
	}
	type OuterBlock struct {
		Attr1 string `alloy:"attr_1,attr"`
		Attr2 string `alloy:"attr_2,attr"`

		UnlabledBlock1 UnlabledBlock `alloy:"unlabeled.a,block"`
		UnlabledBlock2 UnlabledBlock `alloy:"unlabeled.b,block"`
		UnlabledBlock3 UnlabledBlock `alloy:"other_unlabeled,block"`

		LabeledBlock1 LabeledBlock `alloy:"labeled.a,block"`
		LabeledBlock2 LabeledBlock `alloy:"labeled.b,block"`
		LabeledBlock3 LabeledBlock `alloy:"other_labeled,block"`
	}

	val := OuterBlock{
		Attr1: "value_1",
		Attr2: "value_2",
		UnlabledBlock1: UnlabledBlock{
			Value: 1,
		},
		UnlabledBlock2: UnlabledBlock{
			Value: 2,
		},
		UnlabledBlock3: UnlabledBlock{
			Value: 3,
		},
		LabeledBlock1: LabeledBlock{
			Value: 4,
			Label: "label_a",
		},
		LabeledBlock2: LabeledBlock{
			Value: 5,
			Label: "label_b",
		},
		LabeledBlock3: LabeledBlock{
			Value: 6,
			Label: "label_c",
		},
	}

	t.Run("Map decode", func(t *testing.T) {
		var m map[string]any
		require.NoError(t, value.Decode(value.Encode(val), &m))

		type object = map[string]any

		expect := object{
			"attr_1": "value_1",
			"attr_2": "value_2",
			"unlabeled": object{
				"a": object{"value": 1},
				"b": object{"value": 2},
			},
			"other_unlabeled": object{"value": 3},
			"labeled": object{
				"a": object{
					"label_a": object{"value": 4},
				},
				"b": object{
					"label_b": object{"value": 5},
				},
			},
			"other_labeled": object{
				"label_c": object{"value": 6},
			},
		}

		require.Equal(t, m, expect)
	})

	t.Run("Object decode from other object", func(t *testing.T) {
		// Decode into a separate type which is structurally identical but not
		// literally the same.
		type OuterBlock2 OuterBlock

		var actualVal OuterBlock2
		require.NoError(t, value.Decode(value.Encode(val), &actualVal))
		require.Equal(t, val, OuterBlock(actualVal))
	})
}

// TestSquashedBlockRepresentation ensures that the struct tags for squashed
// blocks are represented correctly.
func TestSquashedBlockRepresentation(t *testing.T) {
	type InnerStruct struct {
		InnerField1 string `alloy:"inner_field_1,attr,optional"`
		InnerField2 string `alloy:"inner_field_2,attr,optional"`
	}

	type OuterStruct struct {
		OuterField1 string      `alloy:"outer_field_1,attr,optional"`
		Inner       InnerStruct `alloy:",squash"`
		OuterField2 string      `alloy:"outer_field_2,attr,optional"`
	}

	val := OuterStruct{
		OuterField1: "value1",
		Inner: InnerStruct{
			InnerField1: "value3",
			InnerField2: "value4",
		},
		OuterField2: "value2",
	}

	t.Run("Map decode", func(t *testing.T) {
		var m map[string]any
		require.NoError(t, value.Decode(value.Encode(val), &m))

		type object = map[string]any

		expect := object{
			"outer_field_1": "value1",
			"inner_field_1": "value3",
			"inner_field_2": "value4",
			"outer_field_2": "value2",
		}

		require.Equal(t, m, expect)
	})
}

func TestSliceOfBlocks(t *testing.T) {
	type UnlabledBlock struct {
		Value int `alloy:"value,attr"`
	}
	type LabeledBlock struct {
		Value int    `alloy:"value,attr"`
		Label string `alloy:",label"`
	}
	type OuterBlock struct {
		Attr1 string `alloy:"attr_1,attr"`
		Attr2 string `alloy:"attr_2,attr"`

		Unlabeled []UnlabledBlock `alloy:"unlabeled,block"`
		Labeled   []LabeledBlock  `alloy:"labeled,block"`
	}

	val := OuterBlock{
		Attr1: "value_1",
		Attr2: "value_2",
		Unlabeled: []UnlabledBlock{
			{Value: 1},
			{Value: 2},
			{Value: 3},
		},
		Labeled: []LabeledBlock{
			{Label: "label_a", Value: 4},
			{Label: "label_b", Value: 5},
			{Label: "label_c", Value: 6},
		},
	}

	t.Run("Map decode", func(t *testing.T) {
		var m map[string]any
		require.NoError(t, value.Decode(value.Encode(val), &m))

		type object = map[string]any
		type list = []any

		expect := object{
			"attr_1": "value_1",
			"attr_2": "value_2",
			"unlabeled": list{
				object{"value": 1},
				object{"value": 2},
				object{"value": 3},
			},
			"labeled": object{
				"label_a": object{"value": 4},
				"label_b": object{"value": 5},
				"label_c": object{"value": 6},
			},
		}

		require.Equal(t, m, expect)
	})

	t.Run("Object decode from other object", func(t *testing.T) {
		// Decode into a separate type which is structurally identical but not
		// literally the same.
		type OuterBlock2 OuterBlock

		var actualVal OuterBlock2
		require.NoError(t, value.Decode(value.Encode(val), &actualVal))
		require.Equal(t, val, OuterBlock(actualVal))
	})
}
