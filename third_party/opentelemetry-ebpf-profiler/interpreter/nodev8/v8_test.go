// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nodev8 // import "go.opentelemetry.io/ebpf-profiler/interpreter/nodev8"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegexs(t *testing.T) {
	shouldMatch := []string{
		"node",
		"node8",
		"./node",
		"/foo/bar/node",
		"./foo/bar/node",
		"nsolid",
		"nsolid8",
		"./nsolid",
		"/foo/bar/nsolid",
		"./foo/bar/nsolid",
		"./libnode.so",
		"/lib/libnode.so.12",
	}
	for _, s := range shouldMatch {
		assert.True(t, v8Regex.MatchString(s), "regex %s should match %s",
			v8Regex.String(), s)
	}

	shouldNotMatch := []string{
		"node-foo",
		"./nsolid-bar",
		"/lib/libnodetest.so",
		"/lib/libnode.so.1.2.3.4.5",
		"node-nsolid",
	}
	for _, s := range shouldNotMatch {
		assert.False(t, v8Regex.MatchString(s), "regex %s should not match %s",
			v8Regex.String(), s)
	}
}
