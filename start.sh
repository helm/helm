#!/bin/bash

LOGDIR=log
if [[ ! -d $LOGDIR ]]; then
  mkdir $LOGDIR
fi

KUBECTL=`which kubectl`
if [[ -z $KUBECTL ]] ; then
  echo Cannot find kubectl
  exit 1
fi

echo "Starting resourcifier..."
RESOURCIFIER=bin/resourcifier
if [[ -z $RESOURCIFIER ]] ; then
	echo Cannot find resourcifier
	exit 1
fi
pkill -f $RESOURCIFIER
nohup $RESOURCIFIER > $LOGDIR/resourcifier.log 2>&1 --kubectl=$KUBECTL --port=8082 &
echo

echo "Starting expandybird..."
EXPANDYBIRD=bin/expandybird
if [[ -z $EXPANDYBIRD ]] ; then
  echo Cannot find expandybird
  exit 1
fi
pkill -f $EXPANDYBIRD
nohup $EXPANDYBIRD > $LOGDIR/expandybird.log 2>&1 --port=8081 --expansion_binary=expansion/expansion.py &
echo

echo "Starting deployment manager..."
MANAGER=bin/manager
if [[ -z $MANAGER ]] ; then
  echo Cannot find manager
  exit 1
fi
pkill -f $MANAGER
nohup $MANAGER > $LOGDIR/manager.log 2>&1 --port=8080  --kubectl=$KUBECTL --expanderURL=http://localhost:8081 --deployerURL=http://localhost:8082 &
echo

echo "Starting kubectl proxy..."
pkill -f "$KUBECTL proxy"
nohup $KUBECTL proxy --port=8001 &
sleep 1s
echo

echo "Done."
