package shapecheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type upInner struct {
	A int `mapstructure:"a"`
}

type up struct {
	Name  string  `mapstructure:"name"`
	Inner upInner `mapstructure:"inner"`
	New   string  `mapstructure:"new_field"`
}

type alInner struct {
	A int `alloy:"a,attr,optional"`
}

type al struct {
	Name  string  `alloy:"name,attr"`
	Inner alInner `alloy:"inner,block,optional"`
	Extra bool    `alloy:"extra,attr,optional"`
}

func TestDiff(t *testing.T) {
	problems := Diff(up{}, al{}, Allow{})
	require.Len(t, problems, 2)
	require.Contains(t, problems[0], `"new_field"`)
	require.Contains(t, problems[1], `"extra"`)

	allow := Allow{
		Skip:      map[string]string{"new_field": "not exposed"},
		AlloyOnly: map[string]string{"extra": "alloy only"},
	}
	require.Empty(t, Diff(up{}, al{}, allow))

	// An allowlisted field that vanishes is reported.
	type upGone struct {
		Name string `mapstructure:"name"`
	}
	problems = Diff(upGone{}, al{}, Allow{Skip: allow.Skip, AlloyOnly: allow.AlloyOnly})
	require.Contains(t, problems, `stale Skip: upstream no longer has "new_field" (not exposed)`)

	// An upstream rename shows as a missing field plus a stale allow entry.
	type upRenamed struct {
		Title string `mapstructure:"title"`
	}
	problems = Diff(upRenamed{}, al{Name: ""}, Allow{Rename: map[string]string{"name": "name"}, AlloyOnly: allow.AlloyOnly})
	require.Len(t, problems, 4, problems)
	require.Contains(t, problems[0], "stale Rename")
}

type upItem struct {
	Key string `mapstructure:"key"`
}

type upList struct {
	Items []upItem `mapstructure:"items"`
}

type alItem struct {
	Key string `alloy:"key,attr"`
}

type alList struct {
	Items []alItem `alloy:"item,block,optional"`
}

func TestDiffRepeatedBlockRename(t *testing.T) {
	// Without the rename, every path under the plural list is unmatched.
	require.Len(t, Diff(upList{}, alList{}, Allow{}), 4)

	// The rename covers the list's children too.
	require.Empty(t, Diff(upList{}, alList{}, Allow{Rename: map[string]string{"items": "item"}}))
}

func TestDiffFlattenedWrapper(t *testing.T) {
	type upWrapper struct {
		Protocols struct {
			Inner upInner `mapstructure:"inner"`
		} `mapstructure:"protocols"`
	}
	type alFlat struct {
		Inner alInner `alloy:"inner,block,optional"`
	}
	require.Empty(t, Diff(upWrapper{}, alFlat{}, Allow{Rename: map[string]string{"protocols.inner": "inner"}}))
}
