package discovery

import (
	"fmt"
	"testing"

	commonlabels "github.com/prometheus/common/model"
	modellabels "github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/internal/runtime/equality"
)

func TestTargetBuilder(t *testing.T) {
	testCases := []struct {
		name     string
		init     map[string]string
		op       func(tb TargetBuilder)
		asserts  func(t *testing.T, tb TargetBuilder)
		expected map[string]string
	}{
		{
			name:     "no changes",
			init:     map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			expected: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
		},
		{
			name:     "no changes with empty value deleted",
			init:     map[string]string{"hip": "hop", "boom": "bap", "tiki": ""},
			expected: map[string]string{"hip": "hop", "boom": "bap"},
		},
		{
			name:     "no changes with all values deleted",
			init:     map[string]string{"hip": "", "boom": "", "tiki": ""},
			expected: map[string]string{},
		},
		{
			name:     "no changes from nil",
			init:     nil,
			expected: nil,
		},
		{
			name: "get",
			init: map[string]string{"hip": "hop", "boom": "bap"},
			asserts: func(t *testing.T, tb TargetBuilder) {
				assert.Equal(t, "bap", tb.Get("boom"))
			},
			expected: map[string]string{"hip": "hop", "boom": "bap"},
		},
		{
			name:     "set and get",
			init:     map[string]string{},
			op:       func(tb TargetBuilder) { tb.Set("ka", "boom") },
			asserts:  func(t *testing.T, tb TargetBuilder) { assert.Equal(t, "boom", tb.Get("ka")) },
			expected: map[string]string{"ka": "boom"},
		},
		{
			name:     "set and get from nil",
			init:     nil,
			op:       func(tb TargetBuilder) { tb.Set("ka", "boom") },
			asserts:  func(t *testing.T, tb TargetBuilder) { assert.Equal(t, "boom", tb.Get("ka")) },
			expected: map[string]string{"ka": "boom"},
		},
		{
			name:     "add one",
			init:     map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			op:       func(tb TargetBuilder) { tb.Set("ka", "boom") },
			expected: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta", "ka": "boom"},
		},
		{
			name: "add two",
			init: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			op: func(tb TargetBuilder) {
				tb.Set("ka", "boom")
				tb.Set("foo", "bar")
			},
			expected: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta", "ka": "boom", "foo": "bar"},
		},
		{
			name:     "overwrite one",
			init:     map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			op:       func(tb TargetBuilder) { tb.Set("tiki", "tiki") },
			expected: map[string]string{"hip": "hop", "boom": "bap", "tiki": "tiki"},
		},
		{
			name: "merge with target",
			init: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			op: func(tb TargetBuilder) {
				tb.MergeWith(NewTargetFromMap(map[string]string{"ka": "boom", "tiki": "tiki", "kung": "fu"}))
			},
			expected: map[string]string{"hip": "hop", "boom": "bap", "tiki": "tiki", "ka": "boom", "kung": "fu"},
		},
		{
			name:     "delete one",
			init:     map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			op:       func(tb TargetBuilder) { tb.Del("tiki") },
			expected: map[string]string{"hip": "hop", "boom": "bap"},
		},
		{
			name:     "delete one by setting to empty",
			init:     map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			op:       func(tb TargetBuilder) { tb.Set("tiki", "") },
			expected: map[string]string{"hip": "hop", "boom": "bap"},
		},
		{
			name:     "delete multiple",
			init:     map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			op:       func(tb TargetBuilder) { tb.Del("tiki", "hip") },
			expected: map[string]string{"boom": "bap"},
		},
		{
			name: "add and delete one",
			init: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			op: func(tb TargetBuilder) {
				tb.Set("tiki", "tiki")
				tb.Del("tiki")
			},
			expected: map[string]string{"hip": "hop", "boom": "bap"},
		},
		{
			name: "add and delete one by setting to empty",
			init: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			op: func(tb TargetBuilder) {
				tb.Set("tiki", "tiki")
				tb.Set("tiki", "")
			},
			expected: map[string]string{"hip": "hop", "boom": "bap"},
		},
		{
			name: "delete and add one",
			init: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			op: func(tb TargetBuilder) {
				tb.Del("tiki")
				tb.Set("tiki", "tiki")
			},
			expected: map[string]string{"hip": "hop", "boom": "bap", "tiki": "tiki"},
		},
		{
			name: "delete by setting to empty and add one",
			init: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			op: func(tb TargetBuilder) {
				tb.Set("tiki", "")
				tb.Set("tiki", "tiki")
			},
			expected: map[string]string{"hip": "hop", "boom": "bap", "tiki": "tiki"},
		},
		{
			name: "get after adding",
			init: map[string]string{"hip": "hop", "boom": "bap"},
			op: func(tb TargetBuilder) {
				tb.Set("tiki", "taki")
			},
			asserts: func(t *testing.T, tb TargetBuilder) {
				assert.Equal(t, "taki", tb.Get("tiki"))
			},
			expected: map[string]string{"hip": "hop", "boom": "bap", "tiki": "taki"},
		},
		{
			name: "get after deleting",
			init: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			op: func(tb TargetBuilder) {
				tb.Del("tiki")
			},
			asserts: func(t *testing.T, tb TargetBuilder) {
				assert.Equal(t, "", tb.Get("tiki"))
			},
			expected: map[string]string{"hip": "hop", "boom": "bap"},
		},
		{
			name: "simple range",
			init: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			asserts: func(t *testing.T, tb TargetBuilder) {
				seen := map[string]struct{}{}
				tb.Range(func(label string, value string) {
					seen[fmt.Sprintf("%q: %q", label, value)] = struct{}{}
				})
				assert.Equal(
					t,
					map[string]struct{}{`"hip": "hop"`: {}, `"boom": "bap"`: {}, `"tiki": "ta"`: {}},
					seen,
				)
			},
			expected: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
		},
		{
			name: "range with pending additions and deletes",
			init: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			asserts: func(t *testing.T, tb TargetBuilder) {
				tb.Set("tiki", "taki")
				tb.Del("boom")
				tb.Set("kung", "fu")

				seen := map[string]struct{}{}
				tb.Range(func(label string, value string) {
					seen[fmt.Sprintf("%q: %q", label, value)] = struct{}{}
				})
				assert.Equal(
					t,
					map[string]struct{}{`"hip": "hop"`: {}, `"kung": "fu"`: {}, `"tiki": "taki"`: {}},
					seen,
				)
			},
			expected: map[string]string{"hip": "hop", "kung": "fu", "tiki": "taki"},
		},
		{
			name: "simple range on nil",
			init: nil,
			asserts: func(t *testing.T, tb TargetBuilder) {
				seen := map[string]struct{}{}
				tb.Range(func(label string, value string) {
					seen[fmt.Sprintf("%q: %q", label, value)] = struct{}{}
				})
				assert.Equal(t, map[string]struct{}{}, seen)
			},
			expected: nil,
		},
		{
			name: "range with adding",
			init: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			asserts: func(t *testing.T, tb TargetBuilder) {
				tb.Range(func(label string, value string) {
					tb.Set(value, label)
				})
			},
			expected: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta", "hop": "hip", "bap": "boom", "ta": "tiki"},
		},
		{
			name: "range with overwriting",
			init: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			asserts: func(t *testing.T, tb TargetBuilder) {
				tb.Range(func(label string, value string) {
					tb.Set(label, label)
				})
			},
			expected: map[string]string{"hip": "hip", "boom": "boom", "tiki": "tiki"},
		},
		{
			name: "range with deleting",
			init: map[string]string{"hip": "hop", "boom": "bap", "tiki": "ta"},
			asserts: func(t *testing.T, tb TargetBuilder) {
				tb.Range(func(label string, value string) {
					if len(label) > 3 {
						tb.Del(label)
					}
				})
			},
			expected: map[string]string{"hip": "hop"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expected := NewTargetFromMap(tc.expected)

			runTest := func(t *testing.T, tb TargetBuilder) {
				if tc.op != nil {
					tc.op(tb)
				}
				if tc.asserts != nil {
					tc.asserts(t, tb)
				}
				actual := tb.Target()

				equal := equality.DeepEqual(actual, expected)
				assert.True(t, equal)
				if !equal { // if not equal, run this to get a nice diff view
					assert.Equal(t, expected, actual)
				}

				assert.Equal(t, modellabels.StableHash(actual.PromLabels()), actual.HashLabelsWithPredicate(func(key string) bool {
					return true
				}), "prometheus and alloy target hash codes should match")
			}

			t.Run("prometheus compliant", func(t *testing.T) {
				tb := newPromBuilderAdapter(modellabels.FromMap(tc.init))
				runTest(t, tb)
			})

			t.Run("all labels own", func(t *testing.T) {
				tb := NewTargetBuilderFromLabelSets(nil, labelSetFromMap(tc.init))
				runTest(t, tb)
			})

			t.Run("all labels from group", func(t *testing.T) {
				tb := NewTargetBuilderFromLabelSets(labelSetFromMap(tc.init), nil)
				runTest(t, tb)
			})

			t.Run("group and own split by half", func(t *testing.T) {
				first, second := splitMap(len(tc.init)/2, tc.init)
				tb := NewTargetBuilderFromLabelSets(labelSetFromMap(first), labelSetFromMap(second))
				runTest(t, tb)
			})

			t.Run("group labels overwritten", func(t *testing.T) {
				first, second := splitMap(len(tc.init)/2, tc.init)
				for k, v := range second {
					first[k] = fmt.Sprintf("overwritten_%s", v)
				}
				tb := NewTargetBuilderFromLabelSets(labelSetFromMap(first), labelSetFromMap(second))
				runTest(t, tb)
			})
		})
	}
}

