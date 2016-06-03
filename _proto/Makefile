space := $(empty) $(empty)
comma := ,
empty :=

import_path = k8s.io/helm/pkg/proto/hapi

dst 	= ../pkg/proto
target  = go
plugins = grpc

chart_ias    = $(subst $(space),$(comma),$(addsuffix =$(import_path)/$(chart_pkg),$(addprefix M,$(chart_pbs))))
chart_pbs    = $(wildcard hapi/chart/*.proto)
chart_pkg    = chart

release_ias  = $(subst $(space),$(comma),$(addsuffix =$(import_path)/$(release_pkg),$(addprefix M,$(release_pbs))))
release_pbs  = $(wildcard hapi/release/*.proto)
release_pkg  = release

services_ias = $(subst $(space),$(comma),$(addsuffix =$(import_path)/$(services_pkg),$(addprefix M,$(services_pbs))))
services_pbs = $(wildcard hapi/services/*.proto)
services_pkg = services

google_deps	 = Mgoogle/protobuf/timestamp.proto=github.com/golang/protobuf/ptypes/timestamp,Mgoogle/protobuf/any.proto=github.com/golang/protobuf/ptypes/any

all: chart release services

chart:
	protoc --$(target)_out=plugins=$(plugins),$(google_deps),$(chart_ias):$(dst) $(chart_pbs)

release:
	protoc --$(target)_out=plugins=$(plugins),$(google_deps),$(chart_ias):$(dst) $(release_pbs)

services:
	protoc --$(target)_out=plugins=$(plugins),$(google_deps),$(chart_ias),$(release_ias):$(dst) $(services_pbs)

clean:
	@rm -rf $(dst)/hapi 2>/dev/null
