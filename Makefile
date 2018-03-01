IMAGE_NAME := docker.atl-paas.net/kitt/escalator
GIT_HASH := $(shell git rev-parse --short HEAD)
WORKDIR := ${CURDIR}

local-build: escalator
	go build -o escalator cmd/main.go

.PHONY: docker-build
docker-build:
	docker build --no-cache -t $(IMAGE_NAME):$(GIT_HASH) .

.PHONY: docker-push
docker-push: docker-build
	docker push $(IMAGE_NAME):$(GIT_HASH)
	echo $(IMAGE_NAME):$(GIT_HASH) is READY.

.PHONY: test
test:
	go test ./... -cover