func splitMap(firstSize int, m map[string]string) (map[string]string, map[string]string) {
	if len(m) <= firstSize {
		return m, nil
	}
	first, second := make(map[string]string), make(map[string]string)
	ind := 0
	for k, v := range m {
		if ind < firstSize {
			first[k] = v
		} else {
			second[k] = v
		}
		ind++
	}
	return first, second
}

func labelSetFromMap(m map[string]string) commonlabels.LabelSet {
	r := make(commonlabels.LabelSet, len(m))
	for k, v := range m {
		r[commonlabels.LabelName(k)] = commonlabels.LabelValue(v)
	}
	return r
}

// builderAdapter is used to verify TargetBuilder implementation in this package matches the prometheus model.Builder.
type builderAdapter struct {
	b *modellabels.Builder
}

func (b *builderAdapter) SetKV(kv ...string) TargetBuilder {
	for i := 0; i < len(kv); i += 2 {
		b.b.Set(kv[i], kv[i+1])
	}
	return b
}

func (b *builderAdapter) MergeWith(target Target) TargetBuilder {
	target.ForEachLabel(func(key string, value string) bool {
		b.b.Set(key, value)
		return true
	})
	return b
}

func newPromBuilderAdapter(ls modellabels.Labels) TargetBuilder {
	return &builderAdapter{
		b: modellabels.NewBuilder(ls),
	}
}

func (b *builderAdapter) Get(label string) string {
	return b.b.Get(label)
}

func (b *builderAdapter) Range(f func(label string, value string)) {
	b.b.Range(func(l modellabels.Label) {
		f(l.Name, l.Value)
	})
}

func (b *builderAdapter) Set(label string, val string) {
	b.b.Set(label, val)
}

func (b *builderAdapter) Del(ns ...string) {
	b.b = b.b.Del(ns...)
}

func (b *builderAdapter) Target() Target {
	return NewTargetFromModelLabels(b.b.Labels())
}
