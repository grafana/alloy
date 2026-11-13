package vcs

import (
	"net/http"
	"net/url"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/prometheus/common/config"
)

func newProxyOptions(repository string, config *config.ProxyConfig) (*transport.ProxyOptions, error) {
	proxyFn := config.Proxy()
	if proxyFn == nil {
		return &transport.ProxyOptions{}, nil
	}

	repoUrl, err := url.Parse(repository)
	if err != nil {
		return nil, err
	}
	proxyUrl, err := proxyFn(&http.Request{URL: repoUrl})
	if err != nil {
		return nil, err
	}

	if proxyUrl == nil {
		return &transport.ProxyOptions{}, nil
	}
	if proxyUrl.User == nil {
		return &transport.ProxyOptions{URL: proxyUrl.String()}, nil
	}
	proxyUser := proxyUrl.User
	proxyUrl.User = nil
	proxyOptions := &transport.ProxyOptions{
		URL:      proxyUrl.String(),
		Username: proxyUser.Username(),
	}
	proxyUrl.User = proxyUser
	if password, isSetPassword := proxyUser.Password(); isSetPassword {
		proxyOptions.Password = password
	}
	return proxyOptions, nil
}
