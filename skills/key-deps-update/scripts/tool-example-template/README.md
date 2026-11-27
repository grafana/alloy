## Overview

This is an example tool that can be used as a template for creating new tools.

Demonstrates standard Go tool structure with argument parsing, validation, and subprocess integration for calling external commands.

## Usage

`go run main.go --message "Your message here" --count 5`

**Flags:**

- `--message` (required): The message to print
- `--count` (optional): Number of times to print (default: 1)

## Output

Prints the provided message the specified number of times to stdout.

Example:

```bash
Hello World
Hello World
Hello World
```
