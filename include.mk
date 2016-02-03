.PHONY: test-unit
test-unit:
	@echo Running tests...
	go test -v ./...

.PHONY: lint
lint:
	@echo Running golint...
	golint ./...

.PHONY: vet
vet:
	@echo Running go vet...
	go vet ./...

.PHONY: setup-gotools
setup-gotools:
	@echo Installing golint
	go get -u github.com/golang/lint/golint
	@echo Installing vet
	go get -u -v golang.org/x/tools/cmd/vet
