.PHONY: build
build:
	CGO_ENABLED=1 go build -o escalator cmd/main.go

.PHONY: setup
setup:
	dep ensure

.PHONY: test
test:
	go test ./... -cover
