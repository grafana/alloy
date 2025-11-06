//go:build linux || darwin

package asprof

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// extracting to /tmp
// /tmp dir should be sticky or owned 0700 by the current user
// /tmp/dist-... dir should be owned 0700 by the current user and should not be a symlink
// the rest should use mkdirAt, openAt

// test /tmp/dist-... is not symlink to /proc/conatinerpid/root/tmp/dist-
// test /tmp/dist-... is not symlink to /../../../foo

// write skippable tests with uid=0
func TestStickyDir(t *testing.T) {
	dir := "/tmp"
	tmpDirMarker := fmt.Sprintf("alloy-asprof-%s", uuid.NewString())
	t.Logf("tmpDirMarker: %s", tmpDirMarker)
	dist, err := ExtractDistribution(EmbeddedArchive, dir, tmpDirMarker)
	require.NoError(t, err)
	require.NotNil(t, dist)
}

func TestOwnedDir(t *testing.T) {
	dir := t.TempDir()
	err := os.Chmod(dir, 0755)
	require.NoError(t, err)
	dist, err := ExtractDistribution(EmbeddedArchive, dir, "alloy-asprof")
	require.NoError(t, err)
	require.NotNil(t, dist)
}

func TestOwnedDirWrongPermission(t *testing.T) {
	dir := t.TempDir()
	err := os.Chmod(dir, 0777)
	require.NoError(t, err)
	dist, err := ExtractDistribution(EmbeddedArchive, dir, "alloy-asprof-")
	require.Error(t, err)
	require.Empty(t, dist.extractedDir)
}

func TestDistSymlink(t *testing.T) {
	root := t.TempDir()
	err := os.Chmod(root, 0755)
	require.NoError(t, err)
	manipulated := t.TempDir()
	err = os.Chmod(manipulated, 0755)
	require.NoError(t, err)
	distName := "dist"

	err = os.Symlink(manipulated, filepath.Join(root, distName))
	require.NoError(t, err)

	dist, err := ExtractDistribution(EmbeddedArchive, root, distName)
	t.Logf("expected %s", err)
	require.Error(t, err)
	require.Empty(t, dist.extractedDir)
}
