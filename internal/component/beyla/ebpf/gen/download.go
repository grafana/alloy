//go:build ignore

package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintf(os.Stderr, "usage: download.go <version> <amd64-path> <arm64-path> <stamp-path>\n")
		os.Exit(1)
	}

	version := os.Args[1]
	paths := map[string]string{
		"amd64": os.Args[2],
		"arm64": os.Args[3],
	}
	stampPath := os.Args[4]

	if upToDate(version, stampPath, paths) {
		fmt.Printf("  Beyla %s binaries already up to date\n", version)
		return
	}

	fmt.Printf("Downloading Beyla %s binaries...\n", version)

	for _, arch := range []string{"amd64", "arm64"} {
		url := fmt.Sprintf(
			"https://github.com/grafana/beyla/releases/download/%s/beyla-linux-%s-%s.tar.gz",
			version, arch, version,
		)
		dest := paths[arch]
		if err := downloadBinary(url, dest); err != nil {
			fmt.Fprintf(os.Stderr, "error downloading %s: %v\n", arch, err)
			os.Exit(1)
		}
	}

	if err := os.WriteFile(stampPath, []byte(version+"\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing stamp: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  ✓ Downloaded Beyla %s binaries\n", version)
	for _, arch := range []string{"amd64", "arm64"} {
		if fi, err := os.Stat(paths[arch]); err == nil {
			fmt.Printf("    %s: %d MB\n", paths[arch], fi.Size()/1024/1024)
		}
	}
}

func upToDate(version, stampPath string, paths map[string]string) bool {
	data, err := os.ReadFile(stampPath)
	if err != nil {
		return false
	}
	if strings.TrimSpace(string(data)) != version {
		return false
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			return false
		}
	}
	return true
}

func downloadBinary(url, destPath string) error {
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %s", url, resp.Status)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != "beyla" {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("create %s: %w", destPath, err)
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return fmt.Errorf("write %s: %w", destPath, err)
		}
		return f.Close()
	}

	return fmt.Errorf("beyla binary not found in tarball: %s", url)
}
