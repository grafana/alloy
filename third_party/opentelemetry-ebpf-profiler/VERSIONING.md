# Versioning Policy
This document outlines the versioning strategy for the OpenTelemetry eBPF Profiler project.

# Development Status
This project is currently under active development. As such, users should be aware that significant changes, including breaking API modifications, may occur at any time.

# Automatic Version Tagging
Automatic version tags are generated on a monthly basis to reflect ongoing development progress.

# Versioning Scheme
This project adheres to [Semantic Versioning 2](https://semver.org/spec/v2.0.0.html).

Major Version Zero (0.y.z): While the project is in its initial development phase, the major version will remain 0. Anything MAY change at any time. The public API SHOULD NOT be considered stable. Users are advised to exercise caution and expect potential breaking changes without prior notice during this phase.

# Automatic Tag Format
The format for automatically generated tags currently follows v0.0.x, where x represents the year followed by the week number.

## Example:

- `v0.0.202501` would indicate the first week of 2025.

- `v0.0.202515` would indicate the fifteenth week of 2025.
