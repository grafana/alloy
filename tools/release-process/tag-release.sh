#!/bin/bash
set -e

script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$script_dir/common.sh"

check_env_var_exists "VERSION"
check_env_var_exists "VERSION_PREFIX"

confirm_with_user "All required commits for the release should exist on the release branch. This includes functionality and documentation such as the CHANGELOG.md. All versions in code should have already been updated. Do you confirm this is completed?"

verify_remote_exists "origin"

RELEASE_BRANCH_NAME="release/${VERSION_PREFIX}"

git checkout "${RELEASE_BRANCH_NAME}"
check_on_branch "${RELEASE_BRANCH_NAME}"

git pull origin "${RELEASE_BRANCH_NAME}"
git diff --name-only "origin/${RELEASE_BRANCH_NAME}...HEAD"

GPG_TTY=$(tty) git tag -s "$VERSION"

confirm_with_user "Tag for ${VERSION} successful on ${RELEASE_BRANCH_NAME}. Push to origin?"

git push origin "$VERSION"
