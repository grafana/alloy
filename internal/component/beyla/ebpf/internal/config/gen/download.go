//go:build ignore

package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// maxBeylaBinarySize caps the extracted binary to bound a malicious/compression-bomb archive (real binary is ~120 MB).
	maxBeylaBinarySize = 512 << 20 // 512 MB
	// maxTarballSize bounds the downloaded release tarball (real tarballs are ~50 MB).
	maxTarballSize = 256 << 20 // 256 MB
)

var httpClient = &http.Client{Timeout: 2 * time.Minute}

func assetName(arch, version string) string {
	return fmt.Sprintf("beyla-linux-%s-%s.tar.gz", arch, version)
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "--update-checksums" {
		if len(os.Args) != 4 {
			fmt.Fprintf(os.Stderr, "usage: download.go --update-checksums <version> <version-file>\n")
			os.Exit(1)
		}
		if err := updateVersionFile(os.Args[2], os.Args[3]); err != nil {
			fmt.Fprintf(os.Stderr, "error updating version file: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) != 5 {
		fmt.Fprintf(os.Stderr, "usage: download.go <amd64-path> <arm64-path> <stamp-path> <version-file>\n")
		os.Exit(1)
	}

	paths := map[string]string{
		"amd64": os.Args[1],
		"arm64": os.Args[2],
	}
	stampPath := os.Args[3]
	versionFile := os.Args[4]

	version, checksums, err := readVersionFile(versionFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", versionFile, err)
		os.Exit(1)
	}

	if upToDate(version, stampPath, paths) {
		fmt.Printf("  Beyla %s binaries already up to date\n", version)
		return
	}

	fmt.Printf("Downloading Beyla %s binaries...\n", version)

	for _, arch := range []string{"amd64", "arm64"} {
		want, ok := checksums[arch]
		if !ok {
			fmt.Fprintf(os.Stderr, "no committed checksum for %s in %s; run `make update-beyla`\n", arch, versionFile)
			os.Exit(1)
		}
		url := fmt.Sprintf("https://github.com/grafana/beyla/releases/download/%s/%s", version, assetName(arch, version))
		if err := downloadBinary(url, paths[arch], want); err != nil {
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

// readVersionFile parses beyla_version.yaml: the pinned Beyla version and each
// arch's release-tarball sha256. These committed values are the trust anchor —
// downloads are verified against them, so a compromised upstream can't swap the
// binary for one we didn't review.
func readVersionFile(path string) (version string, checksums map[string]string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	checksums = make(map[string]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		switch key {
		case "version":
			version = val
		case "amd64", "arm64":
			checksums[key] = val
		}
	}
	if err := sc.Err(); err != nil {
		return "", nil, err
	}
	if version == "" {
		return "", nil, fmt.Errorf("no version in %s", path)
	}
	return version, checksums, nil
}

// updateVersionFile records the release's version and each tarball's sha256 into
// beyla_version.yaml. The recorded values are trusted on first use, at PR review
// time; thereafter downloads verify against them.
func updateVersionFile(version, path string) error {
	digests, err := fetchAssetDigests(version)
	if err != nil {
		return err
	}

	checksums := make(map[string]string, 2)
	for _, arch := range []string{"amd64", "arm64"} {
		digest, ok := digests[assetName(arch, version)]
		if !ok {
			return fmt.Errorf("no published digest for %s", assetName(arch, version))
		}
		checksums[arch] = strings.TrimPrefix(digest, "sha256:")
	}

	out := fmt.Sprintf("# Beyla release pinned by Alloy. Regenerate with: make update-beyla TAG=<beyla-version>\n"+
		"version: %s\nchecksums:\n  amd64: %s\n  arm64: %s\n", version, checksums["amd64"], checksums["arm64"])
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		return err
	}
	fmt.Printf("  ✓ recorded Beyla %s in %s\n", version, path)
	return nil
}

// fetchAssetDigests returns each release asset's published "sha256:..." digest,
// keyed by asset name. GITHUB_TOKEN, when set, raises the API rate limit.
func fetchAssetDigests(version string) (map[string]string, error) {
	api := fmt.Sprintf("https://api.github.com/repos/grafana/beyla/releases/tags/%s", version)
	req, err := http.NewRequest(http.MethodGet, api, nil) //nolint:noctx
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", api, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %s", api, resp.Status)
	}

	var rel struct {
		Assets []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release metadata: %w", err)
	}

	digests := make(map[string]string, len(rel.Assets))
	for _, a := range rel.Assets {
		if a.Digest != "" {
			digests[a.Name] = a.Digest
		}
	}
	return digests, nil
}

func downloadBinary(url, destPath, wantHex string) error {
	resp, err := httpClient.Get(url) //nolint:noctx
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %s", url, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxTarballSize+1))
	if err != nil {
		return fmt.Errorf("read %s: %w", url, err)
	}
	if int64(len(data)) > maxTarballSize {
		return fmt.Errorf("tarball %s exceeds %d bytes", url, maxTarballSize)
	}

	// Verify against the committed checksum before extracting.
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != wantHex {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", url, got, wantHex)
	}

	return extractBeyla(bytes.NewReader(data), destPath, url)
}

func extractBeyla(r io.Reader, destPath, url string) error {
	gz, err := gzip.NewReader(r)
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
		if hdr.Size < 0 || hdr.Size > maxBeylaBinarySize {
			return fmt.Errorf("beyla entry size %d out of range (max %d)", hdr.Size, maxBeylaBinarySize)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("create %s: %w", destPath, err)
		}
		// Bound the copy to the header-declared (capped) size so a bad archive can't write unbounded data.
		if _, err := io.CopyN(f, tr, hdr.Size); err != nil {
			f.Close()
			return fmt.Errorf("write %s: %w", destPath, err)
		}
		return f.Close()
	}

	return fmt.Errorf("beyla binary not found in tarball: %s", url)
}
