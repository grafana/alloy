@echo off
REM Wrapper to run Alloy's embedded OpAMP supervisor as PID 1.
"C:\Program Files\GrafanaLabs\Alloy\alloy.exe" otel-supervisor %*
