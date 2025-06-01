# Mukhostho DB Makefile

# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod

# Binary name
BINARY_NAME = mukhostho
BINARY_PATH = ./cmd/mukhostho/main.go

# Build directory
BUILD_DIR = ./build

# Test directories
TEST_DIRS = ./test ./internal/...

# Default target
.PHONY: all
all: clean deps test build

# Clean build artifacts
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	mkdir -p $(BUILD_DIR)

# Download dependencies
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Run tests
.PHONY: test
test:
	$(GOTEST) -v $(TEST_DIRS)

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out $(TEST_DIRS)
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
.PHONY: bench
bench:
	$(GOTEST) -bench=. -benchmem $(TEST_DIRS)

# Run race condition tests
.PHONY: test-race
test-race:
	$(GOTEST) -race -v $(TEST_DIRS)

# Build the application
.PHONY: build
build:
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(BINARY_PATH)

# Build for multiple platforms
.PHONY: build-all
build-all:
	# Linux amd64
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(BINARY_PATH)
	# Windows amd64
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(BINARY_PATH)
	# macOS amd64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(BINARY_PATH)
	# macOS arm64
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(BINARY_PATH)

# Build with optimization flags
.PHONY: build-release
build-release:
	$(GOBUILD) -ldflags "-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(BINARY_PATH)

# Run the application
.PHONY: run
run: build
	$(BUILD_DIR)/$(BINARY_NAME)

# Run with custom config
.PHONY: run-config
run-config: build
	$(BUILD_DIR)/$(BINARY_NAME) -config custom-config.json

# Development mode with auto-restart
.PHONY: dev
dev:
	@which air > /dev/null || (echo "Installing air..." && $(GOGET) github.com/cosmtrek/air@latest)
	air

# Format code
.PHONY: fmt
fmt:
	$(GOCMD) fmt ./...

# Lint code
.PHONY: lint
lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2)
	golangci-lint run

# Check for security vulnerabilities
.PHONY: security
security:
	@which gosec > /dev/null || (echo "Installing gosec..." && $(GOGET) github.com/securecodewarrior/gosec/v2/cmd/gosec@latest)
	gosec ./...

# Generate documentation
.PHONY: docs
docs:
	@which godoc > /dev/null || (echo "Installing godoc..." && $(GOGET) golang.org/x/tools/cmd/godoc@latest)
	@echo "Starting documentation server at http://localhost:6060"
	godoc -http=:6060

# Docker build
.PHONY: docker-build
docker-build:
	docker build -t mukhostho-db:latest .

# Docker run
.PHONY: docker-run
docker-run:
	docker run -p 8080:8080 -v $(PWD)/data:/app/data mukhostho-db:latest

# Load test using curl
.PHONY: load-test
load-test:
	@echo "Running basic load test..."
	@for i in {1..1000}; do \
		curl -s -X POST http://localhost:8080/api/v1/keys/test_$$i \
			-H "Content-Type: application/json" \
			-d '{"value": "test_value_'$$i'"}' > /dev/null; \
	done
	@echo "Load test completed - 1000 keys inserted"

# Performance test
.PHONY: perf-test
perf-test:
	@echo "Running performance tests..."
	$(GOTEST) -bench=BenchmarkStore -benchtime=10s -benchmem ./test

# Memory profiling
.PHONY: profile-mem
profile-mem: build
	$(BUILD_DIR)/$(BINARY_NAME) -cpuprofile=cpu.prof -memprofile=mem.prof &
	sleep 5
	curl -X POST http://localhost:8080/api/v1/keys/profile_test -d '{"value": "test"}' -H "Content-Type: application/json"
	pkill $(BINARY_NAME)
	$(GOCMD) tool pprof mem.prof

# CPU profiling
.PHONY: profile-cpu
profile-cpu: build
	$(BUILD_DIR)/$(BINARY_NAME) -cpuprofile=cpu.prof &
	sleep 5
	curl -X POST http://localhost:8080/api/v1/keys/profile_test -d '{"value": "test"}' -H "Content-Type: application/json"
	pkill $(BINARY_NAME)
	$(GOCMD) tool pprof cpu.prof

# Check if server is running
.PHONY: health-check
health-check:
	@curl -s http://localhost:8080/health | jq .

# Install development tools
.PHONY: install-tools
install-tools:
	$(GOGET) github.com/cosmtrek/air@latest
	$(GOGET) golang.org/x/tools/cmd/godoc@latest
	$(GOGET) github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2

# Full CI pipeline
.PHONY: ci
ci: deps fmt lint security test test-race build

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all          - Clean, download deps, test, and build"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Download dependencies"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  test-race    - Run tests with race detection"
	@echo "  bench        - Run benchmarks"
	@echo "  build        - Build the application"
	@echo "  build-all    - Build for multiple platforms"
	@echo "  build-release- Build with optimizations"
	@echo "  run          - Build and run the application"
	@echo "  run-config   - Run with custom config file"
	@echo "  dev          - Development mode with auto-restart"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code"
	@echo "  security     - Check for security vulnerabilities"
	@echo "  docs         - Start documentation server"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo "  load-test    - Run basic load test"
	@echo "  perf-test    - Run performance tests"
	@echo "  profile-mem  - Memory profiling"
	@echo "  profile-cpu  - CPU profiling"
	@echo "  health-check - Check server health"
	@echo "  install-tools- Install development tools"
	@echo "  ci           - Full CI pipeline"
	@echo "  help         - Show this help"
