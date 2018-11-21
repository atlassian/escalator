.PHONY: build setup test test-race test-vet docker clean distclean fmt

TARGET=escalator
# E.g. set this to -v (I.e. GOCMDOPTS=-v via shell) to get the go command to be verbose
GOCMDOPTS?=
SRC_DIRS=pkg cmd
SOURCES=$(shell for dir in $(SRC_DIRS); do if [ -d $$dir ]; then find $$dir -type f -iname '*.go'; fi; done)

$(TARGET): vendor $(SOURCES)
	go build $(GOCMDOPTS) -o $(TARGET) cmd/main.go

build: $(TARGET)

setup: vendor

vendor: Gopkg.lock
	dep ensure -vendor-only $(GOCMDOPTS)

test: vendor
	go test ./... -cover

test-race: vendor
	go test ./... -cover -race

test-vet: vendor
	go vet ./...

docker: Dockerfile
	docker build -t atlassian/escalator .

clean:
	rm -f $(TARGET)

distclean: clean
	rm -rf vendor

Gopkg.lock: Gopkg.toml
	dep ensure -update $(GOCMDOPTS)

Gopkg.toml: $(SOURCES)
	@if ! dep check; then touch $@; fi;

fmt:
	gofmt -w $(SRC_DIRS)
