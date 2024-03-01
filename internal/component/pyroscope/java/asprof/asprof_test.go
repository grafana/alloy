//go:build linux

package asprof

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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
	p := NewProfiler(dir, EmbeddedArchive)
	p.tmpDirMarker = fmt.Sprintf("grafana-agent-asprof-%s", uuid.NewString())
	t.Logf("tmpDirMarker: %s", p.tmpDirMarker)
	err := p.ExtractDistributions()
	assert.NoError(t, err)
}

func TestOwnedDir(t *testing.T) {
	dir := tempDir(t)
	err := os.Chmod(dir, 0755)
	assert.NoError(t, err)
	p := NewProfiler(dir, EmbeddedArchive)
	err = p.ExtractDistributions()
	assert.NoError(t, err)
}

func TestOwnedDirWrongPermission(t *testing.T) {
	dir := tempDir(t)
	err := os.Chmod(dir, 0777)
	assert.NoError(t, err)
	p := NewProfiler(dir, EmbeddedArchive)
	err = p.ExtractDistributions()
	assert.Error(t, err)
}

func TestDistSymlink(t *testing.T) {
	// check if /tmp/dist-... is a symlink
	td := []bool{true, false}
	for _, glibc := range td {
		t.Run(fmt.Sprintf("glibc=%t", glibc), func(t *testing.T) {
			root := tempDir(t)
			err := os.Chmod(root, 0755)
			assert.NoError(t, err)
			manipulated := tempDir(t)
			err = os.Chmod(manipulated, 0755)
			assert.NoError(t, err)
			p := NewProfiler(root, EmbeddedArchive)
			muslDistName, glibcDistName := p.getDistNames()

			if glibc {
				err = os.Symlink(manipulated, filepath.Join(root, muslDistName))
				assert.NoError(t, err)
			} else {
				err = os.Symlink(manipulated, filepath.Join(root, glibcDistName))
				assert.NoError(t, err)
			}

			err = p.ExtractDistributions()
			t.Logf("expected %s", err)
			assert.Error(t, err)
		})
	}
}

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "asprof-test")
	assert.NoError(t, err)
	t.Logf("dir: %s", dir)
	return dir
}
