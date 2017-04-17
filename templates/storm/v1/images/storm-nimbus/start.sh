#!/bin/sh

/configure.sh ${ZOOKEEPER_SERVICE_HOST:-$1}

exec bin/storm nimbus
