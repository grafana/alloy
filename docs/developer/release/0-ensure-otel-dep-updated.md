# Ensure OpenTelemetry Collector dependency has been updated

Every minor release **must** include an update to a newer version of OpenTelemetry
Collector (when available). Because the release cadence of OpenTelemetry is
three times more frequent, this update should happen near the end of a six-week
release cycle, such as 1-2 weeks out.

If the OpenTelemetry Collector dependency has not been updated within a release
cycle, **the release should be blocked.**

## Steps

1. Examine the CHANGELOG to ensure that the OpenTelemetry Collector dependency
   has been updated within the release cycle.

2. If the dependency has been updated: continue the release process as normal.

3. If the dependency has not been updated: pause the release process and
   orchestrate updating the dependency.
