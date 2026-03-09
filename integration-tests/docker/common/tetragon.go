package common

import (
	"archive/tar"
	"bytes"
	"context"
	"debug/elf"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

// TetragonProbeInitDelay is how long Tetragon runs alone before Alloy starts,
// giving its eBPF probes time to arm before the first Alloy syscall.
const TetragonProbeInitDelay = 10 * time.Second

// TetragonContainerIDEnv is the environment variable used to pass the Tetragon
// container ID from the test runner to the test process so that TestCapabilities
// can read the container logs directly.
const TetragonContainerIDEnv = "TETRAGON_CONTAINER_ID"

// AlloyContainerIDEnv is the environment variable used to pass the Alloy
// container ID from the test runner to the test process. AssertTetragonCapabilities
// uses it to filter Tetragon events to only those originating from the specific
// Alloy container under test, preventing cross-contamination from concurrently
// running tests whose alloy containers may have different capability sets.
const AlloyContainerIDEnv = "ALLOY_CONTAINER_ID"

// capabilitiesPolicy is a Tetragon TracingPolicy that records every cap_capable()
// call made by the alloy binary, capturing the user-space call stack and the
// return value (0 = granted, non-zero = denied) so the reason for each
// capability check and its outcome can be identified.
//
// cap_capable has the signature:
//
//	int cap_capable(const struct cred *cred, struct user_namespace *tns, int cap, unsigned int opts)
//
// We trace arg index 2 (the capability number) and the return value.
// Tetragon captures the user stack at the kprobe entry point (where the full
// call stack is still intact) and emits a single combined event at return.
const capabilitiesPolicy = `apiVersion: cilium.io/v1alpha1
kind: TracingPolicy
metadata:
  name: monitor-capabilities
spec:
  kprobes:
  - call: "cap_capable"
    syscall: false
    return: true
    args:
    - index: 2
      type: "int"
    returnArg:
      index: 0
      type: "int"
    selectors:
    - matchBinaries:
        - operator: "Postfix"
          values:
          - "/bin/alloy"
      matchActions:
        - action: Post
          userStackTrace: true
`

// capabilityNames maps Linux capability numbers to their CAP_* names.
var capabilityNames = map[int]string{
	0:  "CAP_CHOWN",
	1:  "CAP_DAC_OVERRIDE",
	2:  "CAP_DAC_READ_SEARCH",
	3:  "CAP_FOWNER",
	4:  "CAP_FSETID",
	5:  "CAP_KILL",
	6:  "CAP_SETGID",
	7:  "CAP_SETUID",
	8:  "CAP_SETPCAP",
	9:  "CAP_LINUX_IMMUTABLE",
	10: "CAP_NET_BIND_SERVICE",
	11: "CAP_NET_BROADCAST",
	12: "CAP_NET_ADMIN",
	13: "CAP_NET_RAW",
	14: "CAP_IPC_LOCK",
	15: "CAP_IPC_OWNER",
	16: "CAP_SYS_MODULE",
	17: "CAP_SYS_RAWIO",
	18: "CAP_SYS_CHROOT",
	19: "CAP_SYS_PTRACE",
	20: "CAP_SYS_PACCT",
	21: "CAP_SYS_ADMIN",
	22: "CAP_SYS_BOOT",
	23: "CAP_SYS_NICE",
	24: "CAP_SYS_RESOURCE",
	25: "CAP_SYS_TIME",
	26: "CAP_SYS_TTY_CONFIG",
	27: "CAP_MKNOD",
	28: "CAP_LEASE",
	29: "CAP_AUDIT_WRITE",
	30: "CAP_AUDIT_CONTROL",
	31: "CAP_SETFCAP",
	32: "CAP_MAC_OVERRIDE",
	33: "CAP_MAC_ADMIN",
	34: "CAP_SYSLOG",
	35: "CAP_WAKE_ALARM",
	36: "CAP_BLOCK_SUSPEND",
	37: "CAP_AUDIT_READ",
	38: "CAP_PERFMON",
	39: "CAP_BPF",
	40: "CAP_CHECKPOINT_RESTORE",
}

// TetragonCapabilityEvent records one capability usage event from Tetragon.
type TetragonCapabilityEvent struct {
	Comm                 string   // binary basename (e.g. "alloy")
	ContainerID          string   // Docker container ID of the process (full 64-char hex string)
	Capability           string   // e.g. "CAP_SYS_ADMIN"
	Granted              bool     // true when cap_capable returned 0 (capability was granted)
	KernelStackTrace     []string // kernel symbols, innermost first (empty when unavailable)
	UserStackTrace       []string // user-space symbols, innermost first (empty when unavailable)
	UserOffsets          []int64  // raw decimal offsets for addr2line, index-aligned with UserStackTrace
	moduleFromFirstFrame string   // full container path from the first user stack frame module field
}

// ExpectedCapabilityEvent describes a Linux capability usage that a test
// explicitly allows. It matches actual Tetragon events by capability name and,
// optionally, by requiring that certain substrings appear somewhere in the
// combined user stack trace.
type ExpectedCapabilityEvent struct {
	// Capability is the exact capability name, e.g. "CAP_FOWNER".
	Capability string
	// StackContains lists substrings that must all appear somewhere across the
	// full user stack trace of the matching event. An empty slice means only
	// the capability name is checked.
	StackContains []string
}


// StartTetragonContainer starts a Tetragon container configured to record
// capability usage and expose Prometheus metrics on port 2112.
// The capabilities TracingPolicy is injected directly into the container.
// The caller is responsible for terminating it.
//
// Set TETRAGON_LOG_LEVEL=debug or TETRAGON_LOG_LEVEL=trace in the environment
// to enable verbose Tetragon logging, which includes messages about
// /proc/<pid>/maps access and symbolization failures.
func StartTetragonContainer(ctx context.Context, image string) (testcontainers.Container, error) {
	args := []string{
		"/usr/bin/tetragon",
		"--export-filename=/dev/stdout",
		"--metrics-server=:2112",
		"--tracing-policy-dir=/etc/tetragon/tetragon.tp.d",
		// Only export events for processes whose binary path ends with "alloy".
		`--export-allowlist={"binary_regex":[".*alloy$"]}`,
		"--expose-stack-addresses",
	}
	if lvl := os.Getenv("TETRAGON_LOG_LEVEL"); lvl == "debug" || lvl == "trace" {
		args = append(args, "--log-level="+lvl)
	}

	req := testcontainers.ContainerRequest{
		Image:           image,
		AlwaysPullImage: false,
		Entrypoint:      args,
		// Bind container port 2112 to the same host port so metrics are always
		// reachable at http://localhost:2112/metrics during the test run.
		ExposedPorts: []string{"2112/tcp"},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Privileged = true
			hc.PidMode = container.PidMode("host")
			hc.CgroupnsMode = "host"
			hc.Binds = append(hc.Binds, "/sys/kernel:/sys/kernel:ro")
			hc.PortBindings = nat.PortMap{
				"2112/tcp": []nat.PortBinding{
					{HostIP: "127.0.0.1", HostPort: "2112"},
				},
			}
		},
		Files: []testcontainers.ContainerFile{
			{
				Reader:            strings.NewReader(capabilitiesPolicy),
				ContainerFilePath: "/etc/tetragon/tetragon.tp.d/capabilities.yaml",
				FileMode:          0644,
			},
		},
	}
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

// FormatTetragonLogs reads the Tetragon container's NDJSON log output and
// returns a human-readable summary of all capability events. It runs
// "tetra getevents -o compact" inside the Tetragon container for native
// formatting, then falls back to our own formatter with go tool addr2line
// for stack-frame resolution.
func FormatTetragonLogs(ctx context.Context, tetragonContainer testcontainers.Container, alloyContainer testcontainers.Container) (string, error) {
	logs, err := tetragonContainer.Logs(ctx)
	if err != nil {
		return "", fmt.Errorf("reading tetragon logs: %w", err)
	}
	defer logs.Close()

	// testcontainers' Logs() already demultiplexes the Docker stream internally.
	logBytes, err := io.ReadAll(logs)
	if err != nil {
		return "", fmt.Errorf("reading tetragon logs: %w", err)
	}
	logText := string(logBytes)

	events := ParseTetragonCapabilityLogs(logText, "")
	if len(events) == 0 {
		return "(no capability events recorded)", nil
	}

	var sb strings.Builder
	var resolved map[int64]string
	var resolveNote string
	if alloyContainer != nil {
		// Use the module path from the first stack frame as the authoritative
		// binary location (Tetragon reads it from /proc/<pid>/maps).
		binaryInContainer := "/bin/alloy"
		for _, ev := range events {
			if len(ev.moduleFromFirstFrame) > 0 {
				binaryInContainer = ev.moduleFromFirstFrame
				break
			}
		}
		binaryPath, err := extractContainerFile(ctx, alloyContainer, binaryInContainer)
		if err != nil {
			resolveNote = fmt.Sprintf("(could not extract %s for addr2line: %v)", binaryInContainer, err)
		} else {
			defer os.Remove(binaryPath)
			seen := map[int64]struct{}{}
			var offsets []int64
			for _, ev := range events {
				for _, off := range ev.UserOffsets {
					if _, ok := seen[off]; !ok {
						seen[off] = struct{}{}
						offsets = append(offsets, off)
					}
				}
			}
			var resolveErr error
			resolved, resolveErr = resolveOffsetsWithAddr2line(binaryPath, offsets)
			if resolveErr != nil {
				resolveNote = fmt.Sprintf("(addr2line error: %v)", resolveErr)
			}
		}
	}

	if resolveNote != "" {
		fmt.Fprintln(&sb, resolveNote)
	}
	for _, ev := range events {
		status := "GRANTED"
		if !ev.Granted {
			status = "DENIED"
		}
		fmt.Fprintf(&sb, "\n[%s %s] %s\n", ev.Capability, status, ev.Comm)
		if len(ev.UserOffsets) > 0 {
			fmt.Fprintf(&sb, "   User stack:\n")
			for i, off := range ev.UserOffsets {
				if name, ok := resolved[off]; ok {
					fmt.Fprintf(&sb, "      %s\n", name)
				} else {
					fmt.Fprintf(&sb, "      %s\n", ev.UserStackTrace[i])
				}
			}
		}
	}
	return sb.String(), nil
}

// extractContainerFile copies a single file from inside a container to a
// temporary file on the host and returns its path. The caller must remove it.
func extractContainerFile(ctx context.Context, c testcontainers.Container, containerPath string) (string, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}
	defer cli.Close()

	rc, _, err := cli.CopyFromContainer(ctx, c.GetContainerID(), containerPath)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	tmp, err := os.CreateTemp("", "alloy-binary-*")
	if err != nil {
		return "", err
	}

	tr := tar.NewReader(rc)
	if _, err := tr.Next(); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if _, err := io.Copy(tmp, tr); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	tmp.Close()
	return tmp.Name(), nil
}

