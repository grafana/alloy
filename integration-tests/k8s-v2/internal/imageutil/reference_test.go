package imageutil

import "testing"

func TestSplitReference(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantRepo string
		wantTag  string
		wantErr  bool
	}{
		{
			name:     "simple repository tag",
			input:    "alloy-ci:dev",
			wantRepo: "alloy-ci",
			wantTag:  "dev",
		},
		{
			name:     "registry with port and nested repository",
			input:    "localhost:5000/grafana/alloy:pr-123",
			wantRepo: "localhost:5000/grafana/alloy",
			wantTag:  "pr-123",
		},
		{
			name:    "digest reference rejected",
			input:   "docker.io/grafana/alloy@sha256:abc123",
			wantErr: true,
		},
		{
			name:    "missing tag rejected",
			input:   "docker.io/grafana/alloy",
			wantErr: true,
		},
		{
			name:    "empty string rejected",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			repo, tag, err := SplitReference(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none (repo=%q tag=%q)", repo, tag)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo != tt.wantRepo || tag != tt.wantTag {
				t.Fatalf("unexpected split: got repo=%q tag=%q; want repo=%q tag=%q", repo, tag, tt.wantRepo, tt.wantTag)
			}
		})
	}
}
