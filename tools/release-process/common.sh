function confirm_with_user() {
  set +x
  local prompt="$1"

  printf "\n\n$prompt\n\n"
  read -r -p "Answer Y/N: " CONFIRMATION

  set -x
  if [[ ! $CONFIRMATION =~ ^[Yy]$ ]]; then
    echo "Confirmation failed. Exiting..."
    exit 1
  fi
}

function check_env_var_exists() {
  local var_name="$1"
  if [[ -z "${!var_name}" ]]; then
    echo "ERROR: missing environment variable ${var_name}"
    print_help_message
    exit 1
  else
    echo "${var_name} confirmed to exist and set to '${!var_name}'"
  fi
}

function check_on_branch() {
    branch="$1"
    if [[ "$(git branch --show-current)" != "$branch" ]]; then
      echo "ERROR: You are not on the 'main' branch. Please switch to 'main' branch."
      exit 1
    fi
}

function verify_remote_exists() {
    if ! git remote get-url "${1}" > /dev/null 2>&1; then
        echo "Error: Remote '${1}' is not defined. This script requires it to exist and point to Alloy repo."
        exit 1
    fi
}
