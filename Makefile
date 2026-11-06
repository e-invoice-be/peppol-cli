VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := -s -w \
	-X github.com/e-invoice-be/peppol-cli/internal/version.Version=$(VERSION) \
	-X github.com/e-invoice-be/peppol-cli/internal/version.Commit=$(COMMIT) \
	-X github.com/e-invoice-be/peppol-cli/internal/version.Date=$(DATE)

.PHONY: build test lint fmt vuln cover tidy-check check generate release-dry clean

GOLANGCI_LINT ?= golangci-lint
COVERAGE_MIN ?= 85

build:
	go build -ldflags "$(LDFLAGS)" -o peppol ./cmd/peppol/

test:
	go test ./... -race

lint:
	$(GOLANGCI_LINT) run ./...

fmt:
	$(GOLANGCI_LINT) fmt ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

cover:
	go test ./... -race -covermode=atomic -coverprofile=coverage.out
	./scripts/check-coverage.sh coverage.out $(COVERAGE_MIN)

tidy-check:
	go mod tidy -diff

# Same checks as CI Lint + Vuln + Test (coverage floor). CI also runs a separate Build matrix job.
check: tidy-check lint vuln cover

generate:
	./scripts/generate-client.sh

release-dry:
	goreleaser --snapshot --clean

clean:
	rm -f peppol
	rm -rf dist/ coverage.out coverage.html
