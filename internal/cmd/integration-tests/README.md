# Integration tests

This document provides an outline of how to run and add new integration tests to the project.

The purpose of these tests is to verify simple, happy-path pipelines to catch issues between the agent and external dependencies.

The external dependencies are launched as Docker containers.

## Running tests

Execute the integration tests using the following command:

`go run .`

**_NOTE:_** The tests don't run on Windows. If you want to run the tests on Linux, you need to set the environment variable OTEL_EXPORTER_ENDPOINT=172.17.0.1:4318.

### Flags

* `--skip-build`: Run the integration tests without building the agent (default: `false`)
* `--test`: Specifies a particular directory within the tests directory to run (default: runs all tests)

## Adding new tests

Follow these steps to add a new integration test to the project:

1. If the test requires external resources, define them as Docker images within the `docker-compose.yaml` file.
2. Create a new directory under the tests directory to house the files for the new test.
3. Within the new test directory, create a file named `config.river` to hold the pipeline configuration you want to test.
4. Create a `_test.go` file within the new test directory. This file should contain the Go code necessary to run the test and verify the data processing through the pipeline.

 _NOTE_: The tests run concurrently. Each agent must tag its data with a label that corresponds to its specific configuration. This ensures the correct data verification during the Go testing process.