// elfLoadBase returns the virtual address of the first PT_LOAD segment in the
// ELF binary at binaryPath. Tetragon reports stack offsets relative to the
// mapping start address (from /proc/pid/maps), which equals this value for
// non-PIE binaries (e.g. 0x400000). go tool addr2line expects the full virtual
// address, so we add this base to each Tetragon offset before calling it.
// For PIE binaries the first PT_LOAD VirtAddr is 0, so the adjustment is a no-op.
func elfLoadBase(binaryPath string) int64 {
	f, err := elf.Open(binaryPath)
	if err != nil {
		return 0
	}
	defer f.Close()
	for _, prog := range f.Progs {
		if prog.Type == elf.PT_LOAD {
			return int64(prog.Vaddr)
		}
	}
	return 0
}

// resolveOffsetsWithAddr2line runs go tool addr2line against binaryPath for the
// given offsets and returns a map from offset to "funcName (file:line)".
// Offsets that cannot be resolved are omitted from the result.
//
// go tool addr2line (Go 1.18+) takes the binary as a positional argument and
// reads hex addresses from stdin, emitting two lines per address:
//
//	function name
//	file:line
func resolveOffsetsWithAddr2line(binaryPath string, offsets []int64) (map[int64]string, error) {
	if len(offsets) == 0 {
		return nil, nil
	}

	// Tetragon offsets are relative to the binary's mapping base; addr2line
	// needs the ELF virtual address, so we add the load base.
	loadBase := elfLoadBase(binaryPath)

	cmd := exec.Command("go", "tool", "addr2line", binaryPath)

	// Write all hex addresses to stdin.
	var stdinBuf strings.Builder
	for _, off := range offsets {
		fmt.Fprintf(&stdinBuf, "0x%x\n", off+loadBase)
	}
	cmd.Stdin = strings.NewReader(stdinBuf.String())

	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		return nil, fmt.Errorf("go tool addr2line: %w\n%s", err, stderr)
	}

	// addr2line emits two lines per address: function name then file:line.
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	result := make(map[int64]string, len(offsets))
	for i, off := range offsets {
		fn := ""
		loc := ""
		if i*2 < len(lines) {
			fn = lines[i*2]
		}
		if i*2+1 < len(lines) {
			loc = lines[i*2+1]
		}
		if fn == "?" || fn == "" {
			continue
		}
		if loc != "" && loc != "?:0" {
			result[off] = fmt.Sprintf("%s  (%s)", fn, loc)
		} else {
			result[off] = fn
		}
	}
	return result, nil
}

