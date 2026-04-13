//go:build alloyintegrationtests && k8sv2integrationtests

package k8sv2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderTemplatedFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "input.yaml")
	if err := os.WriteFile(src, []byte("name: ${TEST_NAMESPACE}\nlabel: ${TEST_ID}\n"), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	rendered, err := renderTemplatedFile(src, map[string]string{
		"TEST_ID":        "logs-loki-deadbeef",
		"TEST_NAMESPACE": "logs-loki-deadbeef",
	})
	if err != nil {
		t.Fatalf("render template failed: %v", err)
	}
	defer removeTempFile(rendered)

	raw, err := os.ReadFile(rendered)
	if err != nil {
		t.Fatalf("read rendered file: %v", err)
	}
	body := string(raw)
	if strings.Contains(body, "${") {
		t.Fatalf("rendered file still has placeholders: %q", body)
	}
	if !strings.Contains(body, "logs-loki-deadbeef") {
		t.Fatalf("rendered file missing expected substitutions: %q", body)
	}
}
