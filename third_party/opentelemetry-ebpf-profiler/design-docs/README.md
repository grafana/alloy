Profiling agent design documents
================================

This directory contains design documents that discuss major additions of changes
to the profiling agent's architecture.

### When to write a design document?

Invoking the design document process can be beneficial if

- you are proposing changes that have significant impact on multiple
  sub-components of the profiling agent.
- you are proposing a major addition and want to make sure that it will
  actually be accepted upstream the way that you plan to implement it without
  writing the code first (and thus risking need for major reworks later).
- your proposed changes require a significant amount of context to understand
  for reviewers. A short design document can help to clarify the current state
  and the state that you'd like to move towards in these cases.
- you'd like to incrementally apply reworks over the course of multiple
  PRs, to provide extra context for reviewers to understand what the end
  goal is supposed to look like. In simpler cases that can also be achieved
  with a tracking issue.

The above are guidelines: there is no hard rule on when a design document is
necessary. When in doubt, please feel free to create an issue and quickly outline
what you want to do in order to clarify whether a document is needed or not!

### Creating a New Document

- Create a new git branch forking from latest `main`
- Create a directory in the `design-docs` folder
  - The directory name should follow a format like `00000-my-doc-title`
  - The intention for creating a directory per document is to allow bundling
    media (images, diagrams, drawings) with the document
  - The 5 digit number is included to make it easy to spot recent documents and
    to order the documents by their recentness when viewed in the GitHub UI
  - When initially writing the document, the ID is set to `00000`
- Copy one of the templates for design documents from the (`00000-templates`)[./00000-templates]
  directory into into the newly created directory, naming it `README.md`
- Write the design document by following the instructions in the template
- Once reaching draft quality, create a draft PR
  - Add the `design-document` label
  - Include a link to the rendered document. GitHub hides away the ability to
    properly view rendered documents behind multiple clicks, so adding a direct
    link spares others some work. After creating the PR, this link can be
    obtained by clicking "Files changed", searching the `README.md` file,
    clicking the three dots on the right, selecting "View file" and then
    switching the current view to track your branch via the drop-down to the
    left of the file path. The last step ensures that the link automatically
    tracks updates when you push fixup commits later.
- Rename the directory according to the PR ID that was allocated, e.g. if the
  allocated PR ID is 1234 then `00000-my-doc-title` becomes
  `01234-my-doc-title`
- Once you're happy with the state of the document:
    - Mark the PR as ready for review and tag at least 2 maintainers for review
    - Additional people who should be aware of the document but whose review
      isn't mandatory can be notified by mentioning their username in the PR's
      description as `CC @username`
