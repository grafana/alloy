@echo off
REM Delegate to Alloy's otel-supervisor subcommand and pass through all arguments.
"C:\Program Files\GrafanaLabs\Alloy\alloy.exe" otel-supervisor %*
