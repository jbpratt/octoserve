# Octoserve Makefile

.PHONY: build test clean test-conformance test-conformance-quick build-conformance clean-conformance help

# Default target
help:
	@echo "Octoserve - Minimal OCI Registry"
	@echo ""
	@echo "Available targets:"
	@echo "  build                 Build the octoserve binary"
	@echo "  test                  Run unit tests"
	@echo "  clean                 Clean build artifacts"
	@echo ""
	@echo "Conformance Testing:"
	@echo "  build-conformance     Build OCI conformance test binary"
	@echo "  test-conformance      Run full OCI conformance test suite"
	@echo "  test-conformance-quick Run quick conformance tests (pull/push only)"
	@echo "  clean-conformance     Clean conformance test artifacts"
	@echo ""
	@echo "Environment Variables for Conformance Tests:"
	@echo "  OCTOSERVE_PORT        Port for test server (default: 5000)"
	@echo "  OCI_DEBUG             Enable debug output (0 or 1)"
	@echo "  OCI_REPORT_DIR        Test results directory (default: test-results)"

# Build the main binary
build:
	@echo "Building octoserve..."
	@mkdir -p bin
	@go build -o bin/octoserve ./cmd/octoserve
	@echo "Build complete: bin/octoserve"

# Run unit tests
test:
	@echo "Running unit tests..."
	@go test ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@go clean

# Build conformance test binary
build-conformance:
	@echo "Building OCI conformance tests..."
	@./scripts/build-conformance.sh

# Run full conformance test suite
test-conformance: build build-conformance
	@echo "Running OCI Distribution Spec conformance tests..."
	@./scripts/test-conformance.sh

# Run quick conformance tests (pull and push only)
test-conformance-quick: build build-conformance
	@echo "Running quick OCI conformance tests (pull/push only)..."
	@OCI_TEST_CONTENT_DISCOVERY=0 OCI_TEST_CONTENT_MANAGEMENT=0 ./scripts/test-conformance.sh

# Clean conformance test artifacts
clean-conformance:
	@echo "Cleaning conformance test artifacts..."
	@rm -rf test-results/
	@rm -f distribution-spec/conformance/conformance.test
	@rm -rf test-data/
	@rm -f test-octoserve.pid