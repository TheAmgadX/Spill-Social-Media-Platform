.PHONY: help run seed verify test test-unit test-e2e lint fmt build clean

APP_NAME := spill-api
CMD_PATH := ./cmd/main.go

help: ## Show available targets
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

run: ## Run the API server
	go run $(CMD_PATH)

seed: ## Seed database with sample data
	go run ./cmd/seed/main.go

verify: ## Run verification script
	go run ./cmd/verify/main.go

test: test-unit ## Run default test suite

test-unit: ## Run unit tests (excluding e2e)
	go test $$(go list ./... | grep -v '/tests/e2e') -v

test-e2e: ## Run end-to-end tests (requires Docker for testcontainers)
	go test ./tests/e2e/... -v

fmt: ## Format Go code
	gofmt -s -w .

lint: ## Run go vet static analysis
	go vet ./...

build: ## Build binary into ./bin
	mkdir -p bin
	go build -o bin/$(APP_NAME) $(CMD_PATH)

clean: ## Remove build artifacts
	rm -rf bin