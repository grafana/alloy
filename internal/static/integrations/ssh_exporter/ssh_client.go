package ssh_exporter

import (
    "bytes"
    "fmt"
    "os"
    "path/filepath"
    "time"

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

func NewSSHClient(target Target) (*SSHClient, error) {
    usr, err := user.Current()
    if err != nil {
        return nil, fmt.Errorf("unable to determine current user: %w", err)
    }

    knownHostsPath := filepath.Join(usr.HomeDir, ".ssh", "known_hosts")

    if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
        return nil, fmt.Errorf("known_hosts file not found at %s", knownHostsPath)
    }

    hostKeyCallback, err := knownhosts.New(knownHostsPath)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize known_hosts verification: %w", err)
    }

    config := &ssh.ClientConfig{
        User: target.Username,
        Auth: []ssh.AuthMethod{},
        HostKeyCallback: hostKeyCallback,
        Timeout: time.Duration(target.CommandTimeout) * time.Second,
    }

    if target.Password != "" {
        config.Auth = append(config.Auth, ssh.Password(target.Password))
    } else if target.KeyFile != "" {
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
