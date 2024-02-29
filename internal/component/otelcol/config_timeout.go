package otelcol

import "time"

// DefaultTimeout holds the default timeout used for components which can time
// out from requests.
var DefaultTimeout = 5 * time.Second
