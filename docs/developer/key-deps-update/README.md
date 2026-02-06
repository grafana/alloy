# Key dependency updates

In this directory you will find prompts and tools for the LLM agents to help with the process of updating the key dependencies of the Alloy project.

## How to use this

Use your preferred LLM agent and give it a prompt like this:

```text
Follow instructions in the key-dep-updates.md and update all the key dependencies.
```

Or if you want to update only one key dependency, you can try this:

```text
Follow instructions in the key-dep-updates.md and update only the Prometheus dependencies.
```

Or if you only want to execute some first steps, try this:

```text
Follow instructions in the key-dep-updates.md and execute only steps 1-3.
```

If your LLM agent supports planning mode, it may provide better results if you use it.

## Prompt guidelines

Below are some guidelines for writing and improving the LLM agent prompt for the key dependency updates process.

- We need to break the task down into manageable pieces and give the LLM an option to stop and ask for help if it cannot achieve a milestone.

- Try not to remove any special cases mentioned in the prompt. They were usually hand-crafted after the agent failed to notice certain cases. So they are there for a good reason.

- The prompt may seem at times repetitive or redundant, but that's intentional. LLMs often perform better when the instructions and context needed are in close proximity. We want to avoid references where the LLM needs to jump between different sections of the document. This reduces performance and accuracy.

## Tools and Snippets

- When all you need to provide is a snippet of a single command, best practice is to write it in the "Snippets" markdown section directly, especially if it's using commands that LLM will likely know how to use. This will require LLM to jump between the prompt and the snippets section, which is not ideal, but will work well as long as it's not becoming too complex.

- For more complex tools, multi-step commands, or things that can be reliably done with code, we should write a separate Go tool in `./tools` directory. Follow the example of existing tools. This way the LLM can discover and use the tools as needed, reducing its 'cognitive load' and improving performance. This approach is backed by [insights from Anthropic](https://www.anthropic.com/engineering/code-execution-with-mcp).

## Future improvement ideas

- See how we can get the upstreams to agree on the same versions first. For example, get Beyla and others to use the same OTel version as we want to use.

- We could adopt a naming convention for the branches or tags that are used for all the forks. For example, if we fork Prometheus 3.2.1, we could use the branch name `alloy-fork-3.2.1`. This way, we can prepare the forks in advance and let the LLM discover them.

- Make sure we have the issues or PRs that required us to fork in a comment in the go.mod file. This way, the LLM can check the status of the fork and determine if we still need it.

- We could start an update process by making sure the dependencies are ready first:
  - First we determine the versions we want to update to. We could publish them somewhere in a GitHub, so they can be referenced to by the LLM agents.
  - Then we go to key dependencies' repositories and update them to these new versions, for example, making sure that Beyla depends on the same version of OTel as the one we want to update to. This could be facilitated by a similar LLM prompt as the one we have here, with any customisations needed.
  - Once the key dependencies are on the same page, we can update them in Alloy.

- For now we are telling the LLM not to worry about Prometheus exporters forks, becuase there are too many of them now and we know the progress to upstream the required changes is often stalled. We can re-enable this in the future as we further improve the upgrade process.

