.PHONY: info
info:
	@echo "Build tag: ${TAG}"
	@echo "Registry: ${DOCKER_REGISTRY}"
	@echo "Project: ${PROJECT}"
	@echo "Image: ${IMAGE}"

TAG ?= $(shell echo `date +"%s"`_`date +"%N"`)
	