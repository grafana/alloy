# Majore dependency updates notes

This file contains some notes on the major dependency updates process for the Alloy maintainers.

## Prompt guidelines

Below are some guidelines for writing and improving the LLM agent prompt for the major dependency updates process.

- Try not to remove any special cases mentioned in the prompt. They were usually hand-crafted after the agent failed to notice certain cases. So they are there for a good reason.

- The prompt may seem at times repetitive or redundant, but that's intentional. LLMs often perform better when the instructions and context needed are in close proximity.

- We want to avoid references where the LLM needs to jump between different sections of the document. This reduces performance and accuracy. The only exception is the "Tools" section.

- We need to break the task down into manageable pieces and give the LLM an option to stop and ask for help if it cannot achieve a milestone.
  
## Future improvement ideas

- See how we can get the upstreams to agree on the same versions first. For example, get Beyla and others to use the same OTel version as we want to use.

- Build one uber-tool that given a PR or issue or commit sha or release version, find all the information we may need?

- We could adopt a naming convention for the branches or tags that are used for all the forks. For example, if we fork Prometheus 3.2.1, we could use the branch name `alloy-fork-3.2.1`. This way, we can prepare the forks in advance and let the LLM discover them.

- Make sure we have the issues or PRs that required us to fork in a comment in the go.mod file. This way, the LLM can check the status of the fork and determine if we still need it.

- We could start an update process by making sure the dependencies are ready first:
  - First we determine the versions we want to update to. We could publish them somewhere in a GitHub, so they can be referenced to by the LLM agents.
  - Then we go to major dependencies and update them to these new versions, for example, making sure that Beyla depends on the same version of OTel as the one we want to update to. This could be facilitated by a similar LLM prompt as the one we have here, with any customisations needed.
  - Once the major dependencies are on the same page, we can update them in Alloy.

- For now we are telling the LLM not to worry about Prometheus exporters forks, becuase there are too many
  of them now and we know the progress to upstream the required changes is stalled. We can re-enable this in the future as we further improve the upgrade process.
