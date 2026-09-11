package supportbundle

import (
	"archive/zip"
	"io"
	"path"

	"github.com/grafana/alloy/extension/supportbundle/internal/gather"
)

// writeBundle writes the files as a zip archive to w.
// Each file entry is placed under root.
func writeBundle(w io.Writer, root string, files []gather.File) error {
	zw := zip.NewWriter(w)

	for _, f := range files {
		entry, err := zw.Create(path.Join(root, f.Path))
		if err != nil {
			return err
		}
		if _, err := entry.Write(f.Content); err != nil {
			return err
		}
	}

	return zw.Close()
}
