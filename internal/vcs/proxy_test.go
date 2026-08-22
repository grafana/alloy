package vcs

import (
	"net/url"
	"testing"

	"github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
)

func Test_newProxyOptions_NilProxyFn(t *testing.T) {
	opts, err := newProxyOptions("https://github.com/example/repo.git", &config.ProxyConfig{})
	require.NoError(t, err)
	require.Empty(t, opts.URL)
	require.Empty(t, opts.Username)
	require.Empty(t, opts.Password)
}

func Test_newProxyOptions_InvalidRepoURL(t *testing.T) {
	proxyURL, _ := url.Parse("http://proxy.example.com:8080")
	cfg := &config.ProxyConfig{
		ProxyURL: config.URL{URL: proxyURL},
	}
	_, err := newProxyOptions("://not-a-valid-url", cfg)
	require.Error(t, err)
}

func Test_newProxyOptions_ProxyURLNoCredentials(t *testing.T) {
	proxyURL, _ := url.Parse("http://proxy.example.com:8080")
	cfg := &config.ProxyConfig{
		ProxyURL: config.URL{URL: proxyURL},
	}
	opts, err := newProxyOptions("https://github.com/example/repo.git", cfg)
	require.NoError(t, err)
	require.Equal(t, "http://proxy.example.com:8080", opts.URL)
	require.Empty(t, opts.Username)
	require.Empty(t, opts.Password)
}

func Test_newProxyOptions_ProxyURLWithUsernameOnly(t *testing.T) {
	proxyURL, _ := url.Parse("http://username@proxy.example.com:8080")
	cfg := &config.ProxyConfig{
		ProxyURL: config.URL{URL: proxyURL},
	}
	opts, err := newProxyOptions("https://github.com/example/repo.git", cfg)
	require.NoError(t, err)
	require.Equal(t, "http://proxy.example.com:8080", opts.URL)
	require.Equal(t, "username", opts.Username)
	require.Empty(t, opts.Password)
}

func Test_newProxyOptions_ProxyURLWithUsernameAndPassword(t *testing.T) {
	proxyURL, _ := url.Parse("http://username:password@proxy.example.com:8080")
	cfg := &config.ProxyConfig{
		ProxyURL: config.URL{URL: proxyURL},
	}
	opts, err := newProxyOptions("https://github.com/example/repo.git", cfg)
	require.NoError(t, err)
	require.Equal(t, "http://proxy.example.com:8080", opts.URL)
	require.Equal(t, "username", opts.Username)
	require.Equal(t, "password", opts.Password)
}

func Test_newProxyOptions_NoProxy(t *testing.T) {
	proxyURL, _ := url.Parse("http://username:password@proxy.example.com:8080")
	cfg := &config.ProxyConfig{
		ProxyURL: config.URL{URL: proxyURL},
		NoProxy:  "github.com",
	}
	opts, err := newProxyOptions("https://github.com/example/repo.git", cfg)
	require.NoError(t, err)
	require.Empty(t, opts.URL)
	require.Empty(t, opts.Username)
	require.Empty(t, opts.Password)
}
