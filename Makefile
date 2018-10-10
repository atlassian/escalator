.PHONY: build setup test test-race test-vet docker clean distclean

TARGET=escalator
# E.g. set this to -v (I.e. GOCMDOPTS=-v via shell) to get the go command to be verbose
GOCMDOPTS?=
SOURCES=$(shell for dir in pkg cmd; do if [ -d $$dir ]; then find $$dir -type f -iname '*.go'; fi; done)

$(TARGET): vendor $(SOURCES)
	go build $(GOCMDOPTS) -o $(TARGET) cmd/main.go

build: $(TARGET)

setup: vendor

vendor: Gopkg.lock Gopkg.toml
	dep ensure -vendor-only $(GOCMDOPTS)

test:
	go test ./... -cover

test-race:
	go test ./... -cover -race

test-vet:
	go vet ./...

docker: Dockerfile
	docker build -t atlassian/escalator .

clean:
	rm -f $(TARGET)

distclean: clean
	rm -rf vendor
