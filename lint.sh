#!/bin/bash

# Use Goreturns as a linter to pick up spacing formatting. Return exit code 1 if there are any errors
# Runs gofmt + goimports
# go returns
OUTPUT="$(goreturns -d -e cmd/ pkg/)"
test -z "$OUTPUT" || ((>&2 echo -e "$OUTPUT" "\ntry running 'make fmt'") && exit 1)

# golint
golint -set_exit_status ./cmd/...
golint -set_exit_status ./pkg/...
