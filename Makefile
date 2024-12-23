.PHONY: build

build:
	@go build -ldflags='-w -s' -o bin/adcm main.go
