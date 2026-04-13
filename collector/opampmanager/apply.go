package opampmanager

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".opampcfg-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if perm != 0 {
		if err := f.Chmod(perm); err != nil {
			_ = f.Close()
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func copyFileAtomic(dst, src string, perm os.FileMode) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return atomicWriteFile(dst, b, perm)
}

func probeHealth(ctx context.Context, rawURL string, insecure bool) error {
	if rawURL == "" {
		return nil
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = &tls.Config{}
		}
		tr.TLSClientConfig.InsecureSkipVerify = true
	}
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: tr,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("health probe: status %d: %s", resp.StatusCode, string(body))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
