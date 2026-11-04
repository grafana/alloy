//go:build !windows

package positions

import (
	"os"
	"path/filepath"

	renameio "github.com/google/renameio/v2"
	yaml "gopkg.in/yaml.v2"
)

const positionFileMode = 0600

func writePositionFile(filename string, positions map[Entry]string) error {
	buf, err := yaml.Marshal(positionFile{
		Positions: positions,
	})
	if err != nil {
		return err
	}

	target := filepath.Clean(filename)

	return renameio.WriteFile(target, buf, os.FileMode(positionFileMode))
}
