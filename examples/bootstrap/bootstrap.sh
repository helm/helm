#!/bin/bash
KUBECTL=`which kubectl`
if [[ -z $KUBECTL ]] ; then
  echo Cannot find kubectl
  exit 1
fi

echo "Starting resourcifier..."
RESOURCIFIER=`which resourcifier`
if [[ -z $RESOURCIFIER ]] ; then
	echo Cannot find resourcifier
	exit 1
fi
pkill -f $RESOURCIFIER
$RESOURCIFIER > resourcifier.log 2>&1 --kubectl=$KUBECTL --port=8082 &
echo

echo "Starting expandybird..."
EXPANDYBIRD=`which expandybird`
if [[ -z $EXPANDYBIRD ]] ; then
  echo Cannot find expandybird
  exit 1
fi
pkill -f $EXPANDYBIRD
$EXPANDYBIRD > expandybird.log 2>&1 --port=8081 --expansion_binary=expandybird/expansion/expansion.py &
echo

echo "Starting deployment manager..."
MANAGER=`which manager`
if [[ -z $MANAGER ]] ; then
  echo Cannot find manager
  exit 1
fi
pkill -f $MANAGER
$MANAGER > manager.log 2>&1 --port=8080 --expanderURL=http://localhost:8081 --deployerURL=http://localhost:8082 &
echo

echo "Starting kubectl proxy..."
pkill -f "$KUBECTL proxy"
$KUBECTL proxy --port=8001 &
sleep 1s
echo

echo "Done."
