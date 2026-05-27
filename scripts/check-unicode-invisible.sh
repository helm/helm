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

# check-unicode-invisible.sh detects invisible Unicode codepoints in Go source
# files that are harmless to a compiler but invisible to reviewers:
#
#   U+200B–U+200D  zero-width space / non-joiner / joiner  (ZWSP class)
#   U+FEFF         byte-order mark / zero-width no-break space (BOM)
#
# The bidichk golangci-lint linter covers the bidirectional/trojan-source
# range (U+202A–U+202E, U+2066–U+2069). This script fills the gap for the
# ZWSP and BOM codepoints that bidichk does not report.

set -euo pipefail
IFS=$'\n\t'

find_go_files() {
  # Prune common vendored/generated directories by *directory name* to avoid
  # accidentally excluding files that merely contain the substring.
  find . \
    \( -type d \( -name .git -o -name testdata -o -name third_party -o -name vendor \) -prune \) \
    -o \( -type f -name '*.go' -print0 \)
}

FILELIST=$(mktemp "${TMPDIR:-/tmp}/check-unicode-files.XXXXXX")
FINDERR=$(mktemp "${TMPDIR:-/tmp}/check-unicode-finderr.XXXXXX")
trap 'rm -f "$FILELIST" "$FINDERR"' EXIT

if ! find_go_files >"$FILELIST" 2>"$FINDERR"; then
  echo "ERROR: failed to enumerate Go source files." >&2
  cat "$FINDERR" >&2 || true
  exit 2
fi

# Fail loudly on any traversal errors (permission issues, broken symlinks, etc.)
# rather than silently passing with an incomplete file list.
if [[ -s "$FINDERR" ]]; then
  echo "ERROR: errors occurred while enumerating Go source files:" >&2
  cat "$FINDERR" >&2 || true
  exit 2
fi

# If this script is run from the wrong directory, we may find no Go files.
# Treat that as an error so CI/local runs do not silently pass.
if [[ ! -s "$FILELIST" ]]; then
  echo "ERROR: no Go source files found (are you running from the repo root?)" >&2
  exit 2
fi

# Run the checker separately from find so enumeration failures are not
# misreported as Unicode detections or Python errors. Exit codes:
#   0  – no invisible characters found
#   1  – one or more invisible characters detected
#   2  – unexpected OS/tool error (file unreadable, etc.)
rc=0
python3 - "$FILELIST" <<'PYEOF' || rc=$?
import sys

# Invisible codepoints to detect (ZWSP class + BOM). Bidi/trojan-source chars
# (U+202A-U+202E, U+2066-U+2069) are handled by the bidichk golangci-lint linter.
INVISIBLE = frozenset('\u200b\u200c\u200d\ufeff')

try:
    with open(sys.argv[1], 'rb') as listfh:
        paths = listfh.read().split(b'\0')
except OSError as exc:
    print('error: could not read file list: {}'.format(exc), file=sys.stderr)
    sys.exit(2)

found = False
for raw in paths:
    if not raw:  # trailing NUL produces an empty token
        continue
    path = raw.decode('utf-8', errors='surrogateescape')
    try:
        with open(path, encoding='utf-8', errors='strict') as fh:
            for lineno, text in enumerate(fh, 1):
                hits = sorted({c for c in text if c in INVISIBLE})
                if hits:
                    names = ', '.join('U+{:04X}'.format(ord(c)) for c in hits)
                    print('{}:{}: invisible Unicode character(s) found: {}'.format(
                        path, lineno, names))
                    found = True
    except UnicodeDecodeError as exc:
        print('error: could not decode {} as UTF-8: {}'.format(path, exc), file=sys.stderr)
        sys.exit(2)
    except OSError as exc:
        print('error: could not read {}: {}'.format(path, exc), file=sys.stderr)
        sys.exit(2)

sys.exit(1 if found else 0)
PYEOF

case $rc in
  0) ;;
  1)
    echo "FAIL: invisible Unicode character(s) (ZWSP/BOM) detected in Go source."
    echo "Remove or replace the offending characters and re-run 'make test-unicode-invisible'."
    exit 1
    ;;
  2)
    echo "ERROR: unicode check failed while reading Go source files."
    echo "Ensure all Go source files are readable and try again."
    exit 2
    ;;
  *)
    echo "ERROR: unicode check failed with unexpected exit code $rc."
    echo "Ensure python3 is installed and try again."
    exit "$rc"
    ;;
esac
