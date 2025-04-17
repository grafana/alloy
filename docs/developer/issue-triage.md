# Managing issues

We value community engagement with Alloy and want to encourage community members and all users to contribute to Alloy's
development by raising issues for bugs, feature requests, proposals, and more. To ensure that issues are appropriately
reviewed and users get the feedback they desire we have a few patterns for issue triaging.

## Issue triage process

Issues that are created in the Alloy repository should all begin with the `needs-triage` label.
They may also start with other labels like `type/docs`, `proposal`, or `bug` depending on the template selected.

The goal of the triage process is to collect enough information to move the issue into the right category or categories
where it can then be most easily tracked to completion. The triage process might include asking clarifying questions of the author,
researching the behavior of Alloy or other technologies relevant to the issue, and reproducing an issue in a test environment.

### Issue states and labels

After an effort has been made to triage an issue, the issue should be in one of several states

* Waiting for author
  * An issue might be waiting on more feedback from its author if
    * There was insuffucient information available to reproduce the issue
    * There was insufficient information available to fully define the feature requested
    * An answer to the author's problem was proposed using existing functionality
  * These issues should be tagged `waiting-for-author` in addition to any other categorizing tags (`bug`, `enhancement`, etc)
* Waiting for codeowner
  * Some issues related to community components within Alloy will be dependent on the community maintainer of the component
  * These issues should be tagged `waiting-for-codeowner`
  * These issues should retain the `needs-triage` label until the codeowner has responded
* Ready to implement/fix/document
  * An issue is ready to implement or fix if the scope is well understood, and if it is an issue it should be replicatable
  * These issues should be tagged `bug`, `enhancement`, `type/docs`, or `flaky-test`
  * If the issue should be in the next release, it should be tagged `release-blocker`
  * If the issue is a good candidate for a first time contributor or another interested community member, it should be tagged `good first issue`
  * If the issue is a good candidate for a larger investment by an interested community member, it should be tagged `help wanted`
  * *These issues should no longer have the `needs-triage` label*
* Closed
  * An issue might be closed after triage if
    * a solution was offered, the issue was labelled `waiting-for-author`, and the author confirmed their need was met
    * there is an existing duplicate issue with sufficient context to resolve the issue
    * the issue was already solved (there should be a duplicate closed issue in most cases, link to it)
    * based on discussion, the issue should be re-opened as a new `proposal` based on concensus in the issue comments
  * It's unlikely an issue will be closed after first triage, unless it doesn't meet community standards.

### Additional labels

The `needs-attention` label is applied to issues that are seen as stale in a github action.
This includes issues that have not been interacted with in 90 days.
Issues with the `needs-attention` label may be closed if they are not in an actionable state.  
The `keepalive` label can be applied to exempt an issue or pull request from being marked as stale.

There are a variety of other labels that can be applied to issues and pull requests to help provide context to the issue. Wherever possible, relevant labels should be applied.

* The `component-request` label can help identify requests for new components.
* The `os:windows` should be used when changes are relevant to the Windows OS.
Adding the label to a pull request will trigger the full suite of tests on Windows on a pull request.
At this time there are no other OS-based labels.
* There are various `dependencies` and language (`go`, `javascript`, etc) labels that may be applied by bots.
* Component labels like `prometheus.remote_write` should be applied whenever possible. These labels should all be generated (but not automatically applied) by a github action.
* The `v2.0-breaking-change` label may be applied if the issue represents a breaking change that will need to be delayed until Alloy v2.x.

## Community Members

Community members and other interested parties are welcome to help triage issues by investigating the root cause of bugs, adding input for
features they would like to see, or participating in design discussions.

If you are looking to contribute a pull request for an issue that has been triaged, please comment on the issue and request
it to be assigned to you! A maintainer will set the assignment when they are able.