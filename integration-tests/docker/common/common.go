package common

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type Unmarshaler interface {
	Unmarshal([]byte) error
}

const (
	DefaultRetryInterval = 100 * time.Millisecond

	// Timeout to use if no -timeout is provided to go test.
	defaultTimeout = 10 * time.Minute

	// Timeout padding to subtract from the time left until the go test -timeout deadline.
	// This makes sure that polling/assertions fail before the process watchdog kills the test.
	timeoutPadding = 30 * time.Second
)

func FetchDataFromURL(url string, target Unmarshaler) (string, error) {
	return FetchDataFromURLWithHeaders(url, nil, target)
}

func FetchDataFromURLWithHeaders(url string, headers map[string]string, target Unmarshaler) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Non-OK HTTP status: %s, body: %s, url: %s", resp.Status, string(bodyBytes), url)
	}

	err = target.Unmarshal(bodyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal response from %s: Error: %w, Status=%s, Body=%s", url, err, resp.Status, string(bodyBytes))
	}

	return string(bodyBytes), nil
}

// AssertStatefulTestEnv verifies the environment is properly configured if the test is supposed to be stateful
func AssertStatefulTestEnv(t *testing.T) {
	isStateful, err := isStatefulFromEnv()
	if err != nil {
		t.Fatalf("Failed to get stateful test flag from environment: %s", err)
	}
	if !isStateful {
		return
	}

	// If stateful is set to true, ensure AlloyStartTimeEnv is also set
	_, err = startTimeFromEnv()
	if err != nil {
		t.Fatalf("Failed to get Alloy start time from environment: %s", err)
	}
}

// AlloyStartTimeUnix pulls the start time from env.
func AlloyStartTimeUnix() int64 {
	startTime, err := startTimeFromEnv()
	if err != nil {
		return 0
	}
	return startTime
}

func AlloyStartTimeUnixNano() int64 {
	startTime, err := startTimeFromEnv()
	if err != nil {
		return 0
	}
	return startTime * int64(time.Second)
}

func startTimeFromEnv() (int64, error) {
	startingAtEnv, ok := os.LookupEnv(AlloyStartTimeEnv)
	if !ok {
		return 0, fmt.Errorf("%s not set", AlloyStartTimeEnv)
	}

	parsed, err := strconv.ParseInt(startingAtEnv, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s value %s as an int64: %s", AlloyStartTimeEnv, startingAtEnv, err)
	}

	return parsed, nil
}

func IsStatefulTest() bool {
	isStateful, err := isStatefulFromEnv()
	if err != nil {
		return false
	}
	return isStateful
}

func isStatefulFromEnv() (bool, error) {
	statefulEnv := os.Getenv(TestStatefulEnv)
	if statefulEnv == "" {
		return false, nil
	}

	isStateful, err := strconv.ParseBool(statefulEnv)
	if err != nil {
		return false, fmt.Errorf("failed to parse %s value %s as a boolean: %s", TestStatefulEnv, statefulEnv, err)
	}

	return isStateful, nil
}

// TestTimeout returns how long polling helpers should keep retrying. When the
// test is run with go test -timeout (as the integration harness does), this is
// assertionPollingTimeout, but never longer than the time left until that
// deadline. Otherwise it defaults to DefaultTimeout.
func TestTimeout(t *testing.T) time.Duration {
	if t != nil {
		if deadline, ok := t.Deadline(); ok {
			remaining := time.Until(deadline)
			return remaining - timeoutPadding
		}
	}
	return defaultTimeout
}

func SanitizeTestName(t *testing.T) string {
	name := strings.TrimPrefix(t.Name(), "Test")
	return strings.ToLower(name)
}
