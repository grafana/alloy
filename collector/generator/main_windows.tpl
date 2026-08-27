//go:build windows

package main

import (
	"go.opentelemetry.io/collector/otelcol"
)

func run(params otelcol.CollectorSettings) error {
	return runInteractive(params)
}
