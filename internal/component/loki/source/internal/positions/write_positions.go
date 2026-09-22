package positions

// This code is copied from Promtail. The positions package allows logging
// components to keep track of read file offsets on disk and continue from the
// same place in case of a restart.

import (
	"os"

	yaml "gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/util/atomicfile"
)

const positionFileMode os.FileMode = 0600

func writePositionFile(filename string, positions map[Entry]string) error {
	buf, err := yaml.Marshal(File{
		Positions: positions,
	})
	if err != nil {
		return err
	}

	return atomicfile.Write(filename, buf, positionFileMode)
}
