The code was copied from:
https://github.com/open-telemetry/opentelemetry-collector/blob/v0.114.0/otelcol/unmarshaler.go
https://github.com/open-telemetry/opentelemetry-collector/blob/v0.114.0/otelcol/unmarshaler_test.go
https://github.com/open-telemetry/opentelemetry-collector/blob/v0.114.0/otelcol/factories_test.go

The purpose of this forked package is to allow Static mode to use the "Unmarshal(...)" function.
In the original OTel repo it is internal (it's lowercase - "unmarshal(...)").

There is no need to update this forked package every time Alloy's OTel version is updated:
* This is very fundamental code which doesn't change often.
* As long as Static mode's code still behaves as expected and the converter tests pass, that's fine.
* The version of OTel which Agent's repo uses is different from the one which Alloy uses anyway.

The only time when this forked package may need updating is when it fails to build due to refactoring in the OTel repo.