.PHONY: build test test-vet docker clean lint

TARGET=escalator
SRC_DIRS=pkg cmd
SOURCES=$(shell for dir in $(SRC_DIRS); do if [ -d $$dir ]; then find $$dir -type f -iname '*.go'; fi; done)
ENVVAR ?= CGO_ENABLED=0
ARCH=$(if $(TARGETPLATFORM),$(lastword $(subst /, ,$(TARGETPLATFORM))),amd64)

$(TARGET): $(SOURCES)
	$(ENVVAR) GOARCH=$(ARCH) go build -a -installsuffix cgo -o $(TARGET) cmd/main.go

build: $(TARGET)

test:
	go test ./... -cover -race

test-vet:
	go vet ./...

docker: Dockerfile
	docker buildx build --build-arg ENVVAR="$(ENVVAR)" -t atlassian/escalator --platform linux/$(ARCH) .

clean:
	rm -f $(TARGET)

lint:
	golangci-lint run
