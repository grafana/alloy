# Handling Breaking Changes

## Before you begin

* Make sure you've read [the backwards-compatibility document][backwards-compatibility-doc].
* Write down in what way your change will break users. Think of cases where the
  upgrade of Alloy to a new version would require a manual step beyond just
  replacing the binary to keep all the functionality working.

## Considerations

Use the list of considerations below to generate ideas / approaches on how the
change could be handled:

* **Can we make this change non-breaking in Alloy code?**

  Sometimes it's possible to make the change non-breaking with some extra
  effort. For example:

    * If a metric that is used on our official dashboard was renamed, we can add
      an OR to a query and the dashboard can support both: new and old metric.
    * If a CLI flag or a config option was renamed, we can add an alias that
      allows us to use both names.

  **NOTE:** If you choose this option, we may still want to make the breaking
  change in the next major
  release. [Make sure that you correctly track it][tracking-breaking-changes].

* **Is this change covered by our backwards-compatibility RFC guarantees?**
    * **Is this change out-of-scope of our guarantees?**
        * For example: metrics that are not used on our official dashboards.
    * **Is this change called out as an exception?**
        * For example: non-stable functionality or breaking changes in the
          upstream.

  **NOTE:** If the change is not a breaking change by our definition, but it
  would still cause breakage and/or frustration with our users, we may still
  communicate it to the users. See the [communicating the breaking changes section][communicating].

* **Can this change wait for the next major release?**

  Sometimes changes are urgent, e.g. security fixes or users blocked on them,
  but sometimes they can wait for the next major release. Also, consider how far
  in the future is the next major release.

  **NOTE:** If you do pick this option, make sure you
  correctly [track the work necessary for the next major release][tracking-breaking-changes].

* **Can we fork / update an existing fork to make this change non-breaking?**

  Sometimes it may be preferable to bring back the old behaviour or handle the
  breaking change in a backwards-compatible way in the upstream library.
  Consider this as an option.

  **NOTE:** If you choose this option, we may still want to make the breaking
  change in the next major
  release. [Make sure that you correctly track it][tracking-breaking-changes].

* **Should we first deprecate / give a warning and make the breaking change
  later?**

  It may be a good idea for certain breaking changes to give a heads-up / add a
  warning in one release and make the breaking change later. Consider this as an
  option.

* **Do we have an idea how widely the impacted feature is used and how many
  users will be impacted?**

  Sometimes this can help decide which approach to take.

* **Do we need to discuss this with maintainers / a wider community?**

  If there is no exception, no way to make the change non-breaking and we cannot
  wait for the next major release, we may need to discuss this further and
  consider options not listed here.

## Decide the best approach

We'd typically prefer options in the following order:

1. Not make a breaking change at all - via code change in Alloy or in a fork or
   upstream
2. If not possible, we'd prefer to wait for the next major release
3. If not possible, we'd consider using our backwards-compatibility scope
   definition / exceptions / out-of-scope options
4. If not possible, we'd likely need a wider discussion about this change and
   communicate with users

## Communicating the breaking changes

Currently, we use our CHANGELOG.md to communicate the presence of breaking
changes. They are also included on a GitHub release page under the Breaking
Changes section. For changes that require manual steps to migrate, we must also
include a migration guide.

## Tracking work that needs to be done for the next major release

If we need to plan some work for the next major release, it's essential to track
it correctly, so it's not lost, and we don't miss the rare opportunity to make
some breaking changes.

We currently use GitHub issues assigned to 2.0 milestone to track the issues
planned for the next release.

[tracking-breaking-changes]: #tracking-work-that-needs-to-be-done-for-the-next-major-release
[backwards-compatibility-doc]: https://grafana.com/docs/alloy/latest/introduction/backward-compatibility/
[communicating]: #communicating-the-breaking-changes
