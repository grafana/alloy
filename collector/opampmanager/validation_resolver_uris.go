package opampmanager

import (
	"net/url"
	"path/filepath"
	"strings"
)

func MergeResolverURIsForValidation(configURIs, setURIs []string, effectivePath string, candidateYAML []byte) []string {
	yamlCand := "yaml:" + string(candidateYAML)
	if len(configURIs) == 0 && len(setURIs) == 0 {
		return []string{yamlCand}
	}

	effKey, ok := canonicalConfigPath(effectivePath)
	outCfg := append([]string(nil), configURIs...)
	replaced := false
	if ok {
		for i, u := range outCfg {
			if c, ok2 := canonicalConfigPath(u); ok2 && c == effKey {
				outCfg[i] = yamlCand
				replaced = true
				break
			}
		}
	}
	if !replaced {
		outCfg = append(outCfg, yamlCand)
	}
	return append(outCfg, setURIs...)
}

func canonicalConfigPath(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if strings.HasPrefix(s, "file:") {
		u, err := url.Parse(s)
		if err != nil {
			return "", false
		}
		if u.Scheme != "file" {
			return "", false
		}
		p := u.Path
		if p == "" && u.Opaque != "" {
			p = u.Opaque
		}
		if p == "" {
			return "", false
		}
		abs, err := filepath.Abs(filepath.Clean(p))
		if err != nil {
			return "", false
		}
		return abs, true
	}
	abs, err := filepath.Abs(filepath.Clean(s))
	if err != nil {
		return "", false
	}
	return abs, true
}
