#!/usr/bin/env bash

# Copyright The Helm Authors.
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

set -euo pipefail

covermode=${COVERMODE:-atomic}
coverdir=$(mktemp -d /tmp/coverage.XXXXXXXXXX)
trap 'rm -rf "${coverdir}"' EXIT
profile="${coverdir}/cover.out"
html=false
target="./..." # by default the whole repository is tested
for arg in "$@"; do
  case "${arg}" in
    --html)
      html=true
      ;;
    *)
      target="${arg}"
      ;;
  esac
done

generate_cover_data() {
  for d in $(go list "$target"); do
    (
      local output="${coverdir}/${d//\//-}.cover"
      go test -coverprofile="${output}" -covermode="$covermode" "$d"
    )
  done

  echo "mode: $covermode" >"$profile"
  grep -h -v "^mode:" "$coverdir"/*.cover >>"$profile"
}

generate_cover_data
go tool cover -func "${profile}"

if [ "${html}" = "true" ] ; then
    go tool cover -html "${profile}"
fi

