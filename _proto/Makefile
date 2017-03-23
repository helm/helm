space := $(empty) $(empty)
comma := ,
empty :=

import_path = k8s.io/helm/pkg/proto/hapi
go_path := $(shell go env GOPATH)
googleapis_dir := $(go_path)/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis

dst 	= ../pkg/proto
target  = go
plugins = grpc

chart_ias    = $(subst $(space),$(comma),$(addsuffix =$(import_path)/$(chart_pkg),$(addprefix M,$(chart_pbs))))
chart_pbs    = $(sort $(wildcard hapi/chart/*.proto))
chart_pkg    = chart

release_ias  = $(subst $(space),$(comma),$(addsuffix =$(import_path)/$(release_pkg),$(addprefix M,$(release_pbs))))
release_pbs  = $(sort $(wildcard hapi/release/*.proto))
release_pkg  = release

services_ias = $(subst $(space),$(comma),$(addsuffix =$(import_path)/$(services_pkg),$(addprefix M,$(services_pbs))))
services_pbs = $(sort $(wildcard hapi/services/*.proto))
services_pkg = services

version_ias    = $(subst $(space),$(comma),$(addsuffix =$(import_path)/$(version_pkg),$(addprefix M,$(version_pbs))))
version_pbs    = $(sort $(wildcard hapi/version/*.proto))
version_pkg    = version

google_deps	 = Mgoogle/protobuf/timestamp.proto=github.com/golang/protobuf/ptypes/timestamp,Mgoogle/protobuf/any.proto=github.com/golang/protobuf/ptypes/any

.PHONY: all
all: chart release services version

chart:
	PATH=../bin:$(PATH) protoc -I . -I $(googleapis_dir) --$(target)_out=plugins=$(plugins),$(google_deps),$(chart_ias):$(dst) $(chart_pbs)

release:
	PATH=../bin:$(PATH) protoc -I . -I $(googleapis_dir) --$(target)_out=plugins=$(plugins),$(google_deps),$(chart_ias),$(version_ias):$(dst) $(release_pbs)

services:
	PATH=../bin:$(PATH) protoc -I . -I $(googleapis_dir) --$(target)_out=plugins=$(plugins),$(google_deps),$(chart_ias),$(version_ias),$(release_ias):$(dst) $(services_pbs)
	PATH=../bin:$(PATH) protoc -I . -I $(googleapis_dir) --grpc-gateway_out=logtostderr=true,$(google_deps),$(chart_ias),$(version_ias),$(release_ias):$(dst) $(services_pbs)

version:
	PATH=../bin:$(PATH) protoc -I . -I $(googleapis_dir) --$(target)_out=plugins=$(plugins),$(google_deps):$(dst) $(version_pbs)

.PHONY: clean
clean:
	@rm -rf $(dst)/hapi 2>/dev/null
