package ssh_exporter

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v2"

	"os/user"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

var (
	currentUser       = user.Current
	privateKeyPath    string
	publicKeyPath     string
	mockKnownHostsDir string
)

// Mock ssh-keyscan command
var mockSSHKeyscanCommand = func(targetAddress string) ([]byte, error) {
	publicKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test public key: %w", err)
	}
	return append([]byte(targetAddress+" "), publicKey...), nil
}

// Override the production `sshKeyscanCommand` during tests - we arent scanning from real host connection
func init() {
	sshKeyscanCommand = mockSSHKeyscanCommand
}

// TestMain handles test setup and teardown
func TestMain(m *testing.M) {
	var err error
	privateKeyPath, publicKeyPath, err = generateTempKeyPair()
	if err != nil {
		fmt.Printf("Failed to generate key pair: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(privateKeyPath)
	defer os.Remove(publicKeyPath)

	mockKnownHostsDir, err = setupKnownHosts(publicKeyPath)
	if err != nil {
		fmt.Printf("Failed to set up known_hosts: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(mockKnownHostsDir)

	os.Exit(m.Run())
}

// setupKnownHosts creates a mock known_hosts file
func setupKnownHosts(publicKeyPath string) (string, error) {
	tempDir, err := os.MkdirTemp("", "ssh_exporter_test")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	knownHostsDir := filepath.Join(tempDir, ".ssh")
	knownHostsPath := filepath.Join(knownHostsDir, "known_hosts")

	if err := os.MkdirAll(knownHostsDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	publicKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read public key: %w", err)
	}

	entry := fmt.Sprintf("192.168.1.10 %s", publicKey)
	if err := os.WriteFile(knownHostsPath, []byte(entry), 0600); err != nil {
		return "", fmt.Errorf("failed to write known_hosts file: %w", err)
	}

	return tempDir, nil
}

// generateTempKeyPair generates a temporary private-public key pair and returns file paths
func generateTempKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := &bytes.Buffer{}
	pem.Encode(privateKeyPEM, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyBytes})

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create public key: %w", err)
	}
	publicKeyBytes := ssh.MarshalAuthorizedKey(publicKey)

	privateKeyPath := filepath.Join(os.TempDir(), "test_private_key.pem")
	publicKeyPath := filepath.Join(os.TempDir(), "test_public_key.pem")

	if err := os.WriteFile(privateKeyPath, privateKeyPEM.Bytes(), 0600); err != nil {
		return "", "", fmt.Errorf("failed to write private key: %w", err)
	}
	if err := os.WriteFile(publicKeyPath, publicKeyBytes, 0644); err != nil {
		return "", "", fmt.Errorf("failed to write public key: %w", err)
	}

	return privateKeyPath, publicKeyPath, nil
}

// Test for unmarshalling multiple targets from YAML
func TestConfig_UnmarshalYAML_MultipleTargets(t *testing.T) {
	yamlConfig := `
targets:
  - address: "192.168.1.10"
    port: 22
    username: "admin"
    password: "password"
    command_timeout: "10s"
    custom_metrics:
      - name: "load_average"
        command: "echo 1.23"
        type: "gauge"
        help: "Load average over 1 minute"
  - address: "192.168.1.11"
    port: 22
    username: "monitor"
    key_file: "/path/to/private.key"
    command_timeout: "15s"
    custom_metrics:
      - name: "disk_usage"
        command: "echo '50%'"
        type: "gauge"
        help: "Disk usage percentage"
        parse_regex: '(\d+)%'
`

	var c Config
	require.NoError(t, yaml.UnmarshalStrict([]byte(yamlConfig), &c))

	expectedConfig := Config{
		Targets: []Target{
			{
				Address:        "192.168.1.10",
				Port:           22,
				Username:       "admin",
				Password:       "password",
				CommandTimeout: 10 * time.Second,
				CustomMetrics: []CustomMetric{
					{
						Name:    "load_average",
						Command: "echo 1.23",
						Type:    "gauge",
						Help:    "Load average over 1 minute",
					},
				},
			},
			{
				Address:        "192.168.1.11",
				Port:           22,
				Username:       "monitor",
				KeyFile:        "/path/to/private.key",
				CommandTimeout: 15 * time.Second,
				CustomMetrics: []CustomMetric{
					{
						Name:       "disk_usage",
						Command:    "echo '50%'",
						Type:       "gauge",
						Help:       "Disk usage percentage",
						ParseRegex: `(\d+)%`,
					},
				},
			},
		},
	}

	require.Equal(t, expectedConfig, c)
}

type MockSSHClient struct {
	logger         log.Logger
	executeCommand func(command string) (string, error)
}

func (m *MockSSHClient) Execute(command string) (string, error) {
	return m.executeCommand(command)
}

func (m *MockSSHClient) RunCommand(command string) (string, error) {
	return m.Execute(command) // Reuse the same mocked behavior
}

func (m *MockSSHClient) Close() error {
	return nil // Mock close
}

// Updated TestSSHCollector_Collect
func TestSSHCollector_Collect(t *testing.T) {
	// Set up mock known_hosts
	knownHostsPath, err := setupKnownHosts(publicKeyPath)
	require.NoError(t, err)
	defer os.RemoveAll(knownHostsPath)

	currentUser = func() (*user.User, error) {
		return &user.User{HomeDir: filepath.Dir(knownHostsPath)}, nil
	}
	defer func() { currentUser = user.Current }()

	// Create a target with a custom metric
	target := Target{
		Address:  "192.168.1.10",
		Port:     22,
		Username: "admin",
		Password: "password",
		CustomMetrics: []CustomMetric{
			{
				Name:    "mock_metric",
				Command: "echo 1.23",
				Type:    "gauge",
				Help:    "A mock metric for testing",
			},
		},
	}

	// Use a mock SSH client
	mockClient := &MockSSHClient{
		logger: log.NewNopLogger(),
		executeCommand: func(command string) (string, error) {
			if command == "echo 1.23" {
				return "1.23", nil
			}
			return "", fmt.Errorf("unexpected command: %s", command)
		},
	}

	collector := &SSHCollector{
		logger: log.NewNopLogger(),
		target: target,
		client: mockClient, // Use the mock client
		metrics: map[string]*prometheus.Desc{
			"mock_metric": prometheus.NewDesc("mock_metric", "A mock metric for testing", nil, nil),
		},
	}

	// Test Collect
	ch := make(chan prometheus.Metric)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}

	require.NotEmpty(t, metrics) // Ensure metrics are collected
}

