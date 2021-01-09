PACKAGE_NAME  := github.com/laher/servicetray
.DEFAULT_GOAL := help

.PHONY: build
build: ## build via docker
	docker build .

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
