#!/usr/bin/env bash

# Copyright 2016 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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
