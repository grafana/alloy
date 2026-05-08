// Copyright Grafana Labs and OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opamp

import "context"

var ValidateMergedYAML func(ctx context.Context, yaml []byte) error
