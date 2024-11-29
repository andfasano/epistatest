#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

VERSION=v1.62.2
PATH=$PATH:$(go env GOPATH)/bin
if ! command -v golangci-lint; then
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin $VERSION
fi

golangci-lint version
golangci-lint run "$@"
