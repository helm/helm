#!/bin/sh

cat >> conf/storm.yaml <<EOF
storm.local.hostname: "$(hostname -I)"
EOF

cat >> conf/storm.yaml <<EOF
storm.local.dir: "/tmp"
EOF

if [ -n "$1" ]; then
   cat >> conf/storm.yaml <<EOF
storm.zookeeper.servers:
- "$1"
EOF
fi

if [ -n "$2" ]; then
   cat >> conf/storm.yaml <<EOF
nimbus.host: "$2"
EOF
fi

cat conf/storm.yaml
