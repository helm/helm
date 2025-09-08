
.DEFAULT: build
.PHONY: build test vet

.PHONY: plugin.wasm
plugin.wasm:
	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .

build: plugin.wasm

vet:
	GOOS=wasip1 GOARCH=wasm go vet ./...
