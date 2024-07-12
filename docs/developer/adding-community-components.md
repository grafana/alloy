# Adding community components

[Community components][cc] are components that are implemented and maintained by the community.

## Community vs core components

The community components category is mainly targeted at vendor-specific components for which Grafana does not offer commercial support (for example the Datadog exporter).

Some vendor-agnostic components may also be accepted as community components if they are not accepted as core components.

## Before opening a proposal

The first step is to ensure that the proposal meets the following criteria and does not duplicate existing proposals:

* Avoid overlapping functionalities.
* Avoid components that can be implemented as [modules][module].
* Avoid components that affect our dependencies in an undesired way, such as pulling in an incompatible version or bloating the collector.
* Make sure that the code licenses are compatible with Alloy's [license][].
* You are willing to be a maintainer of the component.

While not mandatory, it is beneficial if:

* The component comes from [Opentelemetry's contrib repository][otel].
* The component supports all the [platforms that Alloy supports][platforms].

We are implementing a gradual rollout strategy for community components to allow for process refinement as needed.
Even if a proposal meets all the established criteria, we may exercise caution in its acceptance to ensure a smooth integration process.

## Creating a proposal

To create a proposal, submit a new issue in [Alloy's repo][issue] with the template `Community component proposal`.

Make sure that the issue has the label `community-component` before submitting it.

The proposal will go through our [review process][].

## Implementing the component

When the proposal has been accepted, you can claim it and start the implementation (make sure that you are familiar with our [contribution guidelines][contributing]).

Doing the implementation will make you a maintainer of the component. This will take effect as soon as the pull request is merged to the main branch.

Community components live amongst other components in the code. The only difference with core components is that the flag `Community` should be set to true when registering the component.

The documentation should also follow the same pattern as the core components but at a different [location][cc dir].

## Being a community component maintainer

Community component maintainers may be pinged on GitHub issues and Pull Requests related to their components. They are expected to help keeping their component and the documentation up to date with the project (e.g. if it's a component from [OpenTelemetry's contrib repository][otel], the implementation should match the current OTel version of the project).

Failing to keep the component up to date may result in the component being deprecated, disabled, or removed.

The list of maintainers is kept as a comment in the component's Go file:
* Anyone can volunteer to become a maintainer by opening a pull request to add themselves as code owner for the component.
* Any maintainer can step out of the role by opening a pull request to remove their GitHub handle from code owners for the component.


[cc]: ../sources/get-started/community_components.md
[cc dir]: https://grafana.com/docs/alloy/latest/reference/community_components
[module]: ../sources/get-started/modules.md
[license]: ../../LICENSE
[platforms]: ../sources/introduction/supported-platforms.md
[otel]: https://github.com/open-telemetry/opentelemetry-collector-contrib
[issue]: https://github.com/grafana/alloy/issues/new/choose
[contributing]: contributing.md
[review process]: ../design/README.md
[review template]: ../design/template.md