#!/usr/bin/env bash

# Bash 'Strict Mode'
# http://redsymbol.net/articles/unofficial-bash-strict-mode
set -euo pipefail
IFS=$'\n\t'

# Helper Functions -------------------------------------------------------------

# Display error message and exit
error_exit() {
  echo "error: ${1:-"unknown error"}" 1>&2
  exit 1
}

# Checks if a command exists.  Returns 1 or 0
command_exists() {
  hash "${1}" 2>/dev/null
}

# Program Functions ------------------------------------------------------------

verify_prereqs() {
  echo "Verifying Prerequisites...."
  if command_exists gsutil; then
    echo "Thumbs up! Looks like you have gsutil. Let's continue."
  else
    error_exit "Couldn't find gsutil. Bailing out."
  fi
}

confirm() {
  case $response in
    [yY][eE][sS]|[yY])
      true
      ;;
    *)
      false
      ;;
  esac
}

# Main -------------------------------------------------------------------------

main() {
  if [ "$#" -ne 2 ]; then
    error_exit "Illegal number of parameters. You must pass in local directory path and a GCS bucket name"
  fi

  echo "Getting ready to sync your local directory ($1) to a remote repository at gs://$2"

  verify_prereqs

  # dry run of the command
  gsutil rsync -d -n $1 gs://$2

  read -p "Are you sure you would like to continue with these changes? [y/N]} " confirm
  if [[ $confirm =~ [yY](es)* ]]; then
    gsutil rsync -d $1 gs://$2
  else
    error_exit "Discontinuing sync process."
  fi

  echo "Your remote chart repository now matches the contents of the $1 directory!"

}

main "${@:-}"
