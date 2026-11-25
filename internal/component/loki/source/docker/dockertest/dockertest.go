// Package dockertest provides a fake Docker daemon for tests.
package dockertest

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
)

// Server is a fake Docker daemon that serves container inspect and logs
// requests from in-memory container state. Containers are reported as exited,
// and logs requests return the stored lines and end.
type Server struct {
	URL string

	mu         sync.Mutex
	containers map[string][]line
}

type line struct {
	ts   time.Time
	text string
}

// NewServer starts a fake Docker daemon and closes it on test cleanup.
func NewServer(t testing.TB) *Server {
	s := &Server{containers: make(map[string][]line)}

	mux := http.NewServeMux()
	for _, prefix := range []string{"", "/{version}"} {
		mux.HandleFunc("GET "+prefix+"/_ping", func(http.ResponseWriter, *http.Request) {})
		mux.HandleFunc("HEAD "+prefix+"/_ping", func(http.ResponseWriter, *http.Request) {})
		mux.HandleFunc("GET "+prefix+"/containers/{id}/json", s.inspect)
		mux.HandleFunc("GET "+prefix+"/containers/{id}/logs", s.logs)
	}

	srv := httptest.NewServer(mux)
	s.URL = srv.URL
	t.Cleanup(srv.Close)
	return s
}

// AddContainer adds a container without any logs.
func (s *Server) AddContainer(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.containers[id] = nil
}

// Log appends a stdout line with timestamp ts to the container's logs.
func (s *Server) Log(id string, ts time.Time, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.containers[id] = append(s.containers[id], line{ts: ts, text: text})
}

func (s *Server) inspect(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.Lock()
	_, ok := s.containers[id]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(container.InspectResponse{
		ID: id,
		State: &container.State{
			FinishedAt: time.Time{}.Format(time.RFC3339Nano),
		},
		Config: &container.Config{},
	})
}

func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	since, err := parseSince(r.URL.Query().Get("since"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	lines, ok := s.containers[id]
	lines = append([]line(nil), lines...)
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}

	for _, l := range lines {
		// Docker includes lines at exactly the since time.
		if l.ts.Before(since) {
			continue
		}
		writeFrame(w, []byte(l.ts.Format(time.RFC3339Nano)+" "+l.text+"\n"))
	}
}

// parseSince parses Docker's since format: unix seconds with optional
// fractional nanoseconds.
func parseSince(since string) (time.Time, error) {
	if since == "" {
		return time.Time{}, nil
	}

	secStr, nsecStr, _ := strings.Cut(since, ".")
	sec, err := strconv.ParseInt(secStr, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	var nsec int64
	if nsecStr != "" {
		nsec, err = strconv.ParseInt(nsecStr, 10, 64)
		if err != nil {
			return time.Time{}, err
		}
	}
	return time.Unix(sec, nsec), nil
}

// writeFrame writes p as a stdout frame in the multiplexed stream format that
// stdcopy.StdCopy reads.
func writeFrame(w http.ResponseWriter, p []byte) {
	var hdr [8]byte
	hdr[0] = byte(stdcopy.Stdout)
	binary.BigEndian.PutUint32(hdr[4:], uint32(len(p)))
	_, _ = w.Write(hdr[:])
	_, _ = w.Write(p)
}
