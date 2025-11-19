// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/tools/coredump/cloudstore"
	"go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"
)

func TestCoreDumps(t *testing.T) {
	cases, err := findTestCases(true)
	require.NoError(t, err)
	require.NotEmpty(t, cases)

	cloudClient, err := cloudstore.Client()
	require.NoError(t, err)
	store, err := modulestore.New(cloudClient,
		cloudstore.PublicReadURL(), cloudstore.ModulestoreS3Bucket(), "modulecache")
	require.NoError(t, err)

	for _, filename := range cases {
		t.Run(filename, func(t *testing.T) {
			testCase, err := readTestCase(filename)
			require.NoError(t, err)
			if testCase.Skip != "" {
				t.Skip(testCase.Skip)
			}

			core, err := OpenStoreCoredump(store, testCase.CoredumpRef, testCase.Modules)
			require.NoError(t, err)
			defer core.Close()

			data, err := ExtractTraces(t.Context(), core, false, nil)

			require.NoError(t, err)
			require.Equal(t, testCase.Threads, data)
		})
	}
}
