package common

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

type Unmarshaler interface {
	Unmarshal([]byte) error
}

const (
	DefaultRetryInterval = 100 * time.Millisecond
	DefaultTimeout       = 90 * time.Second
)

func FetchDataFromURL(url string, target Unmarshaler) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Non-OK HTTP status: %s, body: %s, url: %s", resp.Status, string(bodyBytes), url)
	}

	return target.Unmarshal(bodyBytes)
}

// AssertStatefulTestEnv verifies the environment is properly configured if the test is supposed to be stateful
func AssertStatefulTestEnv(t *testing.T) {
	// Check if stateful is set
	statefulEnv := os.Getenv(TestStatefulEnv)
	if statefulEnv == "" {
		return
	}

	isStateful, err := strconv.ParseBool(statefulEnv)
	if err != nil {
		t.Fatalf("Invalid value for %s: %s", TestStatefulEnv, err)
	}

	if !isStateful {
		return
	}

	// If stateful is set to true, ensure AlloyStartTimeEnv is also set
	_, err = startTimeFromEnv()
	if err != nil {
		t.Fatalf("Failed to get Alloy start time from environment: %s", err)
	}
	return
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

func TestTimeoutEnv(t *testing.T) time.Duration {
	if toStr := os.Getenv(TestTimeout); toStr != "" {
		if to, err := time.ParseDuration(toStr); err == nil {
			return to
		} else {
			t.Logf("failed to parse %s value %s as a duration: %s, defaulting to %s", TestTimeout, toStr, err, DefaultTimeout)
		}
	} else {
		t.Logf("%s not set, defaulting to %s", TestTimeout, DefaultTimeout)
	}
	return DefaultTimeout
}
