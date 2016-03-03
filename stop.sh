#!/bin/bash

echo "Stopping resourcifier..."
RESOURCIFIER=`which resourcifier`
if [[ ! -z $RESOURCIFIER ]] ; then
	pkill -f $RESOURCIFIER
fi
echo

echo "Stopping expandybird..."
EXPANDYBIRD=`which expandybird`
if [[ ! -z $EXPANDYBIRD ]] ; then
	pkill -f $EXPANDYBIRD
fi
echo

echo "Stopping deployment manager..."
MANAGER=`which manager`
if [[ ! -z $MANAGER ]] ; then
	pkill -f $MANAGER
fi
echo

echo "Done."
