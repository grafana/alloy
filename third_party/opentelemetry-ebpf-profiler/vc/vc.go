// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package vc provides buildtime information.
package vc // import "go.opentelemetry.io/ebpf-profiler/vc"

var (
	// The following variables are going to be set at link time using ldflags
	// and can be referenced later in the program.

	// revision of the service
	revision = ""
	// buildTimestamp, timestamp of the build
	buildTimestamp = ""
	// version in vX.Y.Z{-N-abbrev} format (via git-describe --tags)
	version = ""
)

// Revision of the service.
func Revision() string {
	return revision
}

// BuildTimestamp returns the timestamp of the build.
func BuildTimestamp() string {
	return buildTimestamp
}

// Version in vX.Y.Z{-N-abbrev} format.
func Version() string {
	return version
}
