package sdlc

import (
	"cmp"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Bound extracted values; annotations can hold up to 256 KiB.
	maxLabelValueLength = 2048
	// The full applied manifest, including Secret data. Never extracted.
	lastAppliedAnnotation = "kubectl.kubernetes.io/last-applied-configuration"
)

// ExtractArguments selects metadata fields, labels and annotations of the
// changed object to attach to its events. Rules follow
// otelcol.processor.k8sattributes.
type ExtractArguments struct {
	Fields      []ExtractFieldRule `alloy:"field,block,optional"`
	Labels      []ExtractRule      `alloy:"label,block,optional"`
	Annotations []ExtractRule      `alloy:"annotation,block,optional"`
}

// ExtractRule selects one key, or every key matching a regular expression.
type ExtractRule struct {
	Key      string `alloy:"key,attr,optional"`
	KeyRegex string `alloy:"key_regex,attr,optional"`
	TagName  string `alloy:"tag_name,attr,optional"`
}

// ExtractFieldRule selects one metadata field: name or namespace.
type ExtractFieldRule struct {
	Key     string `alloy:"key,attr"`
	TagName string `alloy:"tag_name,attr,optional"`
}

// Validate implements syntax.Validator.
func (args *ExtractArguments) Validate() error {
	_, err := newExtractor(*args)
	return err
}

type extractRule struct {
	key, tagName string
	keyRegex     *regexp.Regexp
	// expand is set when tagName refers to keyRegex capture groups.
	expand bool
}

// extractor applies extraction rules. The zero value extracts nothing.
type extractor struct {
	fieldRules, labelRules, annotationRules []extractRule
}

func newExtractor(args ExtractArguments) (extractor, error) {
	var out extractor
	compile := func(block string, rules []ExtractRule, names map[string]bool) ([]extractRule, error) {
		compiled := make([]extractRule, 0, len(rules))
		for _, rule := range rules {
			if (rule.Key == "") == (rule.KeyRegex == "") {
				return nil, fmt.Errorf("%s: exactly one of key or key_regex must be set", block)
			}
			r := extractRule{key: rule.Key, tagName: rule.TagName}
			if rule.KeyRegex != "" {
				re, err := regexp.Compile("^(?:" + rule.KeyRegex + ")$")
				if err != nil {
					return nil, fmt.Errorf("%s: invalid key_regex: %w", block, err)
				}
				if rule.TagName != "" && re.NumSubexp() == 0 {
					return nil, fmt.Errorf("%s: tag_name with key_regex requires a capture group", block)
				}
				r.keyRegex, r.expand = re, rule.TagName != ""
			} else if strings.Contains(rule.TagName, "$") {
				return nil, fmt.Errorf("%s: tag_name references capture groups, which require key_regex", block)
			}
			if rule.TagName != "" && !r.expand {
				if names[rule.TagName] {
					return nil, fmt.Errorf("%s: duplicate tag_name %q", block, rule.TagName)
				}
				names[rule.TagName] = true
			}
			compiled = append(compiled, r)
		}
		return compiled, nil
	}
	fields := make([]ExtractRule, 0, len(args.Fields))
	for _, field := range args.Fields {
		if field.Key != "name" && field.Key != "namespace" {
			return extractor{}, fmt.Errorf(`field: key must be "name" or "namespace"`)
		}
		if strings.Contains(field.TagName, "$") {
			return extractor{}, fmt.Errorf("field: tag_name can't reference capture groups")
		}
		fields = append(fields, ExtractRule{Key: field.Key, TagName: field.TagName})
	}
	var err error
	// Field tag names are checked apart, so labels and annotations can
	// override fields as fallbacks.
	if out.fieldRules, err = compile("field", fields, map[string]bool{}); err != nil {
		return extractor{}, err
	}
	names := map[string]bool{}
	if out.labelRules, err = compile("label", args.Labels, names); err != nil {
		return extractor{}, err
	}
	if out.annotationRules, err = compile("annotation", args.Annotations, names); err != nil {
		return extractor{}, err
	}
	return out, nil
}

func (e extractor) empty() bool {
	return len(e.fieldRules) == 0 && len(e.labelRules) == 0 && len(e.annotationRules) == 0
}

// labels returns the extracted labels of m by name. Fields are applied before
// labels, labels before annotations, and later rules overwrite earlier ones.
func (e extractor) labels(m metav1.Object) map[string]string {
	if e.empty() {
		return nil
	}
	out := map[string]string{}
	if len(e.fieldRules) > 0 {
		fields := map[string]string{}
		for key, value := range map[string]string{"name": m.GetName(), "namespace": m.GetNamespace()} {
			if value != "" {
				fields[key] = value
			}
		}
		apply(e.fieldRules, fields, out)
	}
	apply(e.labelRules, m.GetLabels(), out)
	annotations := m.GetAnnotations()
	if _, ok := annotations[lastAppliedAnnotation]; ok {
		annotations = maps.Clone(annotations)
		delete(annotations, lastAppliedAnnotation)
	}
	apply(e.annotationRules, annotations, out)
	return out
}

// keepAnnotation reports whether an annotation value must stay readable in
// informer caches that otherwise hold hashes.
func (e extractor) keepAnnotation(key string) bool {
	if key == lastAppliedAnnotation {
		return false
	}
	return slices.ContainsFunc(e.annotationRules, func(r extractRule) bool {
		if r.keyRegex != nil {
			return r.keyRegex.MatchString(key)
		}
		return r.key == key
	})
}

func apply(rules []extractRule, values, out map[string]string) {
	if len(values) == 0 {
		return
	}
	// Sorted so overlapping regex matches resolve the same way every time.
	keys := slices.Sorted(maps.Keys(values))
	for _, r := range rules {
		if r.keyRegex == nil {
			if v, ok := values[r.key]; ok {
				out[cmp.Or(r.tagName, r.key)] = truncate(v, maxLabelValueLength)
			}
			continue
		}
		for _, k := range keys {
			v := values[k]
			if v == "" || !r.keyRegex.MatchString(k) {
				continue
			}
			name := k
			if r.expand {
				name = string(r.keyRegex.ExpandString(nil, r.tagName, k, r.keyRegex.FindStringSubmatchIndex(k)))
			}
			if name != "" {
				out[name] = truncate(v, maxLabelValueLength)
			}
		}
	}
}
