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
IFS=$'\n\t'

find_files() {
  find . -not \( \
    \( \
      -wholename './.git' \
      -o -wholename '*testdata*' \
      -o -wholename '*third_party*' \
    \) -prune \
  \) \
  \( -name '*.go' -o -name '*.sh' \)
}

# Disallow invisible or bidi Unicode codepoints in source files. These are
# usually invisible to reviewers but can hide behavior changes ("trojan
# source") or trip downstream scanners on consumers that vendor the module.
#
#   U+200B-U+200D  zero-width space / non-joiner / joiner
#   U+FEFF         byte order mark / zero-width no-break space
#   U+202A-U+202E  bidi explicit formatting
#   U+2066-U+2069  bidi isolates
matches=$(find_files | xargs perl -CSD -ne '
  if (/[\x{200B}-\x{200D}\x{FEFF}\x{202A}-\x{202E}\x{2066}-\x{2069}]/) {
    chomp;
    print "$ARGV:$.: $_\n";
  }
  close ARGV if eof;
' || :)

if [[ -n "$matches" ]]; then
  echo "Disallowed invisible or bidi Unicode codepoints found:"
  echo "$matches"
  exit 1
fi
