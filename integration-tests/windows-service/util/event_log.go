package util

import (
	"os/exec"
	"strings"

	"github.com/stretchr/testify/assert"
)

// AssertEventLogLine checks the Windows Application event log for a specific log line from Alloy.
func AssertEventLogLine(c *assert.CollectT, logLine string) {
	psScript := `Get-WinEvent -LogName Application -MaxEvents 500 -ErrorAction SilentlyContinue | Where-Object { $_.ProviderName -eq 'Alloy' } | ForEach-Object { $_.Message } | Out-String`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	out, err := cmd.CombinedOutput()
	assert.NoError(c, err, "get Windows Event Log")
	msg := string(out)
	assert.True(c, strings.Contains(msg, logLine),
		"event log did not contain log line %q from Alloy; got %d bytes from Alloy events", logLine, len(msg))
}
