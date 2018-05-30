#!/usr/bin/env bash

set -e -o pipefail

helm create test110
helm package --version 1.0.0 test110

expect="test110-1.0.0.tgz"
if [ ! -f $expect ]; then
  echo "Cannot find $expect"
  ls -1
  exit 1
fi
