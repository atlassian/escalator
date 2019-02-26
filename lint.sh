#!/bin/bash

# Use Goreturns as a linter to pick up spacing formatting. Return exit code 1 if there are any errors

OUTPUT="$(goreturns -d -e cmd/ pkg/)"
test -z "$OUTPUT" || (>&2 echo "$OUTPUT") && exit 1