// CollectTetragonCapabilityEvents reads Tetragon container logs and returns all
// unique capability events for processes whose binary basename matches comm.
// Pass an empty comm to collect events for all processes.
func CollectTetragonCapabilityEvents(ctx context.Context, c testcontainers.Container, comm string) ([]TetragonCapabilityEvent, error) {
	logs, err := c.Logs(ctx)
	if err != nil {
		return nil, err
	}
	defer logs.Close()

	logBytes, err := io.ReadAll(logs)
	if err != nil {
		return nil, err
	}
	return ParseTetragonCapabilityLogs(string(logBytes), comm), nil
}

// CollectTetragonCapabilityEventsFromID reads Tetragon logs from an already-running
// container identified by containerID and returns all unique capability events.
// Use this when the container was started by the test runner (e.g. in TestCapabilities).
func CollectTetragonCapabilityEventsFromID(ctx context.Context, containerID string, comm string) ([]TetragonCapabilityEvent, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	reader, err := cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// ContainerLogs returns a Docker-multiplexed stream: each chunk is prefixed
	// with an 8-byte header encoding the stream type and payload length.
	// stdcopy.StdCopy strips those headers, yielding plain text that can be
	// parsed as NDJSON.
	var buf bytes.Buffer
	if _, err := stdcopy.StdCopy(&buf, &buf, reader); err != nil {
		return nil, fmt.Errorf("demultiplexing tetragon logs: %w", err)
	}
	return ParseTetragonCapabilityLogs(buf.String(), comm), nil
}

