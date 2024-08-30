package secretfilter

import (
	"context"
	"crypto/sha1"
	"embed"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

//go:embed gitleaks.toml
var embedFs embed.FS

type AllowRule struct {
	Regex  *regexp.Regexp
	Source string
}

type Rule struct {
	name        string
	regex       *regexp.Regexp
	secretGroup int
	description string
	allowlist   []AllowRule
}

func init() {
	component.Register(component.Registration{
		Name:      "loki.secretfilter",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the secretfilter
// component.
type Arguments struct {
	ForwardTo      []loki.LogsReceiver `alloy:"forward_to,attr"`
	GitleaksConfig string              `alloy:"gitleaks_config,attr,optional"` // Path to the custom gitleaks.toml file. If empty, the embedded one is used
	Types          []string            `alloy:"types,attr,optional"`           // Types of secret to look for (e.g. "aws", "gcp", ...). If empty, all types are included
	RedactWith     string              `alloy:"redact_with,attr,optional"`     // Redact the secret with this string. Use $SECRET_NAME and $SECRET_HASH to include the secret name and hash
	ExcludeGeneric bool                `alloy:"exclude_generic,attr,optional"` // Exclude the generic API key rule (default: false)
	AllowList      []string            `alloy:"allowlist,attr,optional"`       // List of regexes to allowlist (on top of what's in the Gitleaks config)
}

// Exports holds the values exported by the loki.secretfilter component.
type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

// DefaultArguments defines the default settings for log scraping.
var DefaultArguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

var (
	_ component.Component = (*Component)(nil)
)

// Component implements the loki.secretfilter component.
type Component struct {
	opts component.Options

	mut       sync.RWMutex
	args      Arguments
	receiver  loki.LogsReceiver
	fanout    []loki.LogsReceiver
	Rules     []Rule
	AllowList []AllowRule
}

// Non-exhaustive representation. See https://github.com/gitleaks/gitleaks/blob/master/config/config.go
type GitLeaksConfig struct {
	AllowList struct {
		Description string
		Paths       []string
		Regexes     []string
	}
	Rules []struct {
		ID          string
		Description string
		Regex       string
		Keywords    []string
		SecretGroup int

		Allowlist struct {
			StopWords []string
			Regexes   []string
		}
	}
}

// New creates a new loki.secretfilter component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:     o,
		receiver: loki.NewLogsReceiver(),
	}

	// Parse GitLeaks configuration
	var gitleaksCfg GitLeaksConfig
	if args.GitleaksConfig == "" {
		// If no config file is explicitely provided, use the embedded one
		_, err := toml.DecodeFS(embedFs, "gitleaks.toml", &gitleaksCfg)
		if err != nil {
			return nil, err
		}
	} else {
		// If a config file is provided, use that
		_, err := toml.DecodeFile(args.GitleaksConfig, &gitleaksCfg)
		if err != nil {
			return nil, err
		}
	}

	var ruleGenericApiKey *Rule = nil

	// Compile regexes
	for _, rule := range gitleaksCfg.Rules {
		// If specific secret types are provided, only include rules that match the types
		if args.Types != nil && len(args.Types) > 0 {
			var found bool
			for _, t := range args.Types {
				if strings.HasPrefix(strings.ToLower(rule.ID), strings.ToLower(t)) {
					found = true
					continue
				}
			}
			if !found {
				// Skip that rule if it doesn't match any of the secret types in the config
				continue
			}
		}
		re, err := regexp.Compile(rule.Regex)
		if err != nil {
			level.Error(o.Logger).Log("msg", "error compiling regex", "error", err)
			return nil, err
		}

		// Compile rule-specific allowlist regexes
		var allowlist []AllowRule
		for _, r := range rule.Allowlist.Regexes {
			re, err := regexp.Compile(r)
			if err != nil {
				level.Error(o.Logger).Log("msg", "error compiling allowlist regex", "error", err)
				return nil, err
			}
			allowlist = append(allowlist, AllowRule{Regex: re, Source: fmt.Sprintf("rule %s", rule.ID)})
		}

		newRule := Rule{
			name:        rule.ID,
			regex:       re,
			secretGroup: rule.SecretGroup,
			description: rule.Description,
			allowlist:   allowlist,
		}

		// We treat the generic API key rule separately as we want to add it in last position
		// to the list of rules (so that is has the lowest priority)
		if strings.ToLower(rule.ID) == "generic-api-key" {
			ruleGenericApiKey = &newRule
		} else {
			c.Rules = append(c.Rules, newRule)
		}
	}

	// Compiling global allowlist regexes
	// From the Gitleaks config
	for _, r := range gitleaksCfg.AllowList.Regexes {
		re, err := regexp.Compile(r)
		if err != nil {
			level.Error(o.Logger).Log("msg", "error compiling allowlist regex", "error", err)
			return nil, err
		}
		c.AllowList = append(c.AllowList, AllowRule{Regex: re, Source: "gitleaks config"})
	}
	// From the arguments
	for _, r := range args.AllowList {
		re, err := regexp.Compile(r)
		if err != nil {
			level.Error(o.Logger).Log("msg", "error compiling allowlist regex", "error", err)
			return nil, err
		}
		c.AllowList = append(c.AllowList, AllowRule{Regex: re, Source: "alloy config"})
	}

	// Add the generic API key rule last if needed
	if ruleGenericApiKey != nil && !args.ExcludeGeneric {
		c.Rules = append(c.Rules, *ruleGenericApiKey)
	}

	level.Info(c.opts.Logger).Log("Compiled regexes for secret detection", len(c.Rules))

	// Call to Update() once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	// Immediately export the receiver which remains the same for the component
	// lifetime.
	o.OnStateChange(Exports{Receiver: c.receiver})

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.receiver.Chan():
			for _, r := range c.Rules {
				// To find the secret within the text captured by the regex (and avoid being too greedy), we can use the 'secretGroup' field in the gitleaks.toml file.
				// But it's rare for regexes to have this field set, so we can use a simple heuristic in other cases.
				//
				// There seems to be two kinds of regexes in the gitleaks.toml file
				// 1. Regexes that only match the secret (with no submatches). E.g. (?:A3T[A-Z0-9]|AKIA|ASIA|ABIA|ACCA)[A-Z0-9]{16}
				// 2. Regexes that match the secret and some context (or delimiters) and have one submatch (the secret itself). E.g. (?i)\b(AIza[0-9A-Za-z\\-_]{35})(?:['|\"|\n|\r|\s|\x60|;]|$)
				//
				// For the first case, we can replace the entire match with the redaction string.
				// For the second case, we can replace the first submatch with the redaction string (to avoid redacting something else than the secret such as delimiters).
				for _, occ := range r.regex.FindAllStringSubmatch(entry.Line, -1) {
					// By default, the secret is the full match group
					secret := occ[0]

					// If a secretGroup is provided, use that instead
					if r.secretGroup > 0 && len(occ) > r.secretGroup {
						secret = occ[r.secretGroup]
					} else {
						// If not and there are two submatches, the first one is the secret
						if len(occ) == 2 {
							secret = occ[1]
						}
					}

					// Check if the secret is in the allowlist
					var allowRule *AllowRule = nil
					// First check the rule-specific allowlist
					for _, a := range r.allowlist {
						if a.Regex.MatchString(secret) {
							allowRule = &a
							break
						}
					}
					// Then check the global allowlist
					if allowRule == nil {
						for _, a := range c.AllowList {
							if a.Regex.MatchString(secret) {
								allowRule = &a
								break
							}
						}
					}
					// If allowed, skip redaction
					if allowRule != nil {
						level.Info(c.opts.Logger).Log("msg", "secret in allowlist", "rule", r.name, "source", allowRule.Source)
						continue
					}

					var redactWith = "<REDACTED-SECRET:" + r.name + ">"
					if c.args.RedactWith != "" {
						redactWith = c.args.RedactWith
						redactWith = strings.ReplaceAll(redactWith, "$SECRET_NAME", r.name)
						redactWith = strings.ReplaceAll(redactWith, "$SECRET_HASH", hashSecret(secret))
					}

					entry.Line = strings.ReplaceAll(entry.Line, secret, redactWith)
				}
			}

			for _, f := range c.fanout {
				select {
				case <-ctx.Done():
					return nil
				case f.Chan() <- entry:
				}
			}
		}
	}
}

func hashSecret(secret string) string {
	hasher := sha1.New()
	hasher.Write([]byte(secret))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()
	c.args = newArgs

	c.fanout = newArgs.ForwardTo

	return nil
}
