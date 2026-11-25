//go:build (linux && arm64) || (linux && amd64)

package subprocess

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartPassesExtraFilesAtFixedDescriptors(t *testing.T) {
	shell, err := os.ReadFile("/bin/sh")
	if err != nil {
		t.Skipf("no shell to run: %v", err)
	}

	// Free some low descriptors after opening the extra files, so that Start creates the
	// binary's memfd with a number among those the extra files take in the child.
	var placeholders []*os.File
	for range 8 {
		f, err := os.Open(os.DevNull)
		require.NoError(t, err)
		placeholders = append(placeholders, f)
	}

	extraFiles := make([]*os.File, 8)
	for i := range extraFiles {
		path := filepath.Join(t.TempDir(), strconv.Itoa(i))
		require.NoError(t, os.WriteFile(path, []byte(strconv.Itoa(i)), 0o600))
		extraFiles[i], err = os.Open(path)
		require.NoError(t, err)
	}

	for _, f := range placeholders {
		require.NoError(t, f.Close())
	}

	var stdout, stderr bytes.Buffer
	proc, err := Start(t.Context(), Spec{
		Name:       "sh",
		Binary:     shell,
		Args:       []string{"-c", "cat /proc/self/fd/3 /proc/self/fd/10"},
		ExtraFiles: extraFiles,
		Stdout:     &stdout,
		Stderr:     &stderr,
	})
	require.NoError(t, err)
	require.NoError(t, proc.Wait(), stderr.String())

	require.Equal(t, "07", stdout.String())
}
