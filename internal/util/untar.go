package util

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Untar untars a tarball to the provided destination path.
func Untar(tarPath string, destPath string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Create destination path.
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}

	tr := tar.NewReader(f)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return err
		}

		// Protect from a zip slip.
		// https://security.snyk.io/research/zip-slip-vulnerability
		if strings.Contains(header.Name, `../`) ||
			strings.Contains(header.Name, `..\`) {

			return fmt.Errorf("tar: invalid file path: %s", header.Name)
		}

		fullPath := filepath.Join(destPath, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				return err
			}

		case tar.TypeReg:
			f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
			_ = f.Close()

		default:
			return fmt.Errorf("unsupported type: %v", header.Typeflag)
		}
	}

	return nil
}
