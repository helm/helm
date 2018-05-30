#!/usr/bin/env bash

set -e -o pipefail

source "common.bash"

install_chart chart221
sleep 30
uninstall_chart chart221
