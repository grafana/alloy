#!/bin/sh
ALLOY_VERSION=$(sed -e '/^#/d' -e '/^$/d' VERSION | tr -d '\n')

if [ -z "$ALLOY_VERSION" ]; then
    echo "ALLOY_VERSION can't be found. Are you running this from the repo root?"
    exit 1
fi

versionMatcher='^v[0-9]+\.[0-9]+\.[0-9]$'

if ! echo "$ALLOY_VERSION" | grep -Eq "$versionMatcher"; then
    echo "ALLOY_VERSION env var is not in the correct format. It should be in the format of vX.Y.Z"
    exit 1
fi

templates=$(find . -type f -name "*.t" -not -path "./.git/*")
for template in $templates; do
    echo "Generating ${template%.t}"
    sed -e "s/\$ALLOY_VERSION/$ALLOY_VERSION/g" < "$template" > "${template%.t}"
done
