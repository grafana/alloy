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
// requests from in-memory container state. A logs request with follow stays
// open while the container runs and streams lines as they are logged.
type Server struct {
	URL string

	done chan struct{}

	mu         sync.Mutex
	containers map[string]*fakeContainer
}

type fakeContainer struct {
	running    bool
	finishedAt time.Time
	lines      []line
	// changed is closed and replaced whenever the container changes.
	changed chan struct{}
}

type line struct {
	ts   time.Time
	text string
}

// NewServer starts a fake Docker daemon and closes it on test cleanup.
func NewServer(t testing.TB) *Server {
	s := &Server{
		done:       make(chan struct{}),
		containers: make(map[string]*fakeContainer),
	}

	mux := http.NewServeMux()
	for _, prefix := range []string{"", "/{version}"} {
		mux.HandleFunc("GET "+prefix+"/_ping", func(http.ResponseWriter, *http.Request) {})
		mux.HandleFunc("HEAD "+prefix+"/_ping", func(http.ResponseWriter, *http.Request) {})
		mux.HandleFunc("GET "+prefix+"/containers/{id}/json", s.inspect)
		mux.HandleFunc("GET "+prefix+"/containers/{id}/logs", s.logs)
	}

	srv := httptest.NewServer(mux)
	s.URL = srv.URL
	t.Cleanup(func() {
		close(s.done)
		srv.Close()
	})
	return s
}

// AddContainer adds a running container without any logs.
func (s *Server) AddContainer(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.containers[id] = &fakeContainer{running: true, changed: make(chan struct{})}
}

// Log appends a stdout line with timestamp ts to the container's logs.
func (s *Server) Log(id string, ts time.Time, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.containers[id]
	c.lines = append(c.lines, line{ts: ts, text: text})
	c.notify()
}

// Start marks the container as running.
func (s *Server) Start(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.containers[id]
	c.running = true
	c.notify()
}

// Stop marks the container as exited at the given time and ends its followed
// logs requests.
func (s *Server) Stop(id string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.containers[id]
	c.running = false
	c.finishedAt = at
	c.notify()
}

// Remove deletes the container and ends its followed logs requests.
func (s *Server) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.containers[id].notify()
	delete(s.containers, id)
}

func (c *fakeContainer) notify() {
	close(c.changed)
	c.changed = make(chan struct{})
}

func (s *Server) inspect(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.Lock()
	c, ok := s.containers[id]
	var state container.State
	if ok {
		state = container.State{
			Running:    c.running,
			FinishedAt: c.finishedAt.Format(time.RFC3339Nano),
		}
	}
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(container.InspectResponse{
		ID:     id,
		State:  &state,
		Config: &container.Config{},
	})
}

func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	query := r.URL.Query()
	follow := query.Get("follow") == "1" || query.Get("follow") == "true"

	since, err := parseSince(query.Get("since"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	_, ok := s.containers[id]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.WriteHeader(http.StatusOK)
	flusher := w.(http.Flusher)
	flusher.Flush()

	var next int
	for {
		s.mu.Lock()
		c, ok := s.containers[id]
		if !ok {
			s.mu.Unlock()
			return
		}
		lines := append([]line(nil), c.lines[next:]...)
		next = len(c.lines)
		running, changed := c.running, c.changed
		s.mu.Unlock()

		for _, l := range lines {
			// Docker includes lines at exactly the since time, and stops checking
			// since after the first line that passes because timestamps may not
			// be in order.
			if !since.IsZero() {
				if l.ts.Before(since) {
					continue
				}
				since = time.Time{}
			}
			writeFrame(w, []byte(l.ts.Format(time.RFC3339Nano)+" "+l.text+"\n"))
		}
		flusher.Flush()

		if !follow || !running {
			return
		}

		select {
		case <-changed:
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		}
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
