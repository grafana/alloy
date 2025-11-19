#!/usr/bin/env bash

set -eu
set -o pipefail

export SHELLOPTS

file="$1"

OBJDUMP_CMD=llvm-objdump
if ! type -p "${OBJDUMP_CMD}"; then
    OBJDUMP_CMD=llvm-objdump-17
fi

echo -e "\nInstruction counts for ${file}:\n"

total=0
while read line; do 
    name=$(echo $line | awk '{ print $2 }')
    size="0x$(echo $line | awk '{ print $3 }')"
    size=$((size / 8)) # ebpf has 64-bit fixed length instructions
    echo "$name has $size instructions"
    total=$((total + size))
done < <($OBJDUMP_CMD --section-headers "${file}" | grep TEXT)

echo -e "\nTotal instructions: ${total}\n"
