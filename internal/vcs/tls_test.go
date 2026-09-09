package vcs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
)

func Test_newGitTLSConfig_Empty(t *testing.T) {
	cfg, err := newGitTLSConfig(&config.TLSConfig{})
	require.NoError(t, err)
	require.False(t, cfg.InsecureSkipTLS)
	require.Nil(t, cfg.ClientCert)
	require.Nil(t, cfg.ClientKey)
	require.Nil(t, cfg.CABundle)
}

func Test_newGitTLSConfig_InsecureSkipVerify(t *testing.T) {
	cfg, err := newGitTLSConfig(&config.TLSConfig{InsecureSkipVerify: true})
	require.NoError(t, err)
	require.True(t, cfg.InsecureSkipTLS)
}

func Test_newGitTLSConfig_InlineValues(t *testing.T) {
	cfg, err := newGitTLSConfig(&config.TLSConfig{
		Cert: "cert-data",
		Key:  config.Secret("key-data"),
		CA:   "ca-data",
	})
	require.NoError(t, err)
	require.Equal(t, []byte("cert-data"), cfg.ClientCert)
	require.Equal(t, []byte("key-data"), cfg.ClientKey)
	require.Equal(t, []byte("ca-data"), cfg.CABundle)
}

func Test_newGitTLSConfig_FileValues(t *testing.T) {
	dir := t.TempDir()
	certFile := writeFile(t, dir, "cert.pem", "cert-file-data")
	keyFile := writeFile(t, dir, "key.pem", "key-file-data")
	caFile := writeFile(t, dir, "ca.pem", "ca-file-data")

	cfg, err := newGitTLSConfig(&config.TLSConfig{
		CertFile: certFile,
		KeyFile:  keyFile,
		CAFile:   caFile,
	})
	require.NoError(t, err)
	require.Equal(t, []byte("cert-file-data"), cfg.ClientCert)
	require.Equal(t, []byte("key-file-data"), cfg.ClientKey)
	require.Equal(t, []byte("ca-file-data"), cfg.CABundle)
}

func Test_newGitTLSConfig_CertFileMissing(t *testing.T) {
	_, err := newGitTLSConfig(&config.TLSConfig{
		CertFile: "/nonexistent/cert.pem",
	})
	require.ErrorContains(t, err, "unable to load server certificate")
}

func Test_newGitTLSConfig_KeyFileMissing(t *testing.T) {
	_, err := newGitTLSConfig(&config.TLSConfig{
		KeyFile: "/nonexistent/key.pem",
	})
	require.ErrorContains(t, err, "unable to load server key")
}

func Test_newGitTLSConfig_CAFileMissing(t *testing.T) {
	_, err := newGitTLSConfig(&config.TLSConfig{
		CAFile: "/nonexistent/ca.pem",
	})
	require.ErrorContains(t, err, "unable to load client CA certificate")
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
	return path
}
