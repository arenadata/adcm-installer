.PHONY: build test

build:
	@go build -ldflags='-w -s' -o bin/adcm main.go

test:
	@go test -v ./...
