#!/bin/bash

echo "Stopping resourcifier..."
RESOURCIFIER=bin/resourcifier
if [[ ! -z $RESOURCIFIER ]] ; then
	pkill -f $RESOURCIFIER
fi
echo

echo "Stopping expandybird..."
EXPANDYBIRD=bin/expandybird
if [[ ! -z $EXPANDYBIRD ]] ; then
	pkill -f $EXPANDYBIRD
fi
echo

echo "Stopping deployment manager..."
MANAGER=bin/manager
if [[ ! -z $MANAGER ]] ; then
	pkill -f $MANAGER
fi
echo

echo "Done."
