---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/fmt/
description: Learn about the fmt command
labels:
  stage: general-availability
  products:
    - oss
title: fmt
weight: 200
---

# `fmt`

The `fmt` command formats a given {{< param "PRODUCT_NAME" >}} configuration file.

## Usage

```shell
alloy fmt [<FLAG> ...] <FILE_NAME>
```

Replace the following:

* _`<FLAG>`_: One or more flags that define the input and output of the command.
* _`<FILE_NAME>`_: The {{< param "PRODUCT_NAME" >}} configuration file.

If the _`<FILE_NAME>`_ argument isn't provided or if the _`<FILE_NAME>`_ argument is equal to `-`, `fmt` formats the contents of standard input.
Otherwise, `fmt` reads and formats the file from disk specified by the argument.

The `--write` flag can be specified to replace the contents of the original file on disk with the formatted results.
`--write` can only be provided when `fmt` isn't reading from standard input.

The `--test` flag can be specified to test if the contents of the file are formatted correctly.

The `--write` and `--test` flags are mutually exclusive.

The command fails if the file being formatted has syntactically incorrect {{< param "PRODUCT_NAME" >}} configuration, but doesn't validate whether {{< param "PRODUCT_NAME" >}} components are configured properly.

The following flags are supported:

* `--write`, `-w`: Write the formatted file back to disk when not reading from standard input.
* `--test`, `-t`: Only test the input and return a non-zero exit code if changes would have been made.
