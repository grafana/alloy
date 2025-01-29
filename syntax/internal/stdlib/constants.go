package stdlib

import (
	"os"
	"runtime"
	"github.com/grafana/alloy/internal/build"
)

var constants = map[string]string{
	"hostname": "", // Initialized via init function
	"os":       runtime.GOOS,
	"arch":     runtime.GOARCH,
	"version":  "",
}

func init() {
	hostname, err := os.Hostname()

	if err == nil {
		constants["hostname"] = hostname
	}
	if build.Version != "v0.0.0"{
	constants["version"] = build.Version
   	}

}
