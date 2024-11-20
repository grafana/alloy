The code was copied from:
https://github.com/open-telemetry/opentelemetry-collector/tree/v0.114.0/otelcol/internal/configunmarshaler

This forked package exists because the "forked_otelcol" package depends on it.

There is no need to update it every time Alloy's OTel version is updated:
* This is very fundamental code which doesn't change often.
* As long as Static mode's code still behaves as expected and the converter tests pass, that's fine.
* The version of OTel which Agent's repo uses is different from the one which Alloy uses anyway.

The only time when this forked package may need updating is when it fails to build due to refactoring in the OTel repo.