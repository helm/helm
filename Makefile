GO ?= go

PKG       := $(shell glide novendor)
TAGS      :=
TESTS     := .
TESTFLAGS :=
LDFLAGS   :=

BINARIES := helm tiller

.PHONY: all
all: build

.PHONY: build
build:
	@for i in $(BINARIES); do \
		$(GO) build -o ./bin/$$i -v $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' ./cmd/$$i || exit 1; \
	done

.PHONY: test
test: TESTFLAGS += -race -v
test: test-style
test: test-unit

.PHONY: test-unit
test-unit:
	$(GO) test $(GOFLAGS) -run $(TESTS) $(PKG) $(TESTFLAGS)

.PHONY: test-style
test-style:
	@scripts/validate-go.sh

.PHONY: clean
clean:
	rm -rf ./bin

.PHONY: coverage
coverage:
	@scripts/coverage.sh

.PHONY: bootstrap
bootstrap:
	glide install

