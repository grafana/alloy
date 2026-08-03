package subprocess

import (
	"sync"
	"time"
)

const MaxRestarts = 10

type Handle struct {
	mu sync.Mutex

	configPath  string
	configClose func()
	port        int
	addr        string
	profilePort int
	healthAddr  string
	pid         int
	ready       bool

	restartCount int
	backoff      time.Duration
}

func New() *Handle {
	return &Handle{backoff: time.Second}
}

func (h *Handle) SetListen(port int, addr string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.port = port
	h.addr = addr
}

func (h *Handle) SetProfilePort(port int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.profilePort = port
}

func (h *Handle) SetHealthAddr(addr string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.healthAddr = addr
}

func (h *Handle) SetConfig(path string, closeFn func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.configPath = path
	h.configClose = closeFn
}

func (h *Handle) SetPid(pid int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pid = pid
}

func (h *Handle) ConfigPath() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.configPath
}

func (h *Handle) ProfilePort() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.profilePort
}

func (h *Handle) Pid() (int, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pid == 0 {
		return 0, false
	}

	return h.pid, true
}

func (h *Handle) Port() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.port
}

func (h *Handle) ProxyTarget() (addr string, profilePort int, ready bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.addr, h.profilePort, h.ready
}

func (h *Handle) HealthAddr() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.healthAddr
}

func (h *Handle) SetReady(ready bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.ready = ready
}

func (h *Handle) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.configClose != nil {
		h.configClose()
		h.configClose = nil
	}

	h.configPath = ""
	h.profilePort = 0
	h.healthAddr = ""
	h.pid = 0
	h.ready = false
}

func (h *Handle) RecordStart() (prior int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	prior = h.restartCount
	h.restartCount++

	return prior
}

func (h *Handle) ResetRestartTracking() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.restartCount = 0
	h.backoff = time.Second
}

func (h *Handle) ResetBackoffIfElevated() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.backoff <= time.Second {
		return false
	}

	h.backoff = time.Second
	h.restartCount = 0

	return true
}

func (h *Handle) NextBackoff() (backoff time.Duration, count int, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.restartCount >= MaxRestarts {
		return 0, h.restartCount, false
	}

	backoff = h.backoff
	h.backoff = min(h.backoff*2, 30*time.Second)

	return backoff, h.restartCount, true
}
