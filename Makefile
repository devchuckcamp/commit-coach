.PHONY: build test test-race test-coverage clean help lint vet install uninstall

BINARY_NAME=commit-coach
MAIN_PKG=./cmd/aicommits

help:
	@echo "commit-coach Makefile targets:"
	@echo "  build          - Build the binary"
	@echo "  install        - Install commit-coach to ~/.local/bin (Bash)"
	@echo "  uninstall      - Remove commit-coach from ~/.local/bin (Bash)"
	@echo "  test           - Run tests"
	@echo "  test-race      - Run tests with race detector"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  clean          - Clean build artifacts"
	@echo "  lint           - Run linter (requires golangci-lint)"
	@echo "  vet            - Run go vet"

build:
	go build -o $(BINARY_NAME) ./

install:
	bash ./scripts/install.sh

uninstall:
	bash ./scripts/uninstall.sh

test:
	go test -v ./...

test-race:
	go test -race -v ./...

test-coverage:
	go test -cover -v ./... && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

clean:
	rm -f $(BINARY_NAME) coverage.out

lint:
	golangci-lint run ./...

vet:
	go vet ./...

install-deps:
	go mod download
	go mod tidy

run: build
	./$(BINARY_NAME)

.DEFAULT_GOAL := help
