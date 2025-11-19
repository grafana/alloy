// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package readatbuf_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/libpf/readatbuf"
	"go.opentelemetry.io/ebpf-profiler/testsupport"
)

func testVariant(t *testing.T, fileSize, granularity, cacheSize uint) {
	file := testsupport.GenerateTestInputFile(255, fileSize)
	rawReader := bytes.NewReader(file)
	cachingReader, err := readatbuf.New(rawReader, granularity, cacheSize)
	require.NoError(t, err)
	testsupport.ValidateReadAtWrapperTransparency(t, 10000, file, cachingReader)
}

func TestCaching(t *testing.T) {
	testVariant(t, 1024, 64, 1)
	testVariant(t, 1346, 11, 55)
	testVariant(t, 889, 34, 111)
}

func TestOutOfBoundsTail(t *testing.T) {
	buf := bytes.NewReader([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	r, err := readatbuf.New(buf, 5, 10)
	require.NoError(t, err)
	b := make([]byte, 1)
	for i := range int64(32) {
		_, err = r.ReadAt(b, i)
		if i > 7 {
			require.ErrorIs(t, err, io.EOF)
		} else {
			require.NoError(t, err)
		}
	}
}
