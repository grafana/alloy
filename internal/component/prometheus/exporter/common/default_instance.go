package common

import "os"

// HostNameInstanceKey retrieves the hostname identifying the machine the process is
// running on. It will return the value of $HOSTNAME, if defined, and fall
// back to Go's os.Hostname. If that fails, it will return "unknown".
func HostNameInstanceKey() string {
	hostname := os.Getenv("HOSTNAME")
	if hostname != "" {
		return hostname
	}

	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}
