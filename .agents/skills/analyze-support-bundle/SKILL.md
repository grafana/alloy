---
name: analyze-support-bundle
description: |
  Analyzes a Grafana Alloy support bundle to identify issues, misconfigurations, warnings, errors, and performance bottlenecks.
  Relevant when:
    - User asks to inspect or analyze an Alloy support bundle.
    - User asks to troubleshoot an Alloy instance using a support bundle.
    - User provides a directory containing files from an extracted support bundle (like alloy-components.json, alloy-logs.txt, etc.) and asks for a diagnosis.
  Don't use when:
    - User is asking general questions about Alloy configuration unrelated to a specific support bundle.
    - Diagnosing issues on systems where no support bundle has been generated or provided.
license: Apache-2.0
metadata:
  version: v1
  publisher: community
---

# Instructions for Support Bundle Analysis

When triggered, you MUST follow this structured, step-by-step diagnostic workflow to analyze the support bundle directory. Do not skip any steps.

## Step 1: Verify the Directory and Locate Essential Files

Verify that the target directory contains the uncompressed support bundle files. The bundle should contain some or all of the following:
- `alloy-metadata.yaml` (Meta info about operating system, architecture, build version, uptime)
- `alloy-components.json` (Component registry, state, and health info)
- `alloy-logs.txt` (Log entries around the time of bundle generation)
- `alloy-runtime-flags.txt` (CLI flags provided to the Alloy binary)
- `alloy-environment.txt` (Relevant environment variables)
- `alloy-metrics-sample-start.txt` / `alloy-metrics-sample-end.txt` (Alloy metrics snapshots)
- `alloy-peers.json` (Clustering peer details, if clustering is enabled)
- `sources/` (Directory containing the configuration files)

If critical files are missing, notify the user which files were not found and proceed with the available files.

## Step 2: Check Metadata, Version, and Uptime

Read `alloy-metadata.yaml` and `alloy-runtime-flags.txt`:
1. **Identify the Alloy Version**: Check if the Alloy build version is significantly outdated. Recommend upgrading if appropriate.
2. **Check Uptime**: If the uptime is very low (e.g., a few seconds or minutes), it may indicate that the Alloy instance is crashlooping. Cross-reference this with the log analysis.
3. **Check Clustering Configuration**: See if `--clustering.enabled` is set to `true` in `alloy-runtime-flags.txt`.

## Step 3: Analyze Component Health and Configuration

1. **Examine `alloy-components.json`**:
   - Parse this JSON file to extract all components.
   - List any components where the health state is NOT `healthy` (e.g., `unhealthy`, `unknown`) or where the `health.message` field contains errors.
2. **Inspect the `sources/` Directory**:
   - Inspect the `.alloy` files in `sources/` to understand the pipeline configuration.
   - Verify if the unhealthy components are wired correctly.
   - Look for common syntax or semantic misconfigurations (e.g., circular dependencies, missing required arguments, extremely low scrape intervals).

## Step 4: Scan Log Outputs for Errors and Warnings

Read `alloy-logs.txt`:
1. **Search for Panics or Fatal Errors**: Look for `panic:` or log lines containing `level=error` indicating critical crashes.
2. **Search for Exporter/Receiver Failures**: Scan for errors related to integrations/pipelines (e.g., `loki.write` rate-limiting/authorization errors, `prometheus.remote_write` HTTP connection timeouts, discovery target failures).
3. **Identify Component Instability**: Look for log lines showing components repeatedly restarting or failing to evaluate.

## Step 5: Evaluate Resource and Runtime Metrics

Read `alloy-metrics-sample-start.txt` and `alloy-metrics-sample-end.txt` (comparing metrics between the two samples, if both are present):
1. **Go Runtime Health**:
   - **Goroutines**: Check `go_goroutines`. High counts or a steep increase indicates a goroutine leak.
   - **Memory Usage**: Check `go_memstats_alloc_bytes` and `go_memstats_sys_bytes`. Compare start and end metrics to detect memory leaks.
   - **Garbage Collection (GC) CPU Fraction**: Check `go_memstats_gc_cpu_fraction`. If it is higher than 0.05 (5%), the instance is experiencing high GC pressure (possibly due to heap exhaustion or churn).
2. **Pipeline Performance & Dropped Data**:
   - **Prometheus Remote Write**: Check `prometheus_remote_write_failed_request_series_total` and `prometheus_remote_write_retries_total` to see if metrics are failing to send.
   - **Loki Write**: Check `loki_write_failed_entries_total` or similar metrics.
   - **OTel Pipelines**: Check processor drop counters (e.g., `otelcol_processor_dropped_spans` or queue limit drops).
   - **Scrape Targets**: Check `prometheus_target_interval_length_seconds` and targets that are down (`up == 0`).

## Step 6: Verify Clustering Status (If Enabled)

If clustering is enabled, read `alloy-peers.json`:
1. Check the list of peers. Ensure all configured peers are listed and active.
2. Look for any peer connectivity errors or split-brain indicators (e.g., inconsistent cluster size reported across nodes).

## Step 7: Synthesize and Present Findings

Generate a detailed report for the user structured as follows:
1. **Executive Summary**: Overall state of the Alloy instance (Healthy, Warning, or Critical).
2. **System Details**: Version, OS/Architecture, and Uptime.
3. **Unhealthy Components**: A list of components with non-healthy states and their error messages.
4. **Configuration & Wiring Analysis**: Summary of any pipeline or syntax issues identified in `sources/`.
5. **Critical Log Findings**: Table or list of unique errors/warnings found in `alloy-logs.txt`.
6. **Performance & Resource Analysis**: Metrics-based analysis of memory, CPU, GC pressure, goroutines, and pipeline throughput/drops.
7. **Actionable Recommendations**: Clear, prioritized steps to resolve the identified issues.
