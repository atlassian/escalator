.PHONY: build setup test test-race

build:
	CGO_ENABLED=1 go build -o escalator cmd/main.go

setup:
	dep ensure

test:
	go test ./... -cover

test-race:
	go test ./... -cover -race