// tetragonEvent is a minimal representation of the Tetragon NDJSON export format
// for process_kprobe events on cap_capable.
type tetragonEvent struct {
	ProcessKprobe *struct {
		Process struct {
			Binary string `json:"binary"`
			Docker string `json:"docker"` // full container ID of the process
		} `json:"process"`
		FunctionName string `json:"function_name"`
		Args         []struct {
			IntArg *int `json:"int_arg"`
		} `json:"args"`
		// Return holds the return value of cap_capable (0 = granted, non-zero = denied).
		// It is populated when the TracingPolicy has return: true + returnArg.
		Return *struct {
			IntArg *int `json:"int_arg"`
		} `json:"return"`
		KernelStackTrace []struct {
			Symbol string `json:"symbol"`
			Offset string `json:"offset"`
		} `json:"kernel_stack_trace"`
		UserStackTrace []struct {
			Symbol string `json:"symbol"`
			Offset string `json:"offset"` // decimal file offset within Module
			Module string `json:"module"` // absolute path of the binary
		} `json:"user_stack_trace"`
	} `json:"process_kprobe"`
}

// ParseTetragonCapabilityLogs extracts unique TetragonCapabilityEvents from
// Tetragon's NDJSON log output, filtered by binary basename matching comm.
// Events are deduplicated by (comm, capability) since PIDs vary between runs.
func ParseTetragonCapabilityLogs(logs string, comm string) []TetragonCapabilityEvent {
	seen := make(map[string]struct{})
	var out []TetragonCapabilityEvent

	for _, line := range strings.Split(logs, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "cap_capable") {
			continue
		}
		var ev tetragonEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil || ev.ProcessKprobe == nil {
			continue
		}
		kp := ev.ProcessKprobe
		if kp.FunctionName != "cap_capable" {
			continue
		}
		binary := filepath.Base(kp.Process.Binary)
		if comm != "" && binary != comm {
			continue
		}
		if len(kp.Args) == 0 || kp.Args[0].IntArg == nil {
			continue
		}
		capNum := *kp.Args[0].IntArg
		capName, ok := capabilityNames[capNum]
		if !ok {
			capName = fmt.Sprintf("CAP_%d", capNum)
		}

		// Determine whether the capability was granted (return value 0) or
		// denied (any non-zero return). When no return value is present in the
		// event (older Tetragon or policy without returnArg), default to true so
		// existing behaviour is preserved.
		granted := true
		if kp.Return != nil && kp.Return.IntArg != nil {
			granted = *kp.Return.IntArg == 0
		}

		key := binary + "\x00" + capName
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		var kernelStack, userStack []string
		var userOffsets []int64
		var firstModule string
		for _, frame := range kp.KernelStackTrace {
			if frame.Symbol != "" {
				kernelStack = append(kernelStack, fmt.Sprintf("%s+0x%s", frame.Symbol, frame.Offset))
			}
		}
		for _, frame := range kp.UserStackTrace {
			off, _ := strconv.ParseInt(frame.Offset, 10, 64)
			// Prefer the module whose basename matches the process binary name
			// (e.g. "alloy") over runtime linker modules like ld-linux.
			if firstModule == "" && frame.Module != "" {
				firstModule = frame.Module
			}
			if frame.Module != "" && filepath.Base(frame.Module) == binary {
				firstModule = frame.Module
			}
			if frame.Symbol != "" {
				userStack = append(userStack, fmt.Sprintf("%s (%s+0x%x)", frame.Symbol, filepath.Base(frame.Module), off))
			} else {
				userStack = append(userStack, fmt.Sprintf("0x%x (%s)", off, filepath.Base(frame.Module)))
			}
			userOffsets = append(userOffsets, off)
		}

		out = append(out, TetragonCapabilityEvent{
			Comm:                 binary,
			ContainerID:          kp.Process.Docker,
			Capability:           capName,
			Granted:              granted,
			KernelStackTrace:     kernelStack,
			UserStackTrace:       userStack,
			UserOffsets:          userOffsets,
			moduleFromFirstFrame: firstModule,
		})
	}
	return out
}

