#!/usr/bin/env bash
set -euo pipefail

COVERDIR=${COVERDIR:-.coverage}
COVERMODE=${COVERMODE:-atomic}
PACKAGES=($(go list $(glide novendor)))

if [[ ! -d "$COVERDIR" ]]; then
  mkdir -p "$COVERDIR"
fi

echo "mode: ${COVERMODE}" > "${COVERDIR}/coverage.out"

for d in "${PACKAGES[@]}"; do
  go test -coverprofile=profile.out -covermode="$COVERMODE" "$d"
  if [ -f profile.out ]; then
    sed "/mode: $COVERMODE/d" profile.out >> "${COVERDIR}/coverage.out"
    rm profile.out
  fi
done

go tool cover -html "${COVERDIR}/coverage.out" -o "${COVERDIR}/coverage.html"
