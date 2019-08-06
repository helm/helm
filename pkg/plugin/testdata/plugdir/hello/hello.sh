#!/bin/bash

echo "Hello from a Helm plugin"

echo "PARAMS"
echo $*

echo "ENVIRONMENT"
echo $TILLER_HOST
echo $HELM_PATH_CONFIG

$HELM_BIN --host $TILLER_HOST ls --all

