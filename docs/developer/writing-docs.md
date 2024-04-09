# Writing documentation

This page is a collection of guidelines and best practices for writing
documentation for Grafana Alloy.

## Documentation organisation

The documentation is organized into the following sections:

### Introduction

The best place to start for new users who are onboarding.

We showcase the features of Alloy and help users decide when to use it and
whether it's a good fit for them.

### Get started

This section includes how to quickly install Alloy and get hands-on experience
with a simple "hello world" configuration.

### Concepts

As defined in the [writer's toolkit][]:

> Provides an overview and background information. Answers the question “What is
> it?”.

It helps users to learn the concepts of Alloy used throughout the
documentation.

### Tutorials

As defined in the [writer's toolkit][]:

> Provides procedures that users can safely reproduce and learn from. Answers
> the question: “Can you teach me to …?”

These are pages dedicated to learning. These are more broad,
while [Tasks](#tasks) are focused on one objective. Tutorials may use
non-production-ready examples to facilitate learning, while tasks are expected
to provide production-ready solutions.

### Tasks

As defined in the [writer's toolkit][]:

> Provides numbered steps that describe how to achieve an outcome. Answers the
> question “How do I?”.

However, in Alloy documentation we don't mandate the use of numbered steps.
We do expect that tasks allow users to achieve a specific outcome by following
the page step by step, but we don't require numbered steps because some tasks
branch out into multiple paths, and numbering the steps would look more
confusing.

Tasks are production-ready and contain best practices and recommendations. They
are quite detailed, with Reference pages being the only type of documentation
that has more detail.

Tasks should not be paraphrasing things which are already mentioned in the
Reference pages, such as default values and the meaning of the arguments.
Instead, they should link to relevant Reference pages.

### Reference

The Reference section is a collection of pages that describe Alloy components
and their configuration options exhaustively. This is a more narrow definition
than the one found in the [writer's toolkit][].

We have a dedicated page with the best practices for writing Reference
docs: [writing components documentation][writing-docs].

This is our most detailed documentation, and it should be used as a source of
truth. The contents of the Reference pages should not be repeated in other parts
of the documentation.

### Release notes

Release notes notify users of changes in Alloy that require user action when
upgrading. They are updated as part of the release process.

[writer's toolkit]: https://grafana.com/docs/writers-toolkit/structure/topic-types/
[writing-docs]: writing-component-documentation.md
