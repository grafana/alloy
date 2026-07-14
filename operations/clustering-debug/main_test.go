package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeAlloy serves the three endpoints this tool uses, with Alloy-JSON debug
// info shaped like prometheus.scrape's DebugInfo (ScraperStatus/TargetStatus).
func fakeAlloy() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v0/web/peers", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"Name":"alloy-0","Addr":"10.0.0.1:12345","Self":true,"State":"participant"},
			{"Name":"alloy-1","Addr":"10.0.0.2:12345","Self":false,"State":"participant"}
		]`))
	})
	mux.HandleFunc("/api/v0/web/components", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"name":"prometheus.scrape","localID":"prometheus.scrape.foo","moduleID":"","label":"foo","health":{"state":"healthy"}},
			{"name":"prometheus.remote_write","localID":"prometheus.remote_write.default","moduleID":"","label":"default","health":{"state":"healthy"}}
		]`))
	})
	mux.HandleFunc("/api/v0/web/components/prometheus.scrape.foo", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"name":"prometheus.scrape","health":{"state":"healthy"},
			"debugInfo":[
				{"name":"target","type":"block","body":[
					{"name":"job","type":"attr","value":{"type":"string","value":"node"}},
					{"name":"url","type":"attr","value":{"type":"string","value":"http://10.0.0.5:9100/metrics"}},
					{"name":"health","type":"attr","value":{"type":"string","value":"up"}},
					{"name":"last_error","type":"attr","value":{"type":"string","value":""}},
					{"name":"last_scrape","type":"attr","value":{"type":"capsule","value":"2026-06-10T12:00:00Z"}},
					{"name":"last_scrape_duration","type":"attr","value":{"type":"capsule","value":"12.3ms"}},
					{"name":"labels","type":"attr","value":{"type":"object","value":[
						{"key":"instance","value":{"type":"string","value":"10.0.0.5:9100"}},
						{"key":"job","value":{"type":"string","value":"node"}}
					]}}
				]},
				{"name":"target","type":"block","body":[
					{"name":"job","type":"attr","value":{"type":"string","value":"node"}},
					{"name":"url","type":"attr","value":{"type":"string","value":"http://10.0.0.6:9100/metrics"}},
					{"name":"health","type":"attr","value":{"type":"string","value":"down"}},
					{"name":"last_error","type":"attr","value":{"type":"string","value":"connection refused"}}
				]}
			]
		}`))
	})
	return httptest.NewServer(mux)
}

func newClient(base string) *client {
	return &client{
		base:    base,
		prefix:  defaultAPIPrefix,
		headers: map[string]string{},
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func TestExtractAndSimplify(t *testing.T) {
	srv := fakeAlloy()
	defer srv.Close()
	c := newClient(srv.URL)

	raw, err := c.get("/components/prometheus.scrape.foo")
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	var detail compDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	targets := extractBlocks(detail.DebugInfo, "target")
	if len(targets) != 2 {
		t.Fatalf("want 2 targets, got %d", len(targets))
	}

	t0 := targets[0]
	if t0["url"] != "http://10.0.0.5:9100/metrics" {
		t.Errorf("url = %v", t0["url"])
	}
	if t0["health"] != "up" {
		t.Errorf("health = %v", t0["health"])
	}
	if t0["last_scrape_duration"] != "12.3ms" {
		t.Errorf("duration = %v", t0["last_scrape_duration"])
	}
	labels, ok := t0["labels"].(map[string]any)
	if !ok {
		t.Fatalf("labels not a map: %T", t0["labels"])
	}
	if labels["instance"] != "10.0.0.5:9100" {
		t.Errorf("labels.instance = %v", labels["instance"])
	}

	if targets[1]["health"] != "down" || targets[1]["last_error"] != "connection refused" {
		t.Errorf("target[1] = %v", targets[1])
	}
}

func TestFindSelf(t *testing.T) {
	self := findSelf([]byte(`[{"Name":"a","Self":false},{"Name":"b","Self":true}]`))
	m, ok := self.(map[string]any)
	if !ok || m["Name"] != "b" {
		t.Fatalf("findSelf = %v", self)
	}
	if findSelf([]byte(`[{"Name":"a","Self":false}]`)) != nil {
		t.Errorf("expected nil self when none is self")
	}
}

func TestSortByOrdinal(t *testing.T) {
	got := []string{"alloy-10", "alloy-2", "alloy-0", "alloy-1"}
	sortByOrdinal(got)
	want := []string{"alloy-0", "alloy-1", "alloy-2", "alloy-10"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sortByOrdinal = %v, want %v", got, want)
		}
	}
}

func TestForwardingRegex(t *testing.T) {
	cases := map[string]string{
		"Forwarding from 127.0.0.1:54321 -> 3090": "54321",
		"Forwarding from [::1]:62000 -> 12345":    "62000",
	}
	for line, want := range cases {
		m := forwardingRe.FindStringSubmatch(line)
		if m == nil || m[1] != want {
			t.Errorf("line %q -> %v, want port %s", line, m, want)
		}
	}
}

func TestComponentID(t *testing.T) {
	if got := (compInfo{LocalID: "prometheus.scrape.foo"}).id(); got != "prometheus.scrape.foo" {
		t.Errorf("id = %q", got)
	}
	if got := (compInfo{LocalID: "x", ModuleID: "mod/a"}).id(); got != "mod/a/x" {
		t.Errorf("id = %q", got)
	}
}
