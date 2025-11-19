// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package php

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPHPRegexs(t *testing.T) {
	shouldMatch := []string{"php", "./php", "/foo/bar/php", "./foo/bar/php", "php-fpm", "php-cgi7",
		"/usr/lib/apache2/modules/libphp.so", "/libphp.so",
		"/usr/lib/apache2/modules/libphp5.so", "/libphp5.so",
		"/usr/lib/apache2/modules/libphp8.1.so", "/libphp8.1.so"}
	for _, s := range shouldMatch {
		assert.True(t, phpRegex.MatchString(s), "PHP regex %s should match %s",
			phpRegex.String(), s)
	}

	shouldNotMatch := []string{"foophp", "ph p", "ph/p", "php-bar",
		"/usr/lib/apache2/modules/libphp8.1-so", "libphp-so", "/libphp.soap"}
	for _, s := range shouldNotMatch {
		assert.False(t, phpRegex.MatchString(s), "PHP regex %s should not match %s",
			phpRegex.String(), s)
	}
}

func TestVersionExtract(t *testing.T) {
	tests := map[string]struct {
		given       string
		expected    uint
		expectError bool
	}{
		"7.x":          {given: "7.4.19", expected: phpVersion(7, 4, 19), expectError: false},
		"8.x":          {given: "8.2.7", expected: phpVersion(8, 2, 7), expectError: false},
		"double-digit": {given: "8.0.27", expected: phpVersion(8, 0, 27), expectError: false},
		"suffix": {
			given:       "8.1.2-1ubuntu2.14",
			expected:    phpVersion(8, 1, 2),
			expectError: false,
		},
		"no-release":   {given: "7.4", expected: 0, expectError: true},
		"trailing-dot": {given: "8.0.", expected: 0, expectError: true},
		"only-major":   {given: "8", expected: 0, expectError: true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			v, err := versionExtract(test.given)
			assert.Equal(t, test.expected, v)
			if test.expectError {
				assert.Error(t, err, "Expected error, received no error")
			} else {
				assert.NoError(t, err, "Expected no error, received error: %v", err)
			}
		})
	}
}
