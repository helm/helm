#!/usr/bin/env bash
set -euo pipefail

readonly  reset=$(tput sgr0)
readonly    red=$(tput bold; tput setaf 1)
readonly  green=$(tput bold; tput setaf 2)
readonly yellow=$(tput bold; tput setaf 3)

exit_code=0

find_go_files() {
  find . -type f -name "*.go" | grep -v vendor
}

hash golint 2>/dev/null || go get -u github.com/golang/lint/golint

echo "==> Running golint..."
for pkg in $(glide nv); do
  if golint_out=$(golint "$pkg" 2>&1); then
    echo "${yellow}${golint_out}${reset}"
  fi
done

echo "==> Running go vet..."
echo -n "$red"
go vet $(glide nv) 2>&1 | grep -v "^exit status " || exit_code=${PIPESTATUS[0]}
echo -n "$reset"

echo "==> Running gofmt..."
failed_fmt=$(find_go_files | xargs gofmt -s -l)
if [[ -n "${failed_fmt}" ]]; then
  echo -n "${red}"
  echo "gofmt check failed:"
  echo "$failed_fmt"
  gofmt -s -d "${failed_fmt}"
  echo -n "${reset}"
  exit_code=1
fi

exit ${exit_code}
