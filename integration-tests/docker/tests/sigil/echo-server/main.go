//go:build ignore

package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
)

type recorded struct {
	Body        string            `json:"body"`
	ContentType string            `json:"content_type"`
	OrgID       string            `json:"org_id"`
	Auth        string            `json:"auth"`
	Headers     map[string]string `json:"headers"`
}

var (
	mu       sync.Mutex
	requests []recorded
)

func main() {
	// Response status and body are configurable so tests can exercise
	// status/body propagation back through sigil.receive. Defaults match a
	// plain accepted export with no per-generation results.
	respStatus := http.StatusAccepted
	if v := os.Getenv("RESPONSE_STATUS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			respStatus = parsed
		}
	}
	respBody := []byte(`{"results":[]}`)
	if v := os.Getenv("RESPONSE_BODY"); v != "" {
		respBody = []byte(v)
	}

	http.HandleFunc("/api/v1/generations:export", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		headers := make(map[string]string, len(r.Header))
		for k := range r.Header {
			headers[k] = r.Header.Get(k)
		}
		mu.Lock()
		requests = append(requests, recorded{
			Body:        string(body),
			ContentType: r.Header.Get("Content-Type"),
			OrgID:       r.Header.Get("X-Scope-OrgID"),
			Auth:        r.Header.Get("Authorization"),
			Headers:     headers,
		})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(respStatus)
		_, _ = w.Write(respBody)
	})

	http.HandleFunc("/requests", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(requests)
	})

	http.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = nil
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	log.Println("echo-server listening on :8888")
	log.Fatal(http.ListenAndServe(":8888", nil))
}
