package stdlib

import (
	"os"
	"runtime"
)

var constants = map[string]string{
	"hostname": "", // Initialized via init function
	"os":       runtime.GOOS,
	"arch":     runtime.GOARCH,
}

func init() {
	hostname, err := os.Hostname()
	if err == nil {
		constants["hostname"] = hostname
	}
}
