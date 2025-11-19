Long-Form Design Doc Template
=============================

> [!NOTE]
>
> This template is intended for use when a long-form design document is
> necessary. A document using this template should provide sufficient high level
> context to understand the problem as well as detailed proposals with supporting
> data.

# Meta

- **Author(s)**: Name of the author(s)
- **Start Date**: Start date of the design doc
- **Goal End Date**: Intended end date for the design process
- **Primary Reviewers**: A list of at least two primary reviewers

# Abstract

A reasonably concise abstract so that a reader can get an idea of what
the document is about.

# Introduction

A good introduction into the problem that we are trying to solve. This
introduction should not assume much background knowledge from the
reader, but also not re-hash everything from ground up; when any
external documentation can be referenced, this should be given
preference over re-hashing things. The introduction should have the
following sections

## Context

Present ideas, dependencies, systems and general context referred to in
the doc.

### Constraints

Document any relevant constraints that any/all solutions must adhere to.
Try to be as explicit as possible here, as constraints that are obvious
to you may be less so to another reader.

As an example, when working on a task that relates to the eBPF tracers
there are a number of constraints that come into play depending on the
kernel version one wants to support. In this case it would be
appropriate to list each of these constraints, and the kernel versions
they are relevant to.

### Related (Sub-)Problems

If there is a set of sub-problems that need to be explained, and
possibly solved, as part of one or more of your solutions it can make
sense to address them up-front and independently. Include solutions to
such problems here as well, if appropriate.

## Problem Statement

What the problem is, why the problem exists, why do we need to solve it.

**For particularly significant tasks where it is important to solicit
feedback as early as possible it often makes sense to write this section
and the success criteria first. This allows for feedback to be solicited
before writing the rest of the document, in order to ensure everyone is
on the same page before major time investment.**

## Success Criteria

By far the main pitfall in many design docs is not clearly defining what
"success" means. This sometimes can lead the author to meander into
less-important areas, or ignore an important aspect completely. This can
also lead the reviewers to be on a completely different line of thought,
orthogonal to the doc.

This section is more or less what defines the outline and then sets the
stage for the sections/subsections of the document. It's a reality
check that allows the authors and the reviewers to agree on the basis
for the design, and how it should be reviewed. It should help to make
sure everybody is focused on the same thing.

Usually this should be presented as a concise series of bullet points.

## Scope

This expands on the "success criteria", but helps to clear up possible
confusion about what the design doc is about and is not about. It's
usually extremely short, a few bullet points. The most useful info here
is usually "XYZ is not in scope" to avoid ambiguity. This ensures we
don't make incorrect assumptions when reading the doc

# Proposed Solution(s)

Ideally, there should be more than one proposed solution. The document
should list the various solutions, and then go into some depth about
their drawbacks and advantages.

Something to watch out for here is a situation where a problem has N
sub-problems, each with their own M alternative solutions. In that case
it is usually unnecessary, and unreadable, to enumerate all possible
$N \cdot M$ permutations. Assuming it is feasible, outline the possible
solutions to each sub-problem separately, and present a very limited
number of combined solutions, if any.

## Author's Preferred Solution

After presenting the merits of each solution, you should ideally give a
hint as to what your preferred option is, and why. This achieves 2
goals:

-   It ensures solutions are evaluated against each other, despite
    having pros and cons that are not easily comparable. Discussing this
    can provide helpful context to the reader.
-   Writing a justification is a good reality-check, ensuring the design
    doc achieves its goal and provides enough information to make a
    decision.


In case you do not have a preferred solution: to help the reader, you
can describe an appropriate thought process for comparing solutions:

-   Which pros/cons are more important than others?
-   What features or constraints are more important?

We have a finite time/energy budget for assessing designs and providing
this information will help you and the reviewers to prioritize.

# Testing Strategy

Document here how the solution itself will be tested, as well as how the
solution may impact the testability of other components that make use of
it.

The testability of a component itself, as well as its impact on the
testability of other components that make use of it, are important
factors to weigh when choosing between alternate solutions. If it is the
case that your proposed solutions differ in terms of how testable they
are then it may make sense to lift this section into the proposed
solutions, and repeat it for each.

## Testing of Proposed Solution Itself

Outline how the proposed solution will be unit tested and integration
tested. For many implementations this is straightforward and obvious,
but not always. If there are likely to be any difficulties in testing
the solution then outline them here, as well as solutions, if any.

## Impact on Testing of Other Systems/Components

Discuss whether or not the proposed solution will impact the testability
of systems that make use of it, and how. For example, if the solution
involves producing a component that would be difficult to mock out for
the purposes of testing something that makes use of it, then explain
that here.

# Plan to Acquire Missing Data (Optional)

Plan to acquire missing data. Often, some data is missing to properly
evaluate the advantages and disadvantages, and this section details what
data needs to be gotten and how.

# Decision

When all the data is there, a "Decision" section which details what
solution was decided on.
