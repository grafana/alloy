// Program alloy is Grafana Alloy built without the OTel Engine.
package main

import (
	"os"

	"github.com/grafana/alloy/flowcmd"
)

func main() {
	if err := flowcmd.RootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
