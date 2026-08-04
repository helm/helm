#!/bin/sh
if [ $# -eq 0 ]; then
  sed s/BARTEST/BAZTEST/g <&0
else
  sed s/BARTEST/"$*"/g <&0
fi
