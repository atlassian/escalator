.PHONY: build setup test test-race test-vet docker

build:
	go build -o escalator cmd/main.go

setup:
	dep ensure -vendor-only

test:
	go test ./... -cover

test-race:
	go test ./... -cover -race

test-vet:
	go vet ./...

docker:
	docker build -t atlassian/escalator .
