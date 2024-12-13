---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/file/
description: Learn about file functions
menuTitle: file
title: file
---

# file

The `file` namespace contains functions related to files.

## file.path_join

The `file.path_join` function joins any number of path elements into a single path, separating them with an OS-specific separator.

### Examples

```alloy
> file.path_join()
""

> file.path_join("this/is", "a/path")
"this/is/a/path"
```
