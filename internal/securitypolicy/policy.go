// Package securitypolicy implements operator-defined restrictions on which
// Alloy components and features may be used at runtime.
package securitypolicy

import (
	"fmt"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// PolicySection defines a single gate as either an allowlist or a denylist.
// When Mode is empty the section is absent and all names are permitted.
type PolicySection struct {
	// Mode is either "allowlist" or "denylist".
	Mode string `yaml:"mode"`
	// List is the set of names the mode applies to.
	List []string `yaml:"list"`
}

func (s PolicySection) check(kind, name string) error {
	switch s.Mode {
	case "":
		return nil
	case "allowlist":
		if !slices.Contains(s.List, name) {
			return fmt.Errorf("%s %q is not in the security policy allowlist", kind, name)
		}
	case "denylist":
		if slices.Contains(s.List, name) {
			return fmt.Errorf("%s %q is in the security policy denylist", kind, name)
		}
	default:
		return fmt.Errorf("unknown security policy mode %q for %s", s.Mode, kind)
	}
	return nil
}

// EndpointsSection gates outbound URLs using glob patterns.
type EndpointsSection struct {
	// Mode is either "allowlist" or "denylist".
	Mode string `yaml:"mode"`
	// Patterns are URL globs (doublestar syntax). Use ** to cross path separators.
	// Example: "https://grafana.com/**"
	Patterns []string `yaml:"patterns"`
}

func (s EndpointsSection) checkURL(rawURL string) error {
	if s.Mode == "" {
		return nil
	}
	normalized, err := normalizeURL(rawURL)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL %q: %w", rawURL, err)
	}
	switch s.Mode {
	case "allowlist":
		for _, pat := range s.Patterns {
			if ok, _ := doublestar.Match(pat, normalized); ok {
				return nil
			}
		}
		return fmt.Errorf("endpoint %q is not in the security policy allowlist", rawURL)
	case "denylist":
		for _, pat := range s.Patterns {
			if ok, _ := doublestar.Match(pat, normalized); ok {
				return fmt.Errorf("endpoint %q matches security policy denylist pattern %q", rawURL, pat)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown security policy mode %q for endpoints", s.Mode)
	}
}

// normalizeURL lowercases scheme+host, strips default ports, and cleans the path.
func normalizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	u.Scheme = strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port != "" {
		p, err := strconv.Atoi(port)
		if err == nil {
			if (u.Scheme == "http" && p == 80) || (u.Scheme == "https" && p == 443) {
				port = ""
			}
		}
	}
	if port != "" {
		u.Host = host + ":" + port
	} else {
		u.Host = host
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String(), nil
}

// SecurityPolicy is the top-level policy document.
// A nil *SecurityPolicy permits everything.
type SecurityPolicy struct {
	Components PolicySection    `yaml:"components"`
	Endpoints  EndpointsSection `yaml:"endpoints"`
}

// CheckComponent returns an error if name is not permitted by the components section.
func (p *SecurityPolicy) CheckComponent(name string) error {
	if p == nil {
		return nil
	}
	return p.Components.check("component", name)
}

// CheckEndpoint returns an error if rawURL is not permitted by the endpoints section.
func (p *SecurityPolicy) CheckEndpoint(rawURL string) error {
	if p == nil {
		return nil
	}
	return p.Endpoints.checkURL(rawURL)
}

// LoadFromFile reads and parses a YAML security policy from path.
// Returns nil, nil when path is empty (no policy configured).
func LoadFromFile(path string) (*SecurityPolicy, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading security policy file: %w", err)
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	var p SecurityPolicy
	if err := dec.Decode(&p); err != nil {
		if err.Error() == "EOF" {
			// Empty file — no sections configured, treat as nil (allow all).
			return nil, nil
		}
		return nil, fmt.Errorf("parsing security policy file: %w", err)
	}
	return &p, nil
}
