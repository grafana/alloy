package main

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/cmd/integration-tests-k8s/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Running the test in a stateful way means that k8s resources won't be deleted at the end.
// It's useful for debugging. You can run the test like this:
// ALLOY_STATEFUL_K8S_TEST=true make integration-test-k8s
func isStateful() bool {
	stateful, _ := strconv.ParseBool(os.Getenv("ALLOY_STATEFUL_K8S_TEST"))
	return stateful
}

func TestMimirAlerts(t *testing.T) {
	testDir := "./"

	cleanupFunc := util.BootstrapTest(testDir, "mimir-alerts-kubernetes", isStateful())
	defer cleanupFunc()

	terminatePortFwd := util.ExecuteBackgroundCommand(
		"kubectl", []string{"port-forward", "service/mimir-nginx", "12346:80", "--namespace=mimir-test"},
		"Port forward Mimir")
	defer terminatePortFwd()

	expectedMimirConfig := `template_files:
    default_template: |-
        {{ define "__alertmanager" }}AlertManager{{ end }}
        {{ define "__alertmanagerURL" }}{{ .ExternalURL }}/#/alerts?receiver={{ .Receiver | urlquery }}{{ end }}
alertmanager_config: |
    global:
      resolve_timeout: 5m
      http_config:
        follow_redirects: true
        enable_http2: true
      smtp_hello: localhost
      smtp_require_tls: true
    route:
      receiver: "null"
      continue: false
      routes:
      - receiver: testing/alertmgr-config1/null
        matchers:
        - namespace="testing"
        continue: true
        routes:
        - receiver: testing/alertmgr-config1/myamc
          continue: true
      - receiver: testing/alertmgr-config2/null
        matchers:
        - namespace="testing"
        continue: true
        routes:
        - receiver: testing/alertmgr-config2/database-pager
          matchers:
          - service="webapp"
          continue: false
          group_wait: 10s
    receivers:
    - name: "null"
    - name: alloy-namespace/global-config/myreceiver
    - name: testing/alertmgr-config1/null
    - name: testing/alertmgr-config1/myamc
      webhook_configs:
      - send_resolved: false
        http_config:
          follow_redirects: true
          enable_http2: true
        url: http://test.url
        url_file: ""
        max_alerts: 0
        timeout: 0s
    - name: testing/alertmgr-config2/null
    - name: testing/alertmgr-config2/database-pager
    templates:
    - default_template
`

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		actualMimirConfig := util.Curl(c, "http://localhost:12346/api/v1/alerts")
		require.Equal(c, expectedMimirConfig, actualMimirConfig)
	}, 15*time.Second, 100*time.Millisecond)
	// TODO: Print Alloy's logs if the test fails.
	// TODO: Try changing some k8s resources and check Mimir's config again.
}
