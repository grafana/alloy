# Integration tests

This document provides an outline of how to run and add new integration tests to the project.

The purpose of these tests is to verify pipelines in different scenarios to catch issues between Alloy and external dependencies or other end-to-end workflows.

The tests are using a mix of  [testcontainers-go][] and [docker compose][]. 
[docker compose][] is used to define the external dependencies, while [testcontainers-go][] is used to run Alloy as a container in the docker compose network.
This allows for alloy to run in a privileged mode, which is required for beyla, and for clean setup/teardown of alloy.

## Running tests

Execute the integration tests using the following command:

`go run .`

### Flags

* `--skip-build`: Run the integration tests without building Alloy (default: `false`)
* `--test`: Specifies a particular directory within the tests directory to run (default: runs all tests)
* `--stateful`: Run the tests in stateful mode (default: `false`).
This means that the data is not cleared between runs, and all queries use timestamps to differentiate data from different runs. 
It's useful for a quick local feedback loop but not intended for CI.

## Adding new tests

Follow these steps to add a new integration test to the project:

1. If the test requires external resources, define them as Docker images within the `docker-compose.yaml` file.
2. Create a new directory under the tests directory to house the files for the new test.
3. Within the new test directory, create a file named `config.alloy` to hold the pipeline configuration you want to test.
4. Create a `_test.go` file within the new test directory. This file should contain the Go code necessary to run the test and verify the data processing through the pipeline. All test file should have `alloyintegrationtests` as a build tag.
5. If a test folder contains `test.yaml` this file will be read and parsed. This file can specify different setup requirements that the test have like mount or port mapping. See [config.go](./config.go) for the structure of the test configuration.
6. Ensure any data is written with a unique `test_name` label that matches your assertions. 
   * Since the tests are run concurrently, each Alloy instance used for a test queries for data that will match the `test_name`. 
   * This ensures the correct data verification during the Go testing process.


[testcontainers-go]: https://github.com/testcontainers/testcontainers-go
[docker compose]: https://docs.docker.com/compose/
