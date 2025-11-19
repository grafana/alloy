// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apmint // import "go.opentelemetry.io/ebpf-profiler/interpreter/apmint"

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/xsync"
	"go.opentelemetry.io/ebpf-profiler/stringutil"
)

// sendSocket is the shared, unbound socket that we use for communication with
// all agents. It is initialized once when the first APM agent shows up and then
// never closed until HA exit.
var sendSocket xsync.Once[int]

// apmAgentSocket represents a unix socket connection to an APM agent.
type apmAgentSocket struct {
	addr unix.SockaddrUnix
}

// openAPMAgentSocket opens the APM unix socket in the given PID's root filesystem.
//
// This method never blocks.
func openAPMAgentSocket(pid libpf.PID, socketPath string) (*apmAgentSocket, error) {
	// Ensure that the socket path can't escape our root.
	socketPath = filepath.Clean(socketPath)
	if slices.Contains(strings.Split(socketPath, "/"), "..") {
		return nil, errors.New("socket path escapes root")
	}

	// Prepend root system to ensure that this also works with containerized apps.
	socketPath = path.Join("/proc", strconv.Itoa(int(pid)), "root", socketPath)

	// Read effective UID/GID of the APM agent process.
	euid, egid, err := readProcessOwner(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to determine owner of APM process: %v", err)
	}

	// Stat socket and check whether the APM agent process is allowed to access it.
	stat, err := os.Stat(socketPath)
	if err != nil {
		return nil, errors.New("failed to stat file")
	}
	unixStat, ok := stat.Sys().(*syscall.Stat_t)
	if !ok || unixStat == nil {
		return nil, errors.New("failed to get unix stat object from stat")
	}
	if stat.Mode().Type() != fs.ModeSocket {
		return nil, errors.New("file is not a socket")
	}

	userMayAccess := stat.Mode()&unix.S_IWUSR == unix.S_IWUSR && unixStat.Uid == euid
	groupMayAccess := stat.Mode()&unix.S_IWGRP == unix.S_IWGRP && unixStat.Gid == egid
	anyoneMayAccess := stat.Mode()&unix.S_IWOTH == unix.S_IWOTH
	if euid != 0 && !anyoneMayAccess && !userMayAccess && !groupMayAccess {
		return nil, errors.New("APM process does not have perms to open socket")
	}

	return &apmAgentSocket{addr: unix.SockaddrUnix{Name: socketPath}}, nil
}

// SendMessage tries sending the given datagram to the APM agent.
//
// This function intentionally never blocks. If the agent's receive buffer is
// full or the socket was closed, an error is returned and the message is
// discarded.
func (s *apmAgentSocket) SendMessage(msg []byte) error {
	fd, err := sendSocket.GetOrInit(func() (int, error) {
		return unix.Socket(unix.AF_UNIX, unix.SOCK_DGRAM, 0)
	})
	if err != nil {
		return fmt.Errorf("failed to create global send socket: %v", err)
	}

	return unix.Sendto(*fd, msg, unix.MSG_DONTWAIT, &s.addr)
}

// traceCorrMsg represents a trace correlation socket message.
//
// https://github.com/elastic/apm/blob/bd5fa9c1/specs/agents/universal-profiling-integration.md#cpu-profiler-trace-correlation-message
//
//nolint:lll
type traceCorrMsg struct {
	MessageType      uint16
	MinorVersion     uint16
	APMTraceID       libpf.APMTraceID
	APMTransactionID libpf.APMTransactionID
	StackTraceID     libpf.TraceHash
	Count            uint16
}

func (m *traceCorrMsg) Serialize() []byte {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, m.MessageType)
	_ = binary.Write(&buf, binary.LittleEndian, m.MinorVersion)
	_, _ = buf.Write(m.APMTraceID[:])
	_, _ = buf.Write(m.APMTransactionID[:])
	_, _ = buf.Write(m.StackTraceID.Bytes())
	_ = binary.Write(&buf, binary.LittleEndian, m.Count)
	return buf.Bytes()
}

// readProcessOwner reads the effective UID and GID of the target process.
func readProcessOwner(pid libpf.PID) (euid, egid uint32, err error) {
	statusFd, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open process status: %v", err)
	}
	defer statusFd.Close()

	scanner := bufio.NewScanner(statusFd)
	found := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Uid:") {
			euid, err = parseUIDGIDLine(line)
			if err != nil {
				return 0, 0, err
			}
			found++
		} else if strings.HasPrefix(line, "Gid:") {
			egid, err = parseUIDGIDLine(line)
			if err != nil {
				return 0, 0, err
			}
			found++
		}
	}
	if scanner.Err() != nil {
		return 0, 0, fmt.Errorf("failed to read process status: %v", err)
	}

	if found != 2 {
		return 0, 0, errors.New("either euid or egid are missing")
	}

	return euid, egid, nil
}

// parseUIDGIDLine parses the "Uid:" and "Gid:" lines in /proc/$/status.
func parseUIDGIDLine(line string) (uint32, error) {
	var fields [5]string
	if stringutil.FieldsN(line, fields[:]) != 5 {
		return 0, fmt.Errorf("unexpedted `Uid` line layout: %s", line)
	}

	// Fields: real, effective, saved, FS UID
	eid, err := strconv.Atoi(fields[2])
	if err != nil {
		return 0, fmt.Errorf("failed to parse uid/gid int: %v", err)
	}

	return uint32(eid), nil
}