// capabilityEventMatchesExpected returns true when actual satisfies exp:
// the capability name is identical and every string in exp.StackContains
// appears somewhere across the full user stack trace of actual.
func capabilityEventMatchesExpected(actual TetragonCapabilityEvent, exp ExpectedCapabilityEvent) bool {
	if actual.Capability != exp.Capability {
		return false
	}
	fullStack := strings.Join(actual.UserStackTrace, "\n")
	for _, s := range exp.StackContains {
		if !strings.Contains(fullStack, s) {
			return false
		}
	}
	return true
}

// AssertTetragonCapabilities checks that every entry in required was observed
// at least once in the Tetragon capability events for the given process. The
// test fails if a required capability is never seen, catching regressions where
// a needed capability check is removed (e.g. after a refactor).
//
// Observed events that do not match any required entry are silently ignored —
// the function makes no assertions about unexpected capabilities.
//
// When ALLOY_CONTAINER_ID is set in the environment, only events whose Docker
// container ID matches it are considered. This is essential when tests run in
// parallel: Tetragon uses host PID mode and therefore observes every alloy
// process on the host, including those from concurrently running tests with
// different capability sets.
//
// The test is skipped automatically when no Tetragon container is configured
// (i.e. tetragon_image is not set in test.yaml).
func AssertTetragonCapabilities(t *testing.T, comm string, required []ExpectedCapabilityEvent) {
	t.Helper()

	tetragonContainerID := os.Getenv(TetragonContainerIDEnv)
	if tetragonContainerID == "" {
		t.Skip("Tetragon container not configured (tetragon_image not set in test.yaml)")
	}

	events, err := CollectTetragonCapabilityEventsFromID(context.Background(), tetragonContainerID, comm)
	require.NoError(t, err)

	// Filter to events from the specific Alloy container under test so that
	// parallel runs of other tests cannot pollute this test's results.
	alloyContainerID := os.Getenv(AlloyContainerIDEnv)
	var relevant []TetragonCapabilityEvent
	for _, ev := range events {
		if alloyContainerID != "" && ev.ContainerID != alloyContainerID {
			continue
		}
		relevant = append(relevant, ev)
	}

	for _, req := range required {
		found := false
		for _, ev := range relevant {
			if capabilityEventMatchesExpected(ev, req) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("required capability %q (stack must contain %v) was never observed — "+
				"was the code path removed or is the StackContains filter too specific?",
				req.Capability, req.StackContains)
		}
	}
}
