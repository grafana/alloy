package util

import (
	"os/exec"
	"strings"

	"github.com/stretchr/testify/assert"
)

// AssertEventLogLine checks the Windows Application event log for a specific log line from Alloy.
func AssertEventLogLine(c *assert.CollectT, logLine string) {
	// Filter on the provider server-side. Filtering client-side with Where-Object means
	// -MaxEvents caps the newest N events of the whole Application log, so a chatty
	// machine can push Alloy's events out of the window entirely. With -FilterHashtable
	// the cap applies to Alloy's events alone. Get-WinEvent writes a non-terminating
	// error when nothing matches the filter, hence -ErrorAction SilentlyContinue.
	psScript := `Get-WinEvent -FilterHashtable @{LogName='Application'; ProviderName='Alloy'} -MaxEvents 500 -ErrorAction SilentlyContinue | ForEach-Object { $_.Message } | Out-String`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	out, err := cmd.CombinedOutput()
	assert.NoError(c, err, "get Windows Event Log")
	msg := string(out)
	assert.True(c, strings.Contains(msg, logLine),
		"event log did not contain log line %q from Alloy; got %d bytes from Alloy events", logLine, len(msg))
}
