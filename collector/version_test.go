package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_VersionFromFile(t *testing.T) {
	in := "1.2.3 #x-release-please-version \n"
	expect := "v1.2.3"

	actual := CollectorVersionFromFile(in)
	require.Equal(t, expect, actual)
}
