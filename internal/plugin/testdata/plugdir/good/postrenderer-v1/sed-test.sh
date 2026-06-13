#!/bin/sh
if [ $# -eq 0 ]; then
  sed s/FOOTEST/BARTEST/g <&0
else
  sed s/FOOTEST/"$*"/g <&0
fi
