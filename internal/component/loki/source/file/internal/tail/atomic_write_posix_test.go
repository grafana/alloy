//go:build linux || darwin || freebsd || netbsd || openbsd

package tail

import (
	"strings"
	"testing"

	"github.com/natefinch/atomic"
	"github.com/stretchr/testify/require"
)

func atomicWrite(t *testing.T, name, newContent string) {
	require.NoError(t, atomic.WriteFile(name, strings.NewReader(newContent)))
}
