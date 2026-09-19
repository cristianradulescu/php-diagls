GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test

BINARY_NAME=php-diagls

# Tools image (docker/Dockerfile). Any of the *_VERSION variables, UID and
# GID can be set on the command line to pin them, e.g.
#   make docker-build MAGO_VERSION=1.49.0 PHPSTAN_VERSION=2.2.14
DOCKER_IMAGE ?= php-diagls-tools
DOCKER_BUILD_ARGS = PHP_VERSION COMPOSER_VERSION PHP_CS_FIXER_VERSION PHPSTAN_VERSION MAGO_VERSION UID GID

all: build

build:
	$(GOBUILD) -o $(BINARY_NAME) ./main.go

test:
	$(GOTEST) -v ./...

docker-build:
	docker build $(foreach arg,$(DOCKER_BUILD_ARGS),$(if $($(arg)),--build-arg $(arg)=$($(arg)))) -t $(DOCKER_IMAGE) docker/

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

.PHONY: all build test clean docker-build
