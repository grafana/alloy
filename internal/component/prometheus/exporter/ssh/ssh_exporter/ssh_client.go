package ssh_exporter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSHClient struct {
	config  *ssh.ClientConfig
	host    string
	port    int
	logger  log.Logger
	timeout time.Duration
}

var sshKeyscanCommand = func(targetAddress string) ([]byte, error) {
	cmd := exec.Command("ssh-keyscan", "-H", targetAddress)
	return cmd.Output()
}

func ensureKnownHosts(knownHostsPath, targetAddress string) error {
	// On Windows, skip known_hosts handling
	if runtime.GOOS == "windows" {
		return nil
	}
	// Ensure .ssh directory exists
	if err := os.MkdirAll(filepath.Dir(knownHostsPath), 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	var knownHostsContent []string
	if _, err := os.Stat(knownHostsPath); err == nil {
		content, err := os.ReadFile(knownHostsPath)
		if err != nil {
			return fmt.Errorf("failed to read known_hosts file: %w", err)
		}
		knownHostsContent = strings.Split(string(content), "\n")
	}

	// Try up to 3 times to fetch the host key, then skip if unsuccessful
	var output []byte
	var scanErr error
	const maxAttempts = 3
	for i := 0; i < maxAttempts; i++ {
		output, scanErr = sshKeyscanCommand(targetAddress)
		if scanErr == nil && len(output) > 0 {
			break
		}
		fmt.Printf("failed to fetch host key for %s: %v; retrying in 1s...\n", targetAddress, scanErr)
		time.Sleep(time.Second)
	}
	if scanErr != nil || len(output) == 0 {
		// Unable to fetch a host key; skip known_hosts update
		return nil
	}
	scannedKey := strings.TrimSpace(string(output))

	for _, line := range knownHostsContent {
		if strings.Contains(line, targetAddress) {
			// Host already present in known_hosts; skip updating
			return nil
		}
	}

	file, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open known_hosts file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(scannedKey + "\n"); err != nil {
		return fmt.Errorf("failed to write to known_hosts file: %w", err)
	}

	return nil
}

func NewSSHClient(target Target) (*SSHClient, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("unable to determine current user: %w", err)
	}

	knownHostsPath := filepath.Join(usr.HomeDir, ".ssh", "known_hosts")

	// Ensure known_hosts exists and is valid
	if err := ensureKnownHosts(knownHostsPath, target.Address); err != nil {
		return nil, fmt.Errorf("failed to ensure known_hosts: %w", err)
	}

	var hostKeyCallback ssh.HostKeyCallback
	if runtime.GOOS == "windows" {
		// Skip host key verification on Windows
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		var err error
		hostKeyCallback, err = knownhosts.New(knownHostsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize known_hosts verification: %w", err)
		}
	}

	// Build SSH ClientConfig
	config := &ssh.ClientConfig{
		User:            target.Username,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: hostKeyCallback,
		Timeout:         target.CommandTimeout,
	}

	// Add Password Authentication
	if target.Password != "" {
		config.Auth = append(config.Auth, ssh.Password(target.Password))
	}

	// Add Private Key Authentication (if provided)
	if target.KeyFile != "" {
		// Ensure private key file has secure permissions (owner-only)
		fi, err := os.Stat(target.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to stat private key file %s: %w", target.KeyFile, err)
		}
		perm := fi.Mode().Perm()
		if perm&0o077 != 0 {
			return nil, fmt.Errorf("insecure private key file permissions %o for %s: must be owner-only", perm, target.KeyFile)
		}
		// Read and parse the key
		keyBytes, err := os.ReadFile(target.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read private key file %s: %w", target.KeyFile, err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to parse private key: %w", err)
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}

	// Validate at least one auth method (unless skipped)
	if !target.SkipAuth && len(config.Auth) == 0 {
		return nil, fmt.Errorf("no valid authentication method provided (password or private key)")
	}

	return &SSHClient{
		config:  config,
		host:    target.Address,
		port:    target.Port,
		logger:  log.NewNopLogger(),
		timeout: target.CommandTimeout,
	}, nil
}

// RunCommand executes a command on the remote host without context cancellation.
// It is equivalent to RunCommandContext with a background context.
func (c *SSHClient) RunCommand(command string) (string, error) {
	return c.RunCommandContext(context.Background(), command)
}

// RunCommandContext executes a command on the remote host with cancellation support.
func (c *SSHClient) RunCommandContext(ctx context.Context, command string) (string, error) {
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", c.host, c.port), c.config)
	if err != nil {
		if c.logger != nil {
			level.Error(c.logger).Log("msg", "failed to connect to SSH", "host", c.host, "port", c.port, "err", err)
		}
		return "", fmt.Errorf("failed to connect to SSH: %w", err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		if c.logger != nil {
			level.Error(c.logger).Log("msg", "failed to create SSH session", "err", err)
		}
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	var output bytes.Buffer
	session.Stdout = &output
	session.Stderr = &output

	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()

	select {
	case err := <-done:
		if err != nil {
			if c.logger != nil {
				level.Error(c.logger).Log("msg", "command execution failed", "command", command, "err", err)
			}
			return "", fmt.Errorf("command execution failed: %w", err)
		}
	case <-time.After(c.timeout):
		// Attempt to send a termination signal to the remote command
		if err := session.Signal(ssh.SIGKILL); err != nil {
			if c.logger != nil {
				level.Error(c.logger).Log("msg", "failed to send SIGKILL to remote command", "err", err)
			}
		}
		return "", fmt.Errorf("command execution timed out after %v", c.timeout)
	}

	return output.String(), nil
}
