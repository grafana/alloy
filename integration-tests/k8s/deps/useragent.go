package deps

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertUserAgentEquals polls the nginx proxy until it has logged a request whose
// User-Agent equals want.
//
// It checks for presence among all logged User-Agents (not just the latest),
// since a single Alloy process can emit several distinct User-Agents.
// For example, OTel-native components in OTel engine have a different
// User Agent than components in the alloyengine extension.
func AssertUserAgentEquals(t *testing.T, n *NginxProxy, want string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		userAgents, err := n.PushUserAgents()
		require.NoError(c, err)
		require.NotEmpty(c, userAgents, "nginx proxy logged no requests yet")
		for _, ua := range userAgents {
			if ua == want {
				return
			}
		}
		require.Failf(c, "no matching User-Agent", "wanted a User-Agent equal to %q; logged: %v", want, userAgents)
	}, time.Minute, time.Second)
}
