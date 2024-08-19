# Proposal: Standardizing the Alloy standard library

* Author: Robert Fratto (@rfratto)
* Last updated: 2024-06-27
* Discussion link: <https://github.com/grafana/alloy/pull/1161>

## Abstract

This proposal introduces a way to standardise on how new capabilities are added
to Alloy's standard library so that usage of the standard library feels
consistent.

## Problem

As of Alloy v1.2.0, there are 19 identifiers in the standard library:

* `base64_decode`
* `coalesce`
* `concat`
* `constants`
* `env`
* `format`
* `join`
* `json_decode`
* `json_path`
* `nonsensitive`
* `replace`
* `split`
* `to_lower`
* `to_upper`
* `trim`
* `trim_prefix`
* `trim_space`
* `trim_suffix`
* `yaml_decode`

These identifiers were added organically over time; there has been little
thought to consistency across all identifiers.

While 19 identifiers is manageable and possible for a user to remember the most
important ones, if we don't establish a way to standardise existing and new
identifiers, the inconsistency between identifiers will only grow over time as
new identifiers are added and become harder to understand.

This proposal aims to:

* Establish a convention for how to name identifiers in the standard library.
* Propose a way to align existing identifiers with the new convention without
  breaking backwards compatibility.

For the sake of demonstrating the convention, new identifiers are provided as
examples, but are out of scope of this proposal and should not be considered
accepted if this proposal is accepted.

## Proposal

The standard library will introduce namespaces for identifiers that group
similar identifiers together:

* Array functions
    * `array.concat` (previously `concat`)
* Conversion functions
    * `convert.nonsensitive` (previously `nonsensitive`)
* Encoding functions
    * `encoding.from_base64` (previously `base64_decode`)
    * `encoding.from_json` (previously `json_decode`)
    * `encoding.from_yaml` (previously `yaml_decode`)
* System-related functions
    * `sys.env` (previously `env`)
* String functions
    * `string.format` (previously `format`)
    * `string.join` (previously `join`)
    * `string.replace` (previously `replace`)
    * `string.split` (previously `split`)
    * `string.to_lower` (previously `to_lower`)
    * `string.to_upper` (previously `to_upper`)
    * `string.trim` (previously `trim`)
    * `string.trim_prefix` (previously `trim_prefix`)
    * `string.trim_space` (previously `trim_space`)
    * `string.trim_suffix` (previously`trim_suffix`)

> **NOTE**: The decoding functions were placed into an `encoding` namespace to
> leave the door open for having a single namespace for both performing
> decoding and encoding, so that `encoding.from_base64` could coexist alongside
> a `encoding.to_base64`.
>
> In this context, the namespace is used as a noun to categorize both the
> encoding and decoding actions.

For identifiers where a namespace has been introduced, the old identifier will
be marked deprecated for removal for the next major release. The documentation
will present the namespaced functions prominently and mention the old name as a
deprecated alias.

Some identifiers have not been given a namespace because I could not easily
identify one: `constants`, `coalesce`, and `json_path`. Until a namespace is
identified (if ever), these identifiers will not be deprecated.

New identifiers **should** be given a namespace; namespacing makes it easier
for the same identifier to exist in two different namespaces, such as
`string.join` (for joining multiple strings) alongside a hypothetical
`path.join` (for joining file system paths).

Introducing a new namespace should be done as a proposal; ideally proposing
multiple identifiers that would comprise that namespace. Introducing new
identifiers in an existing namespace should also be done as a proposal, much
like today.

I've elected for namespaces to use dot separation (`NAMESPACE.IDENTIFIER`) as
opposed to underscores (`NAMESPACE_IDENTIFIER`) to align with `constants` and
component names, which both use dot separation for namespacing.

### Example future stdlib

Three of the five namespaces above only have one identifier. However, there are
many new identifiers we may want to include in the future. Here's an example of
what these namespaces could eventually look like:

> **NOTE**: These are not part of the proposal and are only provided as an
> example.

