VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "0.0.0-dev" | tr -d '\n')
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED := 0

ifeq ($(GOOS),darwin)
	CGO_ENABLED = 1
endif

.PHONY: build linux in-docker test

build: assets/busybox.tar
	@echo "Build adcm-installer"
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o bin/adi -trimpath -ldflags "-X github.com/arenadata/adcm-installer/cmd.version=$(VERSION) -w -s" main.go

linux:
	$(MAKE) GOOS=linux GOARCH=amd64 build

in-docker:
	@docker run -w /app --rm -it -v $(HOME)/go/pkg/mod:/go/pkg/mod -v `pwd`:/app golang:1.24 make linux

test:
	@go test -v ./...

assets/busybox.tar:
	@echo "Download Busybox image..."
	@go run hack/busybox-image.go "busybox:stable-uclibc" $@
