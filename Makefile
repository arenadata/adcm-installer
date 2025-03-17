VERSION ?= $(shell git describe --tags --abbrev=0 || echo "0.0.0-dev" | tr -d '\n')

.PHONY: build test

build:
	@go build -o bin/adi -trimpath -ldflags "-X github.com/arenadata/arenadata-installer/cmd.version=$(VERSION) -w -s" main.go

test:
	@go test -v ./...
