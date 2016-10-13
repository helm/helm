#!/bin/sh
helm package hashtest
shasum -a 256 hashtest-1.2.3.tgz > hashtest.sha256
