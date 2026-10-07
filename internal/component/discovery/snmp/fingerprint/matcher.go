package fingerprint

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// FingerprintKnown is a sysObjectID that hit a fingerprinter matcher.
	FingerprintKnown = "known"
	// FingerprintUnknown uses the fingerprinter default chain (device_base / if_mib).
	FingerprintUnknown = "unknown"
)

// FingerprintersFile is the SuperQ snmp_exporter#1468 matcher model.
type FingerprintersFile struct {
	Fingerprinters map[string]Fingerprinter `yaml:"fingerprinters"`
}

type Fingerprinter struct {
	ProbeOIDs              []string  `yaml:"probe_oids"`
	DefaultModules         []string  `yaml:"default_modules"`
	DefaultModulesHot      []string  `yaml:"default_modules_hot"`
	DefaultModulesCold     []string  `yaml:"default_modules_cold"`
	DefaultModulesTopology []string  `yaml:"default_modules_topology"`
	Matchers               []Matcher `yaml:"matchers"`
}

type Matcher struct {
	Label           string   `yaml:"label"`
	Regex           string   `yaml:"regex"`
	Modules         []string `yaml:"modules"`
	ModulesHot      []string `yaml:"modules_hot"`
	ModulesCold     []string `yaml:"modules_cold"`
	ModulesTopology []string `yaml:"modules_topology"`
	Comment         string   `yaml:"comment"`
	re              *regexp.Regexp
}

// ModuleTiers is the staggered scrape split (hot≈60s, cold≈30m, topology=optional).
type ModuleTiers struct {
	Hot      []string
	Cold     []string
	Topology []string
}

func (f *Fingerprinter) compile() error {
	for i := range f.Matchers {
		re, err := regexp.Compile(f.Matchers[i].Regex)
		if err != nil {
			return err
		}
		f.Matchers[i].re = re
	}
	return nil
}

func LoadFingerprinters(path string) (FingerprintersFile, error) {
	var fps FingerprintersFile
	b, err := os.ReadFile(path)
	if err != nil {
		return fps, err
	}
	if err := yaml.Unmarshal(b, &fps); err != nil {
		return fps, err
	}
	if len(fps.Fingerprinters) == 0 {
		return fps, fmt.Errorf("no fingerprinters in %s", path)
	}
	for _, fp := range fps.Fingerprinters {
		if err := fp.compile(); err != nil {
			return fps, fmt.Errorf("failed to compile matchers: %w", err)
		}
	}
	return fps, nil
}

func normalizeOID(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, ".")
	s = strings.TrimPrefix(s, "iso.")
	return s
}

func matcherHit(m Matcher, labels map[string]string) bool {
	val := labels[m.Label]
	if val == "" {
		return false
	}
	cand := []string{val, normalizeOID(val), "." + normalizeOID(val)}
	for _, c := range cand {
		if m.re != nil && m.re.MatchString(c) {
			return true
		}
	}
	return false
}

func (m Matcher) hasTier() bool {
	return len(m.ModulesHot)+len(m.ModulesCold)+len(m.ModulesTopology) > 0
}

func isTopologyModule(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "lldp_mib", "cdp_mib", "bgp4_mib", "ospf_mib":
		return true
	}
	for _, suf := range []string{"_topo", "_bgp"} {
		if strings.HasSuffix(n, suf) {
			return true
		}
	}
	for _, tok := range []string{"lldp", "cdp", "ospf", "isis"} {
		if strings.Contains(n, tok) {
			return true
		}
	}
	return false
}

func partitionLegacyModules(mods []string) ModuleTiers {
	var hot, cold, topo []string
	seen := map[string]struct{}{}
	add := func(bucket *[]string, m string) {
		if m == "" {
			return
		}
		if _, ok := seen[m]; ok {
			return
		}
		seen[m] = struct{}{}
		*bucket = append(*bucket, m)
	}
	for _, m := range mods {
		m = strings.TrimSpace(m)
		switch {
		case m == "if_mib" || m == "if32_mib":
			add(&hot, m)
			if m == "if_mib" {
				add(&cold, "if_mib_meta")
			}
			if m == "if32_mib" {
				add(&cold, "if32_mib_meta")
			}
		case m == "if_mib_meta" || m == "if32_mib_meta":
			add(&cold, m)
		case isTopologyModule(m):
			add(&topo, m)
		default:
			add(&cold, m)
		}
	}
	return ModuleTiers{Hot: hot, Cold: cold, Topology: topo}
}

func (f Fingerprinter) MatchTiers(labels map[string]string) ModuleTiers {
	t, _ := f.MatchTiersResult(labels)
	return t
}

// MatchTiersResult is MatchTiers plus whether a matcher hit (known) or the
// default chain was used (unknown — typically device_base + if_mib).
func (f Fingerprinter) MatchTiersResult(labels map[string]string) (ModuleTiers, string) {
	for _, m := range f.Matchers {
		if !matcherHit(m, labels) {
			continue
		}
		if m.hasTier() {
			return ModuleTiers{
				Hot:      uniqueModules(m.ModulesHot),
				Cold:     uniqueModules(m.ModulesCold),
				Topology: uniqueModules(m.ModulesTopology),
			}, FingerprintKnown
		}
		return partitionLegacyModules(m.Modules), FingerprintKnown
	}
	if len(f.DefaultModulesHot)+len(f.DefaultModulesCold)+len(f.DefaultModulesTopology) > 0 {
		return ModuleTiers{
			Hot:      uniqueModules(f.DefaultModulesHot),
			Cold:     uniqueModules(f.DefaultModulesCold),
			Topology: uniqueModules(f.DefaultModulesTopology),
		}, FingerprintUnknown
	}
	return partitionLegacyModules(f.DefaultModules), FingerprintUnknown
}

// Match returns the legacy combined module list (hot+cold+topology).
func (f Fingerprinter) Match(labels map[string]string) []string {
	t := f.MatchTiers(labels)
	out := append([]string{}, t.Hot...)
	out = append(out, t.Cold...)
	out = append(out, t.Topology...)
	return uniqueModules(out)
}

// filterTiersToKnown drops fingerprinter module names that are not in snmp.yml.
// A nil or empty catalog means "do not filter" (tests / auths-only files).
func filterTiersToKnown(t ModuleTiers, known map[string]struct{}) (ModuleTiers, []string) {
	if len(known) == 0 {
		return t, nil
	}
	var dropped []string
	keep := func(in []string) []string {
		var out []string
		for _, m := range in {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if _, ok := known[m]; ok {
				out = append(out, m)
				continue
			}
			dropped = append(dropped, m)
		}
		return out
	}
	return ModuleTiers{
		Hot:      keep(t.Hot),
		Cold:     keep(t.Cold),
		Topology: keep(t.Topology),
	}, uniqueModules(dropped)
}

func uniqueModules(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, m := range in {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}
