IMAGE_NAME := docker.atl-paas.net/kitt/escalator
GIT_HASH := $(shell git rev-parse --short HEAD)
WORKDIR := ${CURDIR}

build:
	docker build --no-cache -t $(IMAGE_NAME):$(GIT_HASH) .

push: build
	docker push $(IMAGE_NAME):$(GIT_HASH)
	echo $(IMAGE_NAME):$(GIT_HASH) is READDY.