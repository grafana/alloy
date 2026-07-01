package deps

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertUserAgentPrefix polls the nginx proxy until it has logged a request whose
// User-Agent starts with want.
//
// It checks for a match among all logged User-Agents (not just the latest), since a
// single Alloy process emits several distinct User-Agents. For example, OTel-native
// components in the OTel engine report a different product name than components in
// the alloyengine extension.
//
// TODO: one day match the full User-Agent string (product name, version, and the
// "(os; deploymode)" metadata) instead of just the prefix. That needs the test to
// know the exact build version of the reused image, which it does not today.
func AssertUserAgentPrefix(t *testing.T, n *NginxProxy, want string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		userAgents, err := n.PushUserAgents()
		require.NoError(c, err)
		require.NotEmpty(c, userAgents, "nginx proxy logged no requests yet")
		for _, ua := range userAgents {
			if strings.HasPrefix(ua, want) {
				return
			}
		}
		require.Failf(c, "no matching User-Agent", "wanted a User-Agent with prefix %q; logged: %v", want, userAgents)
	}, time.Minute, time.Second)
}
