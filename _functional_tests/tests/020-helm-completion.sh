#!/usr/bin/env bash

set -e

# test that command works
helm completion bash > /dev/null
helm completion zsh > /dev/null

# test that generate code is readable by Bash
#source < (helm completion bash)
