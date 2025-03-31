VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "0.0.0-dev" | tr -d '\n')

.PHONY: build test

build: assets/busybox.tar.gz
	@echo "Build adcm-installer"
	@go build -o bin/adi -trimpath -ldflags "-X github.com/arenadata/adcm-installer/cmd.version=$(VERSION) -w -s" main.go

test:
	@go test -v ./...

bin/skopeo:
	@echo "Clone sources and Build Skopeo..."
	@git clone https://github.com/containers/skopeo
	@cd skopeo && make bin/skopeo
	@mv skopeo/bin .

assets/busybox.tar: bin/skopeo
	@echo "Download Busybox image..."
	@mkdir -p assets
	@skopeo copy --override-os=linux --override-arch=amd64 docker://docker.io/library/busybox:stable-uclibc docker-archive:$@:busybox:stable-uclibc

assets/busybox.tar.gz: assets/busybox.tar
	@echo "Gzip Busybox..."
	@gzip assets/busybox.tar -c > assets/busybox.tar.gz
