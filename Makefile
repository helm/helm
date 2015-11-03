SUBDIRS := expandybird/. resourcifier/. manager/. client/.
TARGETS := all build test push container clean

SUBDIRS_TARGETS := \
	$(foreach t,$(TARGETS),$(addsuffix $t,$(SUBDIRS)))

GO_DEPS := github.com/kubernetes/deployment-manager/util/... github.com/kubernetes/deployment-manager/version/... github.com/kubernetes/deployment-manager/expandybird/... github.com/kubernetes/deployment-manager/resourcifier/... github.com/kubernetes/deployment-manager/manager/... github.com/kubernetes/deployment-manager/client/...

.PHONY : all build test clean $(TARGETS) $(SUBDIRS_TARGETS) .project .docker

all: build

clean:
	go clean -v $(GO_DEPS)

test: build
	-go test -v $(GO_DEPS)

build:
	go get -v $(GO_DEPS)
	go install -v $(GO_DEPS)

push: container

container: .project .docker

.project:
	@if [[ -z "${PROJECT}" ]]; then echo "PROJECT variable must be set"; exit 1; fi

.docker:
	@if [[ -z `which docker` ]] || ! docker version &> /dev/null; then echo "docker is not installed correctly"; exit 1; fi

$(TARGETS) : % : $(addsuffix %,$(SUBDIRS))

$(SUBDIRS_TARGETS) :
	$(MAKE) -C $(@D) $(@F:.%=%)
