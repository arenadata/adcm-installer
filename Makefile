VERSION ?= $(shell git describe --tags --abbrev=0 || echo "dev" | tr -d '\n')

.PHONY: build test

build:
	@go build -o bin/adi -ldflags "-X github.com/arenadata/arenadata-installer/cmd.version=$(VERSION) -w -s" main.go

test:
	@go test -v ./...
