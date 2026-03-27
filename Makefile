VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := -s -w \
	-X github.com/e-invoicebe/peppol-cli/internal/version.Version=$(VERSION) \
	-X github.com/e-invoicebe/peppol-cli/internal/version.Commit=$(COMMIT) \
	-X github.com/e-invoicebe/peppol-cli/internal/version.Date=$(DATE)

.PHONY: build test lint generate release-dry clean

build:
	go build -ldflags "$(LDFLAGS)" -o peppol ./cmd/peppol/

test:
	go test ./... -race

lint:
	golangci-lint run

generate:
	./scripts/generate-client.sh

release-dry:
	goreleaser --snapshot --clean

clean:
	rm -f peppol
	rm -rf dist/
