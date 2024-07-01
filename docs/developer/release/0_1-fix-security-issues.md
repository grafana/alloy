# Fix important security issues

It is important that CVEs are fixed before the SLA to fix them has been breached.

## Steps

1. Make sure all [security issues] have been resolved.
   If any issues remain unresolved, the SLA for them should not be breached by the time of the next release.
2. Make sure Trivy and Snyk scans do not report any vulnerabilities.
   If it is not practical to resolve all vulnerabilities, 
   make sure the SLA for any remaining ones will not be breached by the time of the next release.

[security issues]: https://github.com/grafana/alloy/issues?q=is%3Aopen+is%3Aissue+label%3Asecurity