.PHONY: build test test-vet docker clean lint

TARGET=escalator
# E.g. set this to -v (I.e. GOCMDOPTS=-v via shell) to get the go command to be verbose
GOCMDOPTS?=
SRC_DIRS=pkg cmd
SOURCES=$(shell for dir in $(SRC_DIRS); do if [ -d $$dir ]; then find $$dir -type f -iname '*.go'; fi; done)

$(TARGET): $(SOURCES)
	go build $(GOCMDOPTS) -o $(TARGET) cmd/main.go

build: $(TARGET)

test:
	go test ./... -cover -race

test-vet:
	go vet ./...

docker: Dockerfile
	docker build -t atlassian/escalator .

clean:
	rm -f $(TARGET)

lint:
	golangci-lint run
