# Proposal: Add a component to perform label mapping efficiently

* Author(s): Nicolas DUPEUX
* Last updated: 19/11/2024
* Original issue: https://github.com/grafana/alloy/pull/2025

## Abstract

Add a component to populate labels values based on a lookup table.

## Problem

Using `prometheus.relabel` to populate a label value based on another label value is inefficient as we have to have a rule block for each source label value.

If we have 1k values to map, we'll have to execute 1k regex for each datapoint resulting in an algorithm complexity of O(n).

## Proposal

Replace regex computing by a lookup table. Algorithm complexity goes from O(n) to O(1)

## Pros and cons

Pros:
    - resource efficient
    
Cons:
    - New component

## Alternative solutions

- Instanciate more CPU resources to perform the task
- Optimize prometheus.relabel component
- Summarize regex when severals keys have to same value.

## Compatibility

As this is a new component, there isn't any compatibility issue as long as you don't use it.

## Implementation

https://github.com/grafana/alloy/pull/2025

## Related open issues

None
