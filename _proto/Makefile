space := $(empty) $(empty)
comma := ,
empty :=

import_path = k8s.io/helm/pkg/proto/hapi

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

rudder_ias = $(subst $(space),$(comma),$(addsuffix =$(import_path)/$(rudder_pkg),$(addprefix M,$(rudder_pbs))))
rudder_pbs = $(sort $(wildcard hapi/rudder/*.proto))
rudder_pkg = rudder

version_ias    = $(subst $(space),$(comma),$(addsuffix =$(import_path)/$(version_pkg),$(addprefix M,$(version_pbs))))
version_pbs    = $(sort $(wildcard hapi/version/*.proto))
version_pkg    = version

google_deps	 = Mgoogle/protobuf/timestamp.proto=github.com/golang/protobuf/ptypes/timestamp,Mgoogle/protobuf/any.proto=github.com/golang/protobuf/ptypes/any

.PHONY: all
all: chart release services rudder version

.PHONY: chart
chart:
	PATH=../bin:$(PATH) protoc --$(target)_out=plugins=$(plugins),$(google_deps),$(chart_ias):$(dst) $(chart_pbs)

.PHONY: release
release:
	PATH=../bin:$(PATH) protoc --$(target)_out=plugins=$(plugins),$(google_deps),$(chart_ias),$(version_ias):$(dst) $(release_pbs)

.PHONY: services
services:
	PATH=../bin:$(PATH) protoc --$(target)_out=plugins=$(plugins),$(google_deps),$(chart_ias),$(version_ias),$(release_ias):$(dst) $(services_pbs)

.PHONY: rudder
rudder:
	PATH=../bin:$(PATH) protoc --$(target)_out=plugins=$(plugins),$(google_deps),$(chart_ias),$(version_ias),$(release_ias):$(dst) $(rudder_pbs)

.PHONY: version
version:
	PATH=../bin:$(PATH) protoc --$(target)_out=plugins=$(plugins),$(google_deps):$(dst) $(version_pbs)

.PHONY: clean
clean:
	@rm -rf $(dst)/hapi 2>/dev/null
