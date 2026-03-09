package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tetragonpb "github.com/cilium/tetragon/api/v1/tetragon"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TetragonProbeInitDelay is how long Tetragon runs alone before Alloy starts,
// giving its eBPF probes time to arm before the first Alloy syscall.
const TetragonProbeInitDelay = 10 * time.Second

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

// capabilityName returns the CAP_* name for a Linux capability number using
// the Tetragon proto's own CapabilitiesType enum, e.g. "CAP_SYS_PTRACE".
// Unknown values fall back to the proto default, e.g. "CapabilitiesType(41)".
func capabilityName(n int) string {
	return tetragonpb.CapabilitiesType(n).String()
}

// ExpectedCapabilityEvent describes a Linux capability usage that a test
// explicitly expects. It matches live Tetragon gRPC events by capability name
// and, optionally, by requiring that certain substrings appear somewhere in the
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
// capability usage and expose:
//   - Prometheus metrics on port 2112
//   - A gRPC event stream on a random host port (retrieve with TetragonGRPCAddr)
//
// The capabilities TracingPolicy is injected directly into the container.
// The caller is responsible for terminating it.
//
// Set TETRAGON_LOG_LEVEL=debug or TETRAGON_LOG_LEVEL=trace in the environment
// to enable verbose Tetragon logging.
func StartTetragonContainer(ctx context.Context, image string) (testcontainers.Container, error) {
	args := []string{
		"/usr/bin/tetragon",
		// Bind the gRPC server on all interfaces so the host test process can
		// connect via the mapped port. Default is localhost-only.
		"--server-address=0.0.0.0:54321",
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
		// Port 2112 is bound to the same host port for Prometheus metrics.
		// Port 54321 (gRPC) is mapped to a random host port to avoid conflicts
		// when tests run in parallel; use TetragonGRPCAddr to discover it.
		ExposedPorts: []string{"2112/tcp", "54321/tcp"},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Privileged = true
			hc.PidMode = container.PidMode("host")
			hc.CgroupnsMode = "host"
			hc.Binds = append(hc.Binds, "/sys/kernel:/sys/kernel:ro")
			hc.PortBindings = nat.PortMap{
				"2112/tcp": []nat.PortBinding{
					{HostIP: "127.0.0.1", HostPort: "2112"},
				},
				// 54321 is left unbound so the OS picks a free port.
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

// TetragonGRPCAddr returns the host address (host:port) of the Tetragon gRPC
// server for c. The port is the randomly assigned host port mapped from
// container port 54321.
func TetragonGRPCAddr(ctx context.Context, c testcontainers.Container) (string, error) {
	port, err := c.MappedPort(ctx, "54321/tcp")
	if err != nil {
		return "", fmt.Errorf("getting Tetragon gRPC mapped port: %w", err)
	}
	return fmt.Sprintf("localhost:%s", port.Port()), nil
}

// StartCapabilityCollection connects to the Tetragon gRPC server at grpcAddr
// and starts streaming all cap_capable events for the alloy binary in the
// background. It returns a stop function: call it to cancel the stream and
// receive a formatted report of every capability event observed, suitable for
// inclusion in the test failure output.
//
// The report format mirrors the old NDJSON-based output:
//
//	[CAP_SYS_PTRACE DENIED] alloy
//	   User stack:
//	      github.com/grafana/alloy/…discover (alloy+0x…)
//	      …
//
// A return value of 0 from cap_capable means the capability was granted;
// anything else means it was denied.
func StartCapabilityCollection(grpcAddr string) func() string {
	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan string, 1)

	go func() {
		conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			resultCh <- fmt.Sprintf("(failed to connect to Tetragon gRPC at %s: %v)", grpcAddr, err)
			return
		}
		defer conn.Close()

		client := tetragonpb.NewFineGuidanceSensorsClient(conn)
		stream, err := client.GetEvents(ctx, &tetragonpb.GetEventsRequest{})
		if err != nil {
			resultCh <- fmt.Sprintf("(failed to open Tetragon event stream: %v)", err)
			return
		}

		// seen deduplicates events by (capName, status, stack) so that
		// capabilities checked thousands of times (e.g. CAP_SYS_PTRACE by
		// discovery.process on every /proc scan) only appear once.
		seen := make(map[string]struct{})
		var sb strings.Builder
		for {
			resp, err := stream.Recv()
			if err != nil {
				break // context cancelled or server gone — normal shutdown
			}

			kp := resp.GetProcessKprobe()
			if kp == nil || kp.GetFunctionName() != "cap_capable" {
				continue
			}
			proc := kp.GetProcess()
			if proc == nil {
				continue
			}
			args := kp.GetArgs()
			if len(args) == 0 {
				continue
			}

			capName := capabilityName(int(args[0].GetIntArg()))

			granted := true
			if ret := kp.GetReturn(); ret != nil {
				granted = ret.GetIntArg() == 0
			}
			status := "GRANTED"
			if !granted {
				status = "DENIED"
			}

			var stackParts []string
			for _, frame := range kp.GetUserStackTrace() {
				if sym := frame.GetSymbol(); sym != "" {
					stackParts = append(stackParts, sym)
				}
			}

			// Skip duplicates: same capability and same outcome, regardless of
			// call site. Each (capName, status) pair is printed at most once.
			dedupKey := capName + "|" + status
			if _, dup := seen[dedupKey]; dup {
				continue
			}
			seen[dedupKey] = struct{}{}

			fmt.Fprintf(&sb, "[%s %s] %s\n", capName, status, filepath.Base(proc.GetBinary()))
			if len(stackParts) > 0 {
				sb.WriteString("   User stack:\n")
				for _, sym := range stackParts {
					fmt.Fprintf(&sb, "      %s\n", sym)
				}
			}
			sb.WriteString("\n")
		}

		if sb.Len() == 0 {
			resultCh <- "(no capability events observed)"
			return
		}
		resultCh <- sb.String()
	}()

	return func() string {
		cancel()
		return <-resultCh
	}
}

// AssertTetragonCapabilities streams live capability events from the Tetragon
// gRPC server and asserts that every entry in required is observed at least
// once for the given process. The function exits as soon as all required events
// are satisfied or the 2-minute deadline is reached.
//
// Because discovery.process scans /proc periodically, capability events for
// "alloy" keep arriving throughout the test run, so the 2-minute window is
// always sufficient even when the test calls this function at the end.
//
// The test is skipped automatically when TETRAGON_GRPC_ADDR is not set (i.e.
// tetragon_container.image is not configured in test.yaml).
//
// When ALLOY_CONTAINER_ID is set in the environment, only events whose Docker
// container ID matches it are considered. This prevents cross-contamination
// from concurrently running tests whose alloy containers may have different
// capability sets.
func AssertTetragonCapabilities(t *testing.T, comm string, required []ExpectedCapabilityEvent) {
	t.Helper()

	grpcAddr := os.Getenv(TetragonGRPCAddrEnv)
	if grpcAddr == "" {
		t.Skip("Tetragon container not configured (tetragon_container.image not set in test.yaml)")
	}

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "connecting to Tetragon gRPC at %s", grpcAddr)
	defer conn.Close()

	client := tetragonpb.NewFineGuidanceSensorsClient(conn)

	// Use a generous timeout: discovery.process scans /proc periodically so
	// capability events will keep arriving throughout the test run.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	stream, err := client.GetEvents(ctx, &tetragonpb.GetEventsRequest{})
	require.NoError(t, err, "opening Tetragon event stream")

	alloyContainerID := os.Getenv(AlloyContainerIDEnv)
	satisfied := make(map[int]bool, len(required))

	// observedEvents accumulates every capability event that passed the
	// binary/container filters. Logged via t.Log on failure (or -v) so
	// developers can see what was actually observed vs. what was required.
	var observedEvents []string

	for {
		resp, err := stream.Recv()
		if err != nil {
			if ctx.Err() != nil {
				break // deadline reached — fall through to failure reporting
			}
			t.Fatalf("Tetragon gRPC stream error: %v", err)
		}

		kp := resp.GetProcessKprobe()
		if kp == nil || kp.GetFunctionName() != "cap_capable" {
			continue
		}

		proc := kp.GetProcess()
		if proc == nil {
			continue
		}

		// Filter by binary name.
		if comm != "" && filepath.Base(proc.GetBinary()) != comm {
			continue
		}

		// Filter by container: Tetragon may truncate the container ID, so
		// accept events where either string is a prefix of the other.
		if alloyContainerID != "" {
			docker := proc.GetDocker()
			if !strings.HasPrefix(alloyContainerID, docker) && !strings.HasPrefix(docker, alloyContainerID) {
				continue
			}
		}

		args := kp.GetArgs()
		if len(args) == 0 {
			continue
		}
		capName := capabilityName(int(args[0].GetIntArg()))

		// Build a single string of all user-stack symbols so StackContains can
		// be checked with a simple strings.Contains.
		var stackParts []string
		for _, frame := range kp.GetUserStackTrace() {
			if sym := frame.GetSymbol(); sym != "" {
				stackParts = append(stackParts, sym)
			}
		}
		fullStack := strings.Join(stackParts, "\n")

		observedEvents = append(observedEvents, fmt.Sprintf("%s\n%s", capName, fullStack))

		for i, req := range required {
			if satisfied[i] || req.Capability != capName {
				continue
			}
			allContain := true
			for _, sub := range req.StackContains {
				if !strings.Contains(fullStack, sub) {
					allContain = false
					break
				}
			}
			if allContain {
				satisfied[i] = true
			}
		}

		if len(satisfied) == len(required) {
			break // all required events observed — exit early
		}
	}

	t.Logf("Tetragon observed %d capability events for %q:\n%s",
		len(observedEvents), comm, strings.Join(observedEvents, "\n---\n"))

	for i, req := range required {
		if !satisfied[i] {
			t.Errorf("required capability %q (stack must contain %v) was never observed — "+
				"was the code path removed or is the StackContains filter too specific?",
				req.Capability, req.StackContains)
		}
	}
}
