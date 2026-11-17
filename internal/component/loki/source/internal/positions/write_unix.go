//go:build !windows

package positions

import (
	"os"

	renameio "github.com/google/renameio/v2"
)

const positionFileMode = 0600

func writeFileAtomic(path string, buf []byte) error {
	return renameio.WriteFile(path, buf, os.FileMode(positionFileMode))
}
