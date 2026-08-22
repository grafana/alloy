package catalog

import (
	"reflect"
	"strings"
	"testing"
)

func TestInventory(t *testing.T) {
	entries := Entries()
	if got, want := len(entries), 182; got != want {
		t.Fatalf("Entries() returned %d packages; want %d", got, want)
	}
	var aws []string
	for _, entry := range entries {
		if entry.ImportPath == "github.com/grafana/alloy/internal/component/discovery/aws" {
			aws = entry.Components
			break
		}
	}
	wantAWS := []string{"discovery.aws", "discovery.ec2", "discovery.lightsail"}
	if !reflect.DeepEqual(aws, wantAWS) {
		t.Fatalf("AWS components = %#v; want %#v", aws, wantAWS)
	}
}

func TestAccessorsReturnCopies(t *testing.T) {
	entries := Entries()
	originalPath := entries[0].ImportPath
	originalName := entries[0].Components[0]
	entries[0].ImportPath = "changed"
	entries[0].Components[0] = "changed"
	if got := Entries()[0]; got.ImportPath != originalPath || got.Components[0] != originalName {
		t.Fatalf("mutating Entries() result changed the catalog: %#v", got)
	}
}

func TestTagForName(t *testing.T) {
	if got, ok := TagForName("discovery.aws"); !ok || got != "alloy_component_discovery_aws" {
		t.Fatalf("TagForName(discovery.aws) = %q, %v", got, ok)
	}
	if got, ok := TagForName("does.not_exist"); ok || got != "" {
		t.Fatalf("TagForName(unknown) = %q, %v; want empty, false", got, ok)
	}
}

func TestResolveExact(t *testing.T) {
	names := []string{"prometheus.scrape", "discovery.aws", "loki.source.file"}
	got, err := ResolveExact(names)
	if err != nil {
		t.Fatalf("ResolveExact() returned error: %v", err)
	}
	want := []string{
		"alloy_component_discovery_aws",
		"alloy_component_loki_source_file",
		"alloy_component_prometheus_scrape",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveExact() = %#v; want %#v", got, want)
	}
	if !reflect.DeepEqual(names, []string{"prometheus.scrape", "discovery.aws", "loki.source.file"}) {
		t.Fatalf("ResolveExact() mutated its input: %#v", names)
	}

	empty, err := ResolveExact(nil)
	if err != nil {
		t.Fatalf("ResolveExact(nil) returned error: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("ResolveExact(nil) = %#v; want a non-nil empty slice", empty)
	}
}

func TestResolveExactErrorsAreDeterministic(t *testing.T) {
	left, leftErr := ResolveExact([]string{"z.unknown", "discovery.aws", "a.unknown"})
	right, rightErr := ResolveExact([]string{"a.unknown", "z.unknown", "discovery.aws"})
	if left != nil || right != nil {
		t.Fatalf("invalid selections returned tags: %#v, %#v", left, right)
	}
	if leftErr == nil || rightErr == nil {
		t.Fatalf("invalid selections returned errors %v and %v", leftErr, rightErr)
	}
	if leftErr.Error() != rightErr.Error() {
		t.Fatalf("errors differ by input order: %q != %q", leftErr, rightErr)
	}
	if got := leftErr.Error(); !strings.Contains(got, `"a.unknown", "z.unknown"`) {
		t.Fatalf("unknown-name error is not sorted: %q", got)
	}

	_, err := ResolveExact([]string{"prometheus.scrape", "discovery.aws", "prometheus.scrape", "discovery.aws"})
	if err == nil || !strings.Contains(err.Error(), `"discovery.aws", "prometheus.scrape"`) {
		t.Fatalf("duplicate-name error is not sorted: %v", err)
	}
}

func TestValidateDocumentRejectsInvalidCatalogs(t *testing.T) {
	valid := func() document {
		return document{
			Version: catalogVersion,
			Packages: []jsonEntry{
				{ImportPath: componentImportPrefix + "alpha", Components: []string{"alpha.one"}},
				{ImportPath: componentImportPrefix + "beta", Components: []string{"beta.two"}},
			},
		}
	}

	tests := []struct {
		name     string
		mutate   func(*document)
		contains string
	}{
		{
			name: "unsupported version",
			mutate: func(doc *document) {
				doc.Version++
			},
			contains: "unsupported version",
		},
		{
			name: "unsorted packages",
			mutate: func(doc *document) {
				doc.Packages[0], doc.Packages[1] = doc.Packages[1], doc.Packages[0]
			},
			contains: "not strictly sorted",
		},
		{
			name: "duplicate runtime name",
			mutate: func(doc *document) {
				doc.Packages[1].Components = []string{"alpha.one"}
			},
			contains: "registered by both",
		},
		{
			name: "build tag collision",
			mutate: func(doc *document) {
				doc.Packages[0].Components = []string{"alpha.beta_gamma"}
				doc.Packages[1].Components = []string{"alpha_beta.gamma"}
			},
			contains: "build tag",
		},
		{
			name: "generated filename collision",
			mutate: func(doc *document) {
				doc.Packages[0].ImportPath = componentImportPrefix + "alpha/one"
				doc.Packages[1].ImportPath = componentImportPrefix + "alpha_one"
			},
			contains: "generated filename",
		},
		{
			name: "invalid component name",
			mutate: func(doc *document) {
				doc.Packages[0].Components = []string{"alpha-invalid"}
			},
			contains: "invalid component name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := valid()
			tc.mutate(&doc)
			err := validateDocument(doc)
			if err == nil {
				t.Fatal("validateDocument() succeeded; want an error")
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("error %q does not contain %q", err, tc.contains)
			}
		})
	}
}
