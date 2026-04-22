BINARY = sdt
MODULE = github.com/openshift/sdt
BUILD_DIR = bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

.PHONY: all build test lint clean run validate list review install

all: build

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/sdt

install:
	go install $(LDFLAGS) ./cmd/sdt

test:
	go test ./pkg/... -v -count=1

test-short:
	go test ./pkg/... -short -count=1

lint:
	go vet ./...

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

clean:
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY)

# Example commands
validate:
	go run ./cmd/sdt validate specs/examples/

list:
	go run ./cmd/sdt list specs/examples/

review:
	go run ./cmd/sdt review specs/examples/

run-dry:
	go run ./cmd/sdt run specs/examples/ --dry-run

# Cache management
cache-status:
	go run ./cmd/sdt cache status

cache-clear:
	go run ./cmd/sdt cache clear
