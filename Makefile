.PHONY: build setup test test-race test-vet docker clean distclean fmt lint

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

# goreturns runs both gofmt and goimports.
# This is used to pickup more comphrehnsive formatting/codestyle changes
# https://github.com/sqs/goreturns
fmt: vendor
	goreturns -w pkg/ cmd/

# the linting also uses goreturns.
# the lint.sh script reports formatting changes/errors
lint: vendor
	./lint.sh
