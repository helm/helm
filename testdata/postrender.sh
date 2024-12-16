#!/bin/sh

s=$(tee | sed 's/5/25/')

echo "${s}"