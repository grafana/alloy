//go:build integration && linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"context"
	"testing"

	"github.com/cilium/ebpf"
	"golang.org/x/sync/errgroup"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/rlimit"

	"github.com/stretchr/testify/require"
)

func prepareMapInMap(t *testing.T) *ebpf.Map {
	t.Helper()

	restoreRlimit, err := rlimit.MaximizeMemlock()
	require.NoError(t, err)
	defer restoreRlimit()

	outerMapSpec := ebpf.MapSpec{
		Name:       "outer_map",
		Type:       ebpf.HashOfMaps,
		KeySize:    8,
		ValueSize:  4,
		MaxEntries: 3,
		InnerMap: &ebpf.MapSpec{
			Name:       "inner_map",
			Type:       ebpf.Hash,
			KeySize:    8,
			ValueSize:  4,
			MaxEntries: 3,
		},
	}

	outerMap, err := ebpf.NewMap(&outerMapSpec)
	require.NoError(t, err)
	return outerMap
}

func TestAsyncMapUpdaterPool(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	amup := newAsyncMapUpdaterPool(ctx, updatePoolWorkers, updatePoolQueueCap)

	outer := prepareMapInMap(t)
	defer outer.Close()

	defer func() {
		// If EnqueueUpdate() panics, recover() will return a non nil value.
		r := recover()
		require.Nil(t, r)
	}()

	g, _ := errgroup.WithContext(ctx)
	// For every worker start a Go routine that tries to send updates.
	for i := 0; i < updatePoolWorkers; i++ {
		g.Go(func() error {
			for j := 0; j < updatePoolQueueCap*42; j++ {
				// After some time, cancel the context to stop asyncUpdateWorker.
				if j%updatePoolQueueCap == 0 {
					cancel()
				}

				fileID := host.FileID(j)
				// Simulate a delete attempt for fileID after the worker context expired.
				amup.EnqueueUpdate(outer, fileID, nil)
			}
			return nil
		})
	}

	err := g.Wait()
	require.NoError(t, err)
}
