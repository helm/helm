#!/bin/bash

echo "Hello from a Helm plugin"

echo "PARAMS"
echo $*

echo "ENVIRONMENT"
echo $TILLER_HOST
echo $HELM_HOME

$HELM_BIN --host $TILLER_HOST ls --all

