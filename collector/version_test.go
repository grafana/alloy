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

func Test_CollectorVersion(t *testing.T) {
	file := "1.2.3 # x-release-please-version\n"

	tt := []struct {
		name         string
		buildVersion string
		expect       string
	}{
		{"release candidate", "v1.2.3-rc.0", "v1.2.3-rc.0"},
		{"stable release", "v1.2.3", "v1.2.3"},
		{"devel build", "v1.2.3-devel+abc1234", "v1.2.3-devel+abc1234"},
		{"no ldflags version", "v0.0.0", "v1.2.3"},
		{"empty build version", "", "v1.2.3"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expect, collectorVersion(tc.buildVersion, file))
		})
	}
}