* Array functions
    * `array.concat`
    * `array.length` (**new**; return the length of an array)
    * `array.contains` (**new**; check if an array contains an element)
    * `array.distinct` (**new**; return an array with only unique elements)
* Conversion functions
    * `convert.nonsensitive`
    * `convert.sensitive` (**new**; explicitly convert a string into a secret)
    * `convert.to_number` (**new**; convert a string to a number)
    * `convert.to_string` (**new**; convert a number to a string)
    * `convert.to_bool` (**new**; convert a string or number to a boolean)
* Encoding functions
    * `encoding.from_base64`
    * `encoding.from_json`
    * `encoding.from_yaml`
* System-related functions
    * `sys.env`
    * `sys.cpu_count` (**new**; the number of CPUs on a system)
    * `sys.memory` (**new**; the amount of memory available on a system)
* String functions
    * `string.format`
    * `string.join`
    * `string.replace`
    * `string.split`
    * `string.to_lower`
    * `string.to_upper`
    * `string.trim`
    * `string.trim_prefix`
    * `string.trim_space`
    * `string.trim_suffix`
    * `string.contains` (**new**; check if a string contains a substring)

## Pros and cons

Pros:

* Aligns with how constants and component names are namespaced.
* Makes it clear which identifiers are standardized by looking for the
  dot-separated namespace.
* Namespaces enable having two functions with the same name in two different
  namespaces (`path.join` and `string.join`).

Cons:

* Deprecating 17 of the 19 identifiers may be seen as aggressive, even if they
  are never removed.
* Using the standard library becomes more verbose
    * However, some of the verbosity may be bought back in the future if
      [aliases](https://github.com/grafana/alloy/issues/154) are introduced.
* Namespaces in the standard library must not collide with a component
  namespace.

## Alternative solutions

### Separate namespace and identifier with underscore

Rather than separating the namespace and identifier using a dot
(`string.join`), an underscore could be used instead (`string_join`).

Pros:

* Clearer when something is in the standard library compared to being a
  reference to a component.
* Not possible for a standard library namespace to collide with a component
  namespace.

Cons:

* Inconsistent with `constants` which uses dot-separation for retrieving a
  specific constant.
    * This could be mitigated by changing `constants.os` to `constants_os` or
      similar.

### Do not standardize existing identifiers

Alternatively, a pattern for standardizing new identifiers could be
established, while existing identifiers are left untouched.

Pros:

* The majority of the existing standard library remains valid.

Cons:

* Until the number of new identifiers is far more than the number of current
  identifiers, the standard library will appear very inconsistent, with a heavy
  mix of namespaced and non-namespaced identifiers.
* An existing identifier may belong in a namespace alongside new identifiers,
  but is left out for backwards compatibility, causing inconsistencies.

### Do not establish a standardization for identifiers

Alternatively, we can omit standardizing identifiers at all, and continue to
let the standard library grow organically.

Pros:

* Less work for contributors :)

Cons:

* It will be harder to understand and navigate the standard library as the
  number of identifiers grows.

## Compatibility

This proposal is properly backwards compatible, with the old identifiers being
marked deprecated in favour of their namespaced equivalents (where a namespace
was introduced).

It is possible that any namespaces introduced in this proposal may collide with
an existing custom component. Custom components shadow the stdlib, so this is
not a breaking change, but users will have to change the name of their custom
components that collide with a stdlib namespace before being able to use
functions in that namespace.

By default, the removal deprecated identifiers would be considered for an
eventual 2.0 release, but they may be kept around longer based on usage.

## Implementation

The implementation of this proposal will be broken down into the following
steps:

1. Implement the new namespaced identifiers, and deprecate the old ones in
   documentation.

2. Update examples, tests, internal configs, and config converters to use the
   new namespaced identifiers.

2. Find a way to detect and report usage of deprecated identifiers to a user.

3. Add standard library usage stats, so usage of deprecated
   identifiers can be tracked over time, allowing maintainers to make an
   informed decision for if they can be removed alongside a major release.

Additionally, as this change may introduce new instances where custom component
names shadow a namespace, we should add a warning to notify the user of
shadowing (if detected).
