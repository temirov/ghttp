GO_SOURCES := $(shell find . -name '*.go' -not -path "./vendor/*")
UNIT_PACKAGES := $(shell go list ./... | grep -v '/tests$$')
INTEGRATION_TEST_PATTERN := Integration
RELEASE_TARGETS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
RELEASE_DIRECTORY := dist
RELEASE_BINARY_NAME := ghttp

.PHONY: format check-format lint test test-unit test-integration build release ci

format:
	gofmt -w $(GO_SOURCES)

check-format:
	@formatted_files="$$(gofmt -l $(GO_SOURCES))"; \
	if [ -n "$$formatted_files" ]; then \
		echo "Go files require formatting:"; \
		echo "$$formatted_files"; \
		exit 1; \
	fi

lint:
	go vet ./...

test-unit:
	go test $(UNIT_PACKAGES)

test-integration:
	go test ./... -run $(INTEGRATION_TEST_PATTERN)

test: test-unit test-integration

build:
	mkdir -p bin
	go build -o bin/ghttp .

release:
	rm -rf $(RELEASE_DIRECTORY)
	mkdir -p $(RELEASE_DIRECTORY)
	for target in $(RELEASE_TARGETS); do \
		os=$${target%/*}; \
		arch=$${target#*/}; \
		output_path=$(RELEASE_DIRECTORY)/$(RELEASE_BINARY_NAME)-$$os-$$arch; \
		echo "Building $$output_path"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o $$output_path .; \
	done

ci: check-format lint test
