.PHONY: build
build:
	CGO_ENABLED=1 go build -o escalator cmd/main.go

.PHONY: docker-build
docker-build:
	docker build -t atlassian/escalator .

.PHONY: setup
setup:
	go get -u github.com/golang/dep/cmd/dep
	dep ensure

.PHONY: test
test:
	go test ./... -cover
