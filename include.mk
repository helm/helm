.PHONY: info
info:
	@echo "Build tag: ${TAG}"
	@echo "Registry: ${DOCKER_REGISTRY}"
	@echo "Project: ${PROJECT}"
	@echo "Image: ${IMAGE}"

.PHONY: test-unit
test-unit:
	@echo Running tests...
	go test -v ./...

.PHONY: lint
lint:
	@echo Running golint...
	golint ./...
	@echo -----------------

.PHONY: vet
vet:
	@echo Running go vet...
	go vet ./...
	@echo -----------------

.PHONY: setup-gotools
setup-gotools:
	@echo Installing golint
	go get -u github.com/golang/lint/golint
	@echo Installing vet
	go get -u -v golang.org/x/tools/cmd/vet
