VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "0.0.0-dev" | tr -d '\n')

.PHONY: build test

build: assets/busybox.tar.gz
	@echo "Build adcm-installer"
	@go build -o bin/adi -trimpath -ldflags "-X github.com/arenadata/adcm-installer/cmd.version=$(VERSION) -w -s" main.go

test:
	@go test -v ./...

assets/busybox.tar:
	@echo "Download Busybox image..."
	@mkdir -p assets
	@go run hack/busybox-image.go $@

assets/busybox.tar.gz: assets/busybox.tar
	@echo "Gzip Busybox image..."
	@gzip assets/busybox.tar -c > assets/busybox.tar.gz
