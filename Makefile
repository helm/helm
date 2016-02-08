ifndef GOPATH
$(error No GOPATH set)
endif

BIN_DIR := bin
DIST_DIR := _dist
GO_PACKAGES := cmd/helm dm deploy format kubectl
MAIN_GO := github.com/deis/helm-dm/cmd/helm
HELM_BIN := helm-dm
PATH_WITH_HELM = PATH="$(shell pwd)/$(BIN_DIR)/helm:$(PATH)"

VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null)+$(shell git rev-parse --short HEAD)

export GO15VENDOREXPERIMENT=1

ifndef VERSION
  VERSION := git-$(shell git rev-parse --short HEAD)
endif

build:
	go build -o bin/${HELM_BIN} -ldflags "-s -X main.version=${VERSION}" $(MAIN_GO)

bootstrap:
	go get -u github.com/golang/lint/golint github.com/mitchellh/gox
	glide install

build-all:
	gox -verbose \
	-ldflags "-X main.version=${VERSION}" \
	-os="linux darwin " \
	-arch="amd64 386" \
	-output="$(DIST_DIR)/{{.OS}}-{{.Arch}}/{{.Dir}}" .

clean:
	rm -rf $(DIST_DIR) $(BIN_DIR)

dist: build-all
	@cd $(DIST_DIR) && \
	find * -type d -exec zip -jr helm-$(VERSION)-{}.zip {} \; && \
	cd -

install: build
	install -d ${DESTDIR}/usr/local/bin/
	install -m 755 bin/${HELM_BIN} ${DESTDIR}/usr/local/bin/${HELM_BIN}

quicktest:
	$(PATH_WITH_HELM) go test -short $(addprefix ./,$(GO_PACKAGES))

test: test-style
	$(PATH_WITH_HELM) go test -v -cover $(addprefix ./,$(GO_PACKAGES))

test-style:
	@if [ $(shell gofmt -e -l -s *.go $(GO_PACKAGES)) ]; then \
		echo "gofmt check failed:"; gofmt -e -l -s *.go $(GO_PACKAGES); exit 1; \
	fi
	@for i in . $(GO_PACKAGES); do \
		golint $$i; \
	done
	@for i in . $(GO_PACKAGES); do \
		go vet github.com/deis/helm-dm/$$i; \
	done

.PHONY: bootstrap \
				build \
				build-all \
				clean \
				dist \
				install \
				quicktest \
				test \
				test-charts \
				test-style
