MODULE   := github.com/dshills/atlas
BIN_DIR  := ./bin
BIN_NAME := atlas
CMD      := ./cmd/atlas

.PHONY: help build install test lint clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build to ./bin
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BIN_NAME) $(CMD)

install: ## Install to $$GOPATH/bin
	go install $(CMD)

test: ## Run all tests
	go test ./...

test-race: ## Run all tests with race detector
	go test -race ./...

lint: ## Run golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
