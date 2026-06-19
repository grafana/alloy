package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderGolden(t *testing.T) {
	testCases := []struct {
		name   string
		golden string
		data   templateData
	}{
		{
			name:   "homebrew-core",
			golden: "core.golden",
			data: templateData{
				AlloyBin:          "/opt/homebrew/opt/grafana-alloy/bin/alloy",
				ConfigPath:        "/opt/homebrew/etc/grafana-alloy",
				StoragePath:       "/opt/homebrew/var/lib/grafana-alloy/data",
				EnvFile:           "/opt/homebrew/etc/grafana-alloy/config.env",
				ExtraArgsFile:     "/opt/homebrew/etc/grafana-alloy/extra-args.txt",
				OtelExtraArgsFile: "/opt/homebrew/etc/grafana-alloy/otel-extra-args.txt",
			},
		},
		{
			name:   "homebrew-grafana",
			golden: "grafana.golden",
			data: templateData{
				AlloyBin:          "/opt/homebrew/opt/alloy/bin/alloy",
				ConfigPath:        "/opt/homebrew/etc/alloy/config.alloy",
				StoragePath:       "/opt/homebrew/var/lib/alloy/data",
				EnvFile:           "/opt/homebrew/etc/alloy/config.env",
				ExtraArgsFile:     "/opt/homebrew/etc/alloy/extra-args.txt",
				OtelExtraArgsFile: "/opt/homebrew/etc/alloy/otel-extra-args.txt",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := render(tc.data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}

			goldenPath := filepath.Join("testdata", tc.golden)
			if os.Getenv("UPDATE_GOLDEN") != "" {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("update golden: %v", err)
				}
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}

			if string(got) != string(want) {
				t.Errorf("rendered wrapper mismatch.\nWant:\n%s\nGot:\n%s", want, got)
			}
		})
	}
}

func TestRenderValidation(t *testing.T) {
	testCases := []struct {
		name string
		data templateData
	}{
		{
			name: "missing field",
			data: templateData{AlloyBin: "/opt/alloy"},
		},
		{
			name: "quote in value",
			data: templateData{
				AlloyBin:      "/opt/al\"loy",
				ConfigPath:    "/etc/alloy",
				StoragePath:   "/var/lib/alloy",
				EnvFile:       "/etc/alloy/config.env",
				ExtraArgsFile: "/etc/alloy/extra-args.txt",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := render(tc.data); err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}
