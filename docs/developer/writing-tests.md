# Writing tests

## Levels of tests

The table below shows the levels of tests we use in Alloy, from the cheapest and most frequently used level (top) to the most expensive and rarely used level (bottom). More tests live near the top; fewer live near the bottom.

| **Test level** | **Description** | **Run schedule** |
| --- | --- | --- |
| **Unit tests** | Regular Go tests at unit, package, and component level. Test any logic and edge cases or failure modes that the code is expected to handle. | Every PR |
| **Pipeline tests** | Tests which run a pipeline of Alloy components and verify the expected output. Focus is on making sure the components work together correctly and propagate data as expected. Covers some failure modes and safety mechanisms. | Every PR |
| **Integration tests** | These verify that Alloy components that connect to an external dependency work correctly with it (integrate with it). We also verify the configuration and engine features such as the Helm chart or the health check endpoints. We use Docker and local k8s clusters to keep tests realistic. | Every PR |
| **Dogfooding** | Maintainers run Alloy in their own clusters in real-world scenarios. Covers e2e and scale at the same time, but only for functionality we currently use. | 24/7 |
| **End-to-end tests** | (Planned) Further tests of the most important use-cases in more realistic environments. Can run for longer and at larger scale. | Nightly (TBD) |
| **Scale tests** | (Planned) Benchmarks for performance and resource use at scale. Purpose is to produce useful performance metrics and catch any regressions. | Nightly (TBD) |
| **Manual testing** | Ad-hoc exploration in maintainer's own or shared environments. | Ad-hoc |

There are also microbenchmarks and fuzz tests, but these are not covered here right now. For writing these, use general Go programming best practices.

## Choosing the level of tests to write

Everything we build should have some tests. Use these guidelines to choose the level(s):

### Always: unit tests

We always add unit tests for code we write. Exceptions are rare. Make sure the test covers any non-trivial logic and edge cases or failure modes that the code is expected to handle.

### Consider: pipeline tests

If the change you are making modifies the way that components connect to each other, we generally expect to have a pipeline test to verify it works as expected. Pipeline tests are also useful if you want to verify a feature or a behaviour of several components together.

Be careful not to implement pipeline tests for features that would be more suitable to verify in unit tests.

Pipeline tests can be found in the `internal/pipelinetest` package.

### Consider: integration tests

Components that talk to an external dependency would ideally have an integration test to verify they work correctly together. This is not always practical, especially when connecting to a very complex or proprietary 3rd party dependency.

Cross-cutting features like Alloy's configuration management via Helm chart or its API and endpoints are also good candidates for integration tests.

Be careful not to implement tests that verify components inside a pipeline work correctly together. These would typically be covered by pipeline tests.

Integration tests can be found in the `integration-tests` directory. We recommend using the `k8s` tests whenever possible, as they are the most frequent deployment target and should be able to scale to larger number of tests.

### Consider: dogfooding

NOTE: this section may be only relevant to Alloy maintainers.

Dogfooding goes beyond other levels of tests in that it uses a more realistic environment and more resources for extended periods of time. We want to use it for any critical and most popular features on top of previous levels.

If we benefit from using a feature in our own Alloy deployments without compromising on cost or supportability, we should add a dogfooding configuration for it. If the feature would conflict with or overlap an existing solution too much, we may consider adding end-to-end and/or scale tests instead.

When we dogfood, we should consume the feature the same way most of our users would. Diverging from standard user setups is a signal that the standard may need to change. We want to keep best practices and our dogfooding setup in sync.

### Consider: end-to-end tests

NOTE: we currently don't have any formal end-to-end tests; these are planned for the future.

If a feature isn't sufficiently covered by integration tests, can't be dogfooded, and is popular or critical, we should probably add end-to-end tests for it.

### Consider: scale tests

NOTE: we currently don't have any formal scale tests; these are planned for the future.

This is a more controlled way to talk about performance. Over time, major features for which users raise performance concerns or questions, and anything we cover in resource-usage estimate docs, should have a scale test.

### Consider: manual testing

Ideally we want to scale our efforts and add automated tests that will prevent regressions, so manual tests are not recommended. However, these can be used as an additional safety check when we see given change as risky.
