//go:build !windows

package positions

// This code is copied from Promtail. The positions package allows logging
// components to keep track of read file offsets on disk and continue from the
// same place in case of a restart.

import (
	"os"
	"path/filepath"

	renameio "github.com/google/renameio/v2"
	yaml "gopkg.in/yaml.v2"
)

func writePositionFile(filename string, positions map[Entry]string) error {
	buf, err := yaml.Marshal(File{
		Positions: positions,
	})
	if err != nil {
		return err
	}

	target := filepath.Clean(filename)

	return renameio.WriteFile(target, buf, os.FileMode(positionFileMode))
}
