#!/bin/bash

echo "Stopping resourcifier..."
RESOURCIFIER=bin/resourcifier
pkill -f $RESOURCIFIER || echo "Resourcifier is not running"

echo "Stopping expandybird..."
EXPANDYBIRD=bin/expandybird
pkill -f $EXPANDYBIRD || echo "Expandybird is not running"

echo "Stopping deployment manager..."
MANAGER=bin/manager
pkill -f $MANAGER || echo "Manager is not running"
