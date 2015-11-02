# Makefile for the Docker image gcr.io/$(PROJECT)/expandybird
# MAINTAINER: Jack Greenfield <jackgr@google.com>
# If you update this image please check the tag value before pushing.

.PHONY : all build test push container clean

test: client
	client --action=expand test/guestbook.yaml test/replicatedservice.py test/redis.jinja > /dev/null

client:
	go get -v ./...
	go install -v ./...
