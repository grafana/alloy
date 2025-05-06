#!/usr/bin/env bash

# Get the script directory
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
echo The script directory is $SCRIPT_DIR

for dir in $SCRIPT_DIR/tests/*/
do
	dir=${dir%*/}
    echo Running the tests in $dir
	cd $dir && $(GO_ENV) go test -timeout 1h
done
