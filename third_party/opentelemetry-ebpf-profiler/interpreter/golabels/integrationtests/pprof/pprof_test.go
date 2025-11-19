// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprof // import "go.opentelemetry.io/ebpf-profiler/interpreter/golabels/integrationtests/pprof"

import (
	"context"
	"math/rand"
	"runtime/debug"
	"runtime/pprof"
	"testing"
	"time"
)

//nolint:gosec
func randomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func setPprofLabels(t *testing.T, ctx context.Context, cookie string, busyFunc func()) {
	t.Helper()
	labels := pprof.Labels(
		"l1"+cookie, "label1"+randomString(16),
		"l2"+cookie, "label2"+randomString(24),
		"l3"+cookie, "label3"+randomString(48))
	lastUpdate := time.Now()
	pprof.Do(ctx, labels, func(context.Context) {
		for time.Since(lastUpdate) < 10*time.Second {
			// CPU go burr on purpose.
			busyFunc()
			if ctx.Err() != nil {
				return
			}
		}
	})
}

// TestPprof ...
func TestPprof(t *testing.T) {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatalf("Failed to get build info")
	}

	withCGO := false
	for _, setting := range buildInfo.Settings {
		if setting.Key == "CGO_ENABLED" {
			withCGO = true
		}
	}
	t.Logf("%s - CGo is enabled: %t", buildInfo.GoVersion, withCGO)

	cookie := buildInfo.GoVersion

	setPprofLabels(t, context.TODO(), cookie, busyFunc)
}
