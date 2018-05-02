.PHONY: build setup test test-race test-vet

build:
	CGO_ENABLED=1 go build -o escalator cmd/main.go

setup:
	dep ensure

test:
	go test ./... -cover

test-race:
	go test ./... -cover -race

test-vet:
	go vet ./...

docker:
	docker build -t atlassian/escalator .