// Use centralized keys in TestNewSSHClient_AuthMethods
func TestNewSSHClient_AuthMethods(t *testing.T) {
	knownHostsPath, err := setupKnownHosts(publicKeyPath)
	require.NoError(t, err)
	defer os.RemoveAll(knownHostsPath)

	tests := []struct {
		name          string
		target        Target
		expectedError string
	}{
		{
			name: "password authentication",
			target: Target{
				Address:  "192.168.1.10",
				Port:     22,
				Username: "user",
				Password: "password",
			},
			expectedError: "",
		},
		{
			name: "private key authentication",
			target: Target{
				Address:  "192.168.1.10",
				Port:     22,
				Username: "user",
				KeyFile:  privateKeyPath,
			},
			expectedError: "",
		},
		{
			name: "missing auth",
			target: Target{
				Address:  "192.168.1.10",
				Port:     22,
				Username: "user",
			},
			expectedError: "no valid authentication method provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewSSHClient(tt.target)
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				require.NotNil(t, client)
			}
		})
	}
}

// Additional tests preserved for new integrations
func TestConfig_NewIntegration(t *testing.T) {
	c := &Config{
		Targets: []Target{
			{
				Address:        "192.168.1.10",
				Port:           22,
				Username:       "admin",
				Password:       "password",
				CommandTimeout: 10 * time.Second,
				CustomMetrics: []CustomMetric{
					{
						Name:    "load_average",
						Command: "cat /proc/loadavg | awk '{print $1}'",
						Type:    "gauge",
						Help:    "Load average over 1 minute",
					},
				},
			},
		},
	}

	logger := log.NewJSONLogger(os.Stdout)
	i, err := c.NewIntegration(logger)
	require.NoError(t, err)
	require.NotNil(t, i)

	lvl := level.NewFilter(logger, level.AllowAll())
	level.Debug(lvl).Log("msg", "test debug log")
}

// TestSSHCollector_Collect_WithLabels verifies that labels defined in custom_metrics
// are applied to the emitted Prometheus metrics.
func TestSSHCollector_Collect_WithLabels(t *testing.T) {
	// Setup a target with labels
	target := Target{
		Address:  "host1",
		Port:     22,
		Username: "user",
		Password: "pass",
		CustomMetrics: []CustomMetric{{
			Name:    "test_metric",
			Command: "echo 5",
			Type:    "gauge",
			Help:    "Test metric",
			Labels:  map[string]string{"host": "host1", "env": "prod"},
		}},
	}
	// Mock client returns '5'
	mockClient := &MockSSHClient{
		logger: log.NewNopLogger(),
		executeCommand: func(command string) (string, error) {
			if command == "echo 5" {
				return "5", nil
			}
			return "", fmt.Errorf("unexpected command: %s", command)
		},
	}
	// Descriptor must list labels in same order as keys slice
	// Use sorted label keys to match collector's sorted ordering
	desc := prometheus.NewDesc("test_metric", "Test metric",
		[]string{"env", "host"}, nil)
	collector := &SSHCollector{
		logger:  log.NewNopLogger(),
		target:  target,
		client:  mockClient,
		metrics: map[string]*prometheus.Desc{"test_metric": desc},
	}
	ch := make(chan prometheus.Metric, 1)
	collector.Collect(ch)
	close(ch)
	metric, ok := <-ch
	require.True(t, ok)
	var dtoMetric dto.Metric
	require.NoError(t, metric.Write(&dtoMetric))
	// Map labels to verify
	got := map[string]string{}
	for _, lp := range dtoMetric.Label {
		got[lp.GetName()] = lp.GetValue()
	}
	require.Equal(t, map[string]string{"host": "host1", "env": "prod"}, got)
}
