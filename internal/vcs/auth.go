package vcs

import (
	"fmt"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/grafana/alloy/syntax/alloytypes"
)

type GitAuthConfig struct {
	BasicAuth *BasicAuth `alloy:"basic_auth,block,optional"`
	SSHKey    *SSHKey    `alloy:"ssh_key,block,optional"`
}

// Convert converts HTTPClientConfig to the native Prometheus type. If h is
// nil, the default client config is returned.
func (h *GitAuthConfig) Convert() (transport.AuthMethod, error) {
	if h == nil {
		return nil, nil
	}
	if h.BasicAuth != nil {
		return h.BasicAuth.Convert(), nil
	}

	if h.SSHKey != nil {
		return h.SSHKey.Convert()
	}
	return nil, nil
}

type BasicAuth struct {
	Username string            `alloy:"username,attr"`
	Password alloytypes.Secret `alloy:"password,attr"`
}

// Convert converts our type to the native prometheus type
func (b *BasicAuth) Convert() (t transport.AuthMethod) {
	if b == nil {
		return nil
	}
	return &http.BasicAuth{
		Username: b.Username,
		Password: string(b.Password),
	}
}

type SSHKey struct {
	Username   string            `alloy:"username,attr"`
	Key        alloytypes.Secret `alloy:"key,attr,optional"`
	Keyfile    string            `alloy:"key_file,attr,optional"`
	Passphrase alloytypes.Secret `alloy:"passphrase,attr,optional"`
}

// Convert converts our type to the native prometheus type
func (s *SSHKey) Convert() (transport.AuthMethod, error) {
	if s == nil {
		return nil, nil
	}

	if s.Key != "" {
		publickeys, err := ssh.NewPublicKeys(s.Username, []byte(s.Key), string(s.Passphrase))
		if err != nil {
			return nil, fmt.Errorf("loading SSH keys failed: %s", err.Error())
		}
		return publickeys, nil
	}

	if s.Keyfile != "" {
		publickeys, err := ssh.NewPublicKeysFromFile(s.Username, s.Keyfile, string(s.Passphrase))
		if err != nil {
			return nil, fmt.Errorf("loading SSH keys failed: %s", err.Error())
		}
		return publickeys, nil
	}

	return nil, nil
}
