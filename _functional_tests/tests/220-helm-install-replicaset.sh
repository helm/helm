#!/usr/bin/env bash

set -e -o pipefail

source "common.bash"

install_chart chart220
sleep 30
uninstall_chart chart220
