package testlivedebugging_test

import (
	"testing"

	"github.com/grafana/alloy/internal/util/testlivedebugging"
	"github.com/stretchr/testify/require"
)

func TestLogShallowCopy(t *testing.T) {
	log := testlivedebugging.NewLog()
	log.Append("test")

	logSlice1 := log.Get()
	logSlice1[0] = "asdf"

	logSlice2 := log.Get()
	require.Equal(t, []string{"test"}, logSlice2)
}
