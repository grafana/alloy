// logpullmock is a minimal mock of the Cloudflare Logpull API for integration testing.
// It serves NDJSON log entries on the /client/v4/zones/{zoneID}/logs/received endpoint.
package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/client/v4/zones/", handleLogpull)

	log.Println("logpullmock listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func handleLogpull(w http.ResponseWriter, r *http.Request) {
	// Expected path: /client/v4/zones/{zoneID}/logs/received
	if !strings.HasSuffix(r.URL.Path, "/logs/received") {
		http.NotFound(w, r)
		return
	}

	log.Printf("logpull request: %s %s", r.Method, r.URL.String())

	now := time.Now()
	entries := []struct {
		ClientIP            string
		ClientRequestHost   string
		ClientRequestMethod string
		ClientRequestURI    string
		EdgeResponseStatus  int
		EdgeResponseBytes   int
		EdgeRequestHost     string
		RayID               string
	}{
		{"192.168.0.1", "example.com", "GET", "/api/v1/health", 200, 1024, "example.com", "test-ray-001"},
		{"10.0.0.2", "example.com", "POST", "/api/v1/data", 201, 2048, "example.com", "test-ray-002"},
		{"172.16.0.3", "example.com", "GET", "/api/v1/status", 200, 512, "example.com", "test-ray-003"},
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)

	for i, e := range entries {
		ts := now.Add(-90*time.Second + time.Duration(i)*time.Second).UnixNano()
		line := fmt.Sprintf(
			`{"EdgeStartTimestamp":%d,"EdgeEndTimestamp":%d,"ClientIP":"%s","ClientRequestHost":"%s","ClientRequestMethod":"%s","ClientRequestURI":"%s","EdgeResponseBytes":%d,"EdgeRequestHost":"%s","EdgeResponseStatus":%d,"RayID":"%s"}`,
			ts, ts+int64(time.Millisecond),
			e.ClientIP, e.ClientRequestHost, e.ClientRequestMethod, e.ClientRequestURI,
			e.EdgeResponseBytes, e.EdgeRequestHost, e.EdgeResponseStatus, e.RayID,
		)
		fmt.Fprintln(w, line)
	}
}
