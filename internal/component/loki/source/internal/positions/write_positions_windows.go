//go:build windows

package positions

import (
	"bytes"

	"github.com/natefinch/atomic"
	yaml "gopkg.in/yaml.v2"
)

func writePositionFile(filename string, positions map[Entry]string) error {
	buf, err := yaml.Marshal(positionFile{
		Positions: positions,
	})
	if err != nil {
		return err
	}
	return atomic.WriteFile(filename, bytes.NewReader(buf))

}
