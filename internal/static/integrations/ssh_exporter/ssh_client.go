package ssh_exporter

import (
    "bytes"
    "fmt"
    "os"
    "path/filepath"
    "time"
    "os/exec"
    "strings"

    "github.com/go-kit/log"
    "github.com/go-kit/log/level"
    "golang.org/x/crypto/ssh"
    "golang.org/x/crypto/ssh/knownhosts"
    "os/user"
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

    var output []byte
    var scanErr error
    for i := 0; i < 3; i++ {
        output, scanErr = sshKeyscanCommand(targetAddress)
        if scanErr == nil {
            break
        }
        fmt.Printf("Attempt %d: failed to fetch host key for %s: %v\n", i+1, targetAddress, scanErr)
        time.Sleep(time.Second)
    }
    if len(output) == 0 {
        return fmt.Errorf("failed to fetch host key for %s after 3 attempts: last error: %w", targetAddress, scanErr)
    }
    scannedKey := strings.TrimSpace(string(output))

    for _, line := range knownHostsContent {
        if strings.Contains(line, targetAddress) {
            if line != scannedKey {
                return fmt.Errorf(
                    "host key mismatch for %s: existing key [%s] differs from scanned key [%s]. Manual verification required.",
                    targetAddress, line, scannedKey,
                )            }
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

    hostKeyCallback, err := knownhosts.New(knownHostsPath)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize known_hosts verification: %w", err)
    }

    // Build SSH ClientConfig
    config := &ssh.ClientConfig{
        User:            target.Username,
        Auth:            []ssh.AuthMethod{},
        HostKeyCallback: hostKeyCallback,
        Timeout:         time.Duration(target.CommandTimeout) * time.Second,
    }

    // Add Password Authentication
    if target.Password != "" {
        config.Auth = append(config.Auth, ssh.Password(target.Password))
    }

    // Add Private Key Authentication
    if target.KeyFile != "" {
        key, err := os.ReadFile(target.KeyFile)
        if err != nil {
            return nil, fmt.Errorf("unable to read private key file %s: %w", target.KeyFile, err)
        }
        signer, err := ssh.ParsePrivateKey(key)
        if err != nil {
            return nil, fmt.Errorf("unable to parse private key: %w", err)
        }
        config.Auth = append(config.Auth, ssh.PublicKeys(signer))
    }

    // Validate at least one auth method
    if len(config.Auth) == 0 {
        return nil, fmt.Errorf("no valid authentication method provided (password or private key)")
    }

    return &SSHClient{
        config:  config,
        host:    target.Address,
        port:    target.Port,
        logger:  log.NewNopLogger(),
        timeout: time.Duration(target.CommandTimeout) * time.Second,
    }, nil
}



func (c *SSHClient) RunCommand(command string) (string, error) {
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
