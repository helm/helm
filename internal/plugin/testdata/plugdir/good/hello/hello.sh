#!/bin/bash

echo "Hello from a Helm plugin"

echo "PARAMS"
echo $*

$HELM_BIN ls --all

