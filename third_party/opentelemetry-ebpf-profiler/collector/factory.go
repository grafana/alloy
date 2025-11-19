// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collector // import "go.opentelemetry.io/ebpf-profiler/collector"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"go.opentelemetry.io/ebpf-profiler/collector/config"
	"go.opentelemetry.io/ebpf-profiler/collector/internal"
	"go.opentelemetry.io/ebpf-profiler/internal/controller"
)

var (
	typeStr = component.MustNewType("profiling")

	errInvalidConfig = errors.New("invalid config")
)

// NewFactory creates a factory for the receiver.
func NewFactory() receiver.Factory {
	return xreceiver.NewFactory(
		typeStr,
		defaultConfig,
		xreceiver.WithProfiles(BuildProfilesReceiver(), component.StabilityLevelAlpha))
}

func BuildProfilesReceiver(options ...Option) xreceiver.CreateProfilesFunc {
	return func(ctx context.Context,
		rs receiver.Settings,
		baseCfg component.Config,
		nextConsumer xconsumer.Profiles,
	) (xreceiver.Profiles, error) {
		cfg, ok := baseCfg.(*config.Config)
		if !ok {
			return nil, errInvalidConfig
		}

		controllerOption := &controllerOption{}
		for _, option := range options {
			controllerOption = option.apply(controllerOption)
		}

		controlerCfg := &controller.Config{
			Config:             *cfg,
			ExecutableReporter: controllerOption.executableReporter,
			ReporterFactory:    controllerOption.reporterFactory,
			OnShutdown:         controllerOption.onShutdown,
		}

		return internal.NewController(controlerCfg, rs, nextConsumer)
	}
}

func defaultConfig() component.Config {
	return &config.Config{
		ReporterInterval:       5 * time.Second,
		MonitorInterval:        5 * time.Second,
		SamplesPerSecond:       20,
		ProbabilisticInterval:  1 * time.Minute,
		ProbabilisticThreshold: 100,
		Tracers:                "all",
		ClockSyncInterval:      3 * time.Minute,
		MaxGRPCRetries:         5,
		MaxRPCMsgSize:          32 << 20, // 32 MiB,
	}
}
