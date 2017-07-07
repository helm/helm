#!/usr/bin/env bash

source "common.bash"

set -e -o pipefail

install_chart chart200
sleep 10
uninstall_chart chart200
