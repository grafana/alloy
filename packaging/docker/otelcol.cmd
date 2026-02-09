@echo off
REM Compatibility wrapper for OpenTelemetry Collector integrations.
REM Delegate to Alloy's OTel engine and pass through all arguments.
"C:\Program Files\GrafanaLabs\Alloy\alloy.exe" otel %*
