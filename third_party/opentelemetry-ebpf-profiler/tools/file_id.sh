#!/usr/bin/env bash

set -eu

# This script computes the FileID as we do in Go code.

# We must resolve symlinks before we can lookup the size of the file via `stat`
file=$(readlink -f "$1")
filesize=$(stat --printf="%s" "$file")

hash=$(cat \
    <(head -c 4096 "$file") \
    <(tail -c 4096 "$file") \
    <(printf $(printf "%.16x" $filesize | sed 's/\(..\)/\\x\1/g')) \
    | sha256sum)

echo "$hash" | head -c 32
