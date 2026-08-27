package main

import "testing"

func TestIsOtelMode(t *testing.T) {
	truthy := []string{"1", "true", "yes", "on"}
	for _, v := range truthy {
		if !isOtelMode(v) {
			t.Errorf("isOtelMode(%q) = false, want true", v)
		}
	}

	notTruthy := []string{"", "0", "false", "no", "off", "maybe", "TRUE", "Yes", "ON"}
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
			name:              "truthy toggle is case-sensitive: wrong case stays default",
			otelMode:          "TRUE",
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
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveEngineArgs(tc.otelMode, tc.otelConfigDefault, tc.defaultEngineArgs)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}
