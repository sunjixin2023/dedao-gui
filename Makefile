SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

PLATFORM ?= auto
VERSION ?=
RELEASE_ARGS ?=

.PHONY: help bootstrap run build release deploy clean

help:
	@echo "dedao quick commands"
	@echo "  make bootstrap                 # install/verify dependencies"
	@echo "  make run                       # run app in dev mode"
	@echo "  make build                     # one-click build for current host + package artifact"
	@echo "  make release PLATFORM=<target> # build for target platform + package artifact"
	@echo "  make deploy VERSION=vX.Y.Z     # create & push git tag to trigger release workflow"
	@echo "  make clean                     # remove build output"
	@echo ""
	@echo "release target examples:"
	@echo "  auto (default), darwin/universal, darwin/arm64, darwin/amd64, windows/amd64, linux/amd64"
	@echo ""
	@echo "optional RELEASE_ARGS examples:"
	@echo "  RELEASE_ARGS='--skip-install'"
	@echo "  RELEASE_ARGS='--no-package'"

bootstrap:
	@bash ./scripts/bootstrap.sh

run:
	@wails dev

build:
	@bash ./scripts/release.sh auto $(RELEASE_ARGS)

release:
	@bash ./scripts/release.sh $(PLATFORM) $(RELEASE_ARGS)

deploy:
	@if [[ -z "$(VERSION)" ]]; then echo "VERSION is required, example: make deploy VERSION=v1.0.0"; exit 1; fi
	@bash ./scripts/deploy.sh $(VERSION)

clean:
	@rm -rf ./build/bin
	@echo "cleaned ./build/bin"
