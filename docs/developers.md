# Developers Guide

This guide explains how to set up your environment for developing on
Helm and Tiller.

## Prerequisites

Tiller uses gRPC. To get started with gRPC, you will need to...

- Install `protoc` for compiling protobuf files. Releases are
  [here](https://github.com/google/protobuf/releases)
- Install the protoc Go plugin: `go get -u github.com/golang/protobuf/protoc-gen-go`

Note that you need to be on protobuf 3.x (`protoc --version`) and use the latest Go plugin.

## The Helm API (HAPI)

We use gRPC as an API layer. See `pkg/hapi` for the generated Go code,
and `_proto` for the protocol buffer definitions.

To regenerate `hapi`, use `go generate pkg/hapi`.
