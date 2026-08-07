@echo off
REM Wrapper to run Alloy's embedded OpAMP supervisor as PID 1.
REM Delegate to Alloy's otel-supervisor subcommand and pass through all arguments.
"C:\Program Files\GrafanaLabs\Alloy\alloy.exe" otel-supervisor %*
