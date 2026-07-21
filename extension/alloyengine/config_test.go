package alloyengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name:    "no config source",
			wantErr: "either config.path or config.inline.content must be set",
		},
		{
			name: "path and file",
			config: Config{AlloyConfig: AlloyConfig{
				Path: "config.alloy",
				File: "legacy.alloy",
			}},
			wantErr: "config.path and config.file are mutually exclusive; config.file is deprecated, use config.path",
		},
		{
			name: "path and inline",
			config: Config{AlloyConfig: AlloyConfig{
				Path:   "config.alloy",
				Inline: InlineAlloyConfig{Content: "logging {}"},
			}},
			wantErr: "exactly one of config.path or config.inline.content must be set",
		},
		{
			name: "file and inline",
			config: Config{AlloyConfig: AlloyConfig{
				File:   "legacy.alloy",
				Inline: InlineAlloyConfig{Content: "logging {}"},
			}},
			wantErr: "exactly one of config.path or config.inline.content must be set",
		},
		{
			name: "path file and inline",
			config: Config{AlloyConfig: AlloyConfig{
				Path:   "config.alloy",
				File:   "legacy.alloy",
				Inline: InlineAlloyConfig{Content: "logging {}"},
			}},
			wantErr: "exactly one of config.path or config.inline.content must be set",
		},
		{
			name: "module path without inline content",
			config: Config{AlloyConfig: AlloyConfig{
				Inline: InlineAlloyConfig{ModulePath: "/modules"},
			}},
			wantErr: "either config.path or config.inline.content must be set",
		},
		{
			name: "path",
			config: Config{AlloyConfig: AlloyConfig{
				Path: "config.alloy",
			}},
		},
		{
			name: "file",
			config: Config{AlloyConfig: AlloyConfig{
				File: "legacy.alloy",
			}},
		},
		{
			name: "inline",
			config: Config{AlloyConfig: AlloyConfig{
				Inline: InlineAlloyConfig{
					ModulePath: "/modules",
					Content:    "logging {}",
				},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
