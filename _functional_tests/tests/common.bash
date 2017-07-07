#!/usr/bin/env bash

function install_chart {
  helm install tests/$1 -n $1 --namespace $NAMESPACE
}

function uninstall_chart {
  helm delete $1
}

