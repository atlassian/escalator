IMAGE_NAME := docker.atl-paas.net/kitt/escalator
GIT_HASH := $(shell git rev-parse --short HEAD)
WORKDIR := ${CURDIR}

local-build:
	go build cmd/main.go
	mv main escalator

docker-build:
	docker build --no-cache -t $(IMAGE_NAME):$(GIT_HASH) .

docker-push: build
	docker push $(IMAGE_NAME):$(GIT_HASH)
	echo $(IMAGE_NAME):$(GIT_HASH) is READDY.