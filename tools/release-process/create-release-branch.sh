#!/bin/bash
set -x -e

script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$script_dir/common.sh"

function print_help_message() {
  set +x
  echo "USAGE: ${0}"
  echo "Required environment variables:"
  echo "  - VERSION_PREFIX is the v<MAJOR>.<MINOR>, for example v1.5"
  echo "  - VERSION is the v<MAJOR>.<MINOR>.<PATCH>, for example v1.5.1"
  echo "  - COMMIT_SHA is the commit sha that we want to release"
  set -x
}

check_env_var_exists "VERSION"
check_env_var_exists "VERSION_PREFIX"
check_env_var_exists "COMMIT_SHA"

pushd "$(git rev-parse --show-toplevel)"

  check_on_branch "main"

  git pull

  git checkout "${COMMIT_SHA}" > /dev/null 2>&1
  git rev-parse --verify "${COMMIT_SHA}"

  verify_remote_exists "origin"

  RELEASE_BRANCH_NAME="release/${VERSION_PREFIX}"

  if [[ -n "$(git branch -r --list "origin/${RELEASE_BRANCH_NAME}")" ]]; then
    echo "Branch ${RELEASE_BRANCH_NAME} already exists in the upstream."
    exit 1
  fi

  echo "Branch ${RELEASE_BRANCH_NAME} doesn't exist in the upstream yet. Creating..."
  git checkout -b "${RELEASE_BRANCH_NAME}"
  git push origin "${RELEASE_BRANCH_NAME}"
  echo "DONE"

  git checkout main # Go back to where we were.
popd
