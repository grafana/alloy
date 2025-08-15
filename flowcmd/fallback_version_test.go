package flowcmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_fallbackVersionFromText(t *testing.T) {
	in := `# This is a comment 
# This is another comment 

v1.2.3

This line is ignored!`
	expect := "v1.2.3-devel"

	actual := fallbackVersionFromText([]byte(in))
	require.Equal(t, expect, actual)
}
