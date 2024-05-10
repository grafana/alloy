---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/path_join/
description: Learn about path_join
title: path_join
---

# path_join

`path_join` joins any number of path elements into a single path, separating them with an OS specific separator.

## Examples

```alloy
> path_join("this/is", "a/path")
"this/is/a/path"
> path_join("empty/path", "")
"empty/path"
> join("foo/", "/bar/", "foo/bar", "foo")
"foo/bar/foo/bar/foo"
```
