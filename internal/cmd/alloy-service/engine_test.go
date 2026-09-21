package main

import (
	"slices"
	"testing"
)

func TestIsOtelMode(t *testing.T) {
	truthy := []string{"1", "true", "yes", "on", "TRUE", "Yes", "ON"}
	for _, v := range truthy {
		if !isOtelMode(v) {
			t.Errorf("isOtelMode(%q) = false, want true", v)
		}
	}

	notTruthy := []string{"", "0", "false", "no", "off", "maybe"}
	for _, v := range notTruthy {
		if isOtelMode(v) {
			t.Errorf("isOtelMode(%q) = true, want false", v)
		}
	}
}

func TestResolveEngineArgs(t *testing.T) {
	defaultArgs := []string{"run", `C:\ProgramData\GrafanaLabs\Alloy\config.alloy`, `--storage.path=C:\ProgramData\GrafanaLabs\Alloy\data`}

	tt := []struct {
		name              string
		otelMode          string
		otelConfigDefault string
		otelArguments     []string
		defaultEngineArgs []string
		want              []string
	}{
		{
			name:              "unset toggle: unchanged",
			defaultEngineArgs: defaultArgs,
			want:              defaultArgs,
		},
		{
			name:              "present-but-empty toggle: unchanged",
			otelMode:          "",
			defaultEngineArgs: defaultArgs,
			want:              defaultArgs,
		},
		{
			name:              "non-truthy toggle stays on default engine",
			otelMode:          "maybe",
			defaultEngineArgs: defaultArgs,
			want:              defaultArgs,
		},
		{
			name:              "truthy toggle matching is case-insensitive",
			otelMode:          "TRUE",
			otelConfigDefault: `C:\ProgramData\GrafanaLabs\Alloy\config.yaml`,
			defaultEngineArgs: defaultArgs,
			want:              []string{"otel", `--config=C:\ProgramData\GrafanaLabs\Alloy\config.yaml`},
		},
		{
			name:              "default engine ignores OTelArguments, the OTel engine's flags",
			otelArguments:     []string{"--set=processors.batch.timeout=2s"},
			defaultEngineArgs: defaultArgs,
			want:              defaultArgs,
		},
		{
			name:              "otel mode, config path",
			otelMode:          "1",
			otelConfigDefault: `C:\ProgramData\GrafanaLabs\Alloy\config.yaml`,
			defaultEngineArgs: defaultArgs,
			want:              []string{"otel", `--config=C:\ProgramData\GrafanaLabs\Alloy\config.yaml`},
		},
		{
			name:              "otel mode via other truthy spellings",
			otelMode:          "yes",
			otelConfigDefault: `C:\ProgramData\GrafanaLabs\Alloy\config.yaml`,
			defaultEngineArgs: defaultArgs,
			want:              []string{"otel", `--config=C:\ProgramData\GrafanaLabs\Alloy\config.yaml`},
		},
		{
			name:              "otel mode, OTelArguments applies after --config",
			otelMode:          "1",
			otelConfigDefault: `C:\ProgramData\GrafanaLabs\Alloy\config.yaml`,
			otelArguments:     []string{"--set=processors.batch.timeout=2s"},
			defaultEngineArgs: defaultArgs,
			want:              []string{"otel", `--config=C:\ProgramData\GrafanaLabs\Alloy\config.yaml`, "--set=processors.batch.timeout=2s"},
		},
		{
			name:              "otel mode, a --config in OTelArguments lands after the installer's default --config",
			otelMode:          "1",
			otelConfigDefault: `C:\ProgramData\GrafanaLabs\Alloy\config.yaml`,
			otelArguments:     []string{"--config=C:\\custom\\override.yaml"},
			defaultEngineArgs: defaultArgs,
			want:              []string{"otel", `--config=C:\ProgramData\GrafanaLabs\Alloy\config.yaml`, `--config=C:\custom\override.yaml`},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveEngineArgs(tc.otelMode, tc.otelConfigDefault, tc.otelArguments, tc.defaultEngineArgs)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
