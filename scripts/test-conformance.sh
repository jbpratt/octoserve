#!/bin/bash

# OCI Distribution Spec Conformance Test Runner
# This script starts octoserve, runs the conformance tests, and cleans up

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
OCTOSERVE_PORT=${OCTOSERVE_PORT:-5000}
OCTOSERVE_HOST=${OCTOSERVE_HOST:-localhost}
OCTOSERVE_URL="http://${OCTOSERVE_HOST}:${OCTOSERVE_PORT}"
TEST_NAMESPACE=${OCI_NAMESPACE:-"test/repo"}
CROSSMOUNT_NAMESPACE=${OCI_CROSSMOUNT_NAMESPACE:-"test/other"}
TEST_RESULTS_DIR=${OCI_REPORT_DIR:-"test-results"}
OCTOSERVE_DATA_DIR="./test-data"
OCTOSERVE_PID_FILE="./test-octoserve.pid"

# Test configuration - can be overridden by environment
OCI_TEST_PULL=${OCI_TEST_PULL:-1}
OCI_TEST_PUSH=${OCI_TEST_PUSH:-1}
OCI_TEST_CONTENT_DISCOVERY=${OCI_TEST_CONTENT_DISCOVERY:-1}
OCI_TEST_CONTENT_MANAGEMENT=${OCI_TEST_CONTENT_MANAGEMENT:-1}
OCI_HIDE_SKIPPED_WORKFLOWS=${OCI_HIDE_SKIPPED_WORKFLOWS:-1}
OCI_DEBUG=${OCI_DEBUG:-0}
OCI_DELETE_MANIFEST_BEFORE_BLOBS=${OCI_DELETE_MANIFEST_BEFORE_BLOBS:-1}

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    
    # Stop octoserve if it's running
    if [[ -f "$OCTOSERVE_PID_FILE" ]]; then
        local pid=$(cat "$OCTOSERVE_PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            echo "Stopping octoserve (PID: $pid)"
            kill "$pid"
            # Wait up to 10 seconds for graceful shutdown
            for i in {1..10}; do
                if ! kill -0 "$pid" 2>/dev/null; then
                    break
                fi
                sleep 1
            done
            # Force kill if still running
            if kill -0 "$pid" 2>/dev/null; then
                kill -9 "$pid"
                echo "Force killed octoserve"
            fi
        fi
        rm -f "$OCTOSERVE_PID_FILE"
    fi
    
    # Clean up test data
    if [[ -d "$OCTOSERVE_DATA_DIR" ]]; then
        rm -rf "$OCTOSERVE_DATA_DIR"
    fi
}

# Set up cleanup trap
trap cleanup EXIT INT TERM

# Function to check if octoserve is ready
wait_for_server() {
    echo "Waiting for octoserve to be ready..."
    local max_attempts=30
    local attempt=1
    
    while [[ $attempt -le $max_attempts ]]; do
        if curl -f -s "$OCTOSERVE_URL/v2/" > /dev/null 2>&1; then
            echo -e "${GREEN}octoserve is ready!${NC}"
            return 0
        fi
        echo "Attempt $attempt/$max_attempts: Server not ready yet..."
        sleep 2
        ((attempt++))
    done
    
    echo -e "${RED}Failed to connect to octoserve after $max_attempts attempts${NC}"
    return 1
}

# Function to start octoserve
start_octoserve() {
    echo -e "${YELLOW}Starting octoserve...${NC}"
    
    # Create test data directory
    mkdir -p "$OCTOSERVE_DATA_DIR"
    
    # Build octoserve if needed
    if [[ ! -f "./bin/octoserve" ]]; then
        echo "Building octoserve..."
        go build -o bin/octoserve ./cmd/octoserve
    fi
    
    # Start octoserve in background
    OCTOSERVE_SERVER_PORT=$OCTOSERVE_PORT \
    OCTOSERVE_STORAGE_PATH="$OCTOSERVE_DATA_DIR" \
    OCTOSERVE_LOG_LEVEL=info \
    ./bin/octoserve &
    
    # Save PID
    echo $! > "$OCTOSERVE_PID_FILE"
    
    # Wait for server to be ready
    wait_for_server
}

# Function to build conformance tests
build_conformance_tests() {
    echo -e "${YELLOW}Building conformance tests...${NC}"
    
    if [[ ! -f "./distribution-spec/conformance/conformance.test" ]]; then
        cd distribution-spec/conformance
        go test -c
        cd ../..
    else
        echo "Conformance test binary already exists"
    fi
}

# Function to run conformance tests
run_conformance_tests() {
    echo -e "${YELLOW}Running OCI Distribution Spec conformance tests...${NC}"
    
    # Create results directory
    mkdir -p "$TEST_RESULTS_DIR"
    
    # Set environment variables for conformance tests
    export OCI_ROOT_URL="$OCTOSERVE_URL"
    export OCI_NAMESPACE="$TEST_NAMESPACE"
    export OCI_CROSSMOUNT_NAMESPACE="$CROSSMOUNT_NAMESPACE"
    export OCI_USERNAME=""
    export OCI_PASSWORD=""
    export OCI_TEST_PULL
    export OCI_TEST_PUSH
    export OCI_TEST_CONTENT_DISCOVERY
    export OCI_TEST_CONTENT_MANAGEMENT
    export OCI_HIDE_SKIPPED_WORKFLOWS
    export OCI_DEBUG
    export OCI_DELETE_MANIFEST_BEFORE_BLOBS
    export OCI_REPORT_DIR="$TEST_RESULTS_DIR"
    
    # Change to conformance directory and run tests
    cd distribution-spec/conformance
    
    echo "Running conformance tests with configuration:"
    echo "  Registry URL: $OCI_ROOT_URL"
    echo "  Namespace: $OCI_NAMESPACE"
    echo "  Cross-mount Namespace: $OCI_CROSSMOUNT_NAMESPACE"
    echo "  Pull Tests: $OCI_TEST_PULL"
    echo "  Push Tests: $OCI_TEST_PUSH"
    echo "  Content Discovery: $OCI_TEST_CONTENT_DISCOVERY"
    echo "  Content Management: $OCI_TEST_CONTENT_MANAGEMENT"
    echo "  Results Directory: $OCI_REPORT_DIR"
    echo ""
    
    # Run the tests
    if ./conformance.test; then
        cd ../..
        echo -e "${GREEN}Conformance tests passed!${NC}"
        
        # Show results location
        if [[ -f "$TEST_RESULTS_DIR/report.html" ]]; then
            echo -e "${GREEN}HTML report: $TEST_RESULTS_DIR/report.html${NC}"
        fi
        if [[ -f "$TEST_RESULTS_DIR/junit.xml" ]]; then
            echo -e "${GREEN}JUnit XML: $TEST_RESULTS_DIR/junit.xml${NC}"
        fi
        
        return 0
    else
        cd ../..
        echo -e "${RED}Conformance tests failed!${NC}"
        
        # Show results location even on failure
        if [[ -f "$TEST_RESULTS_DIR/report.html" ]]; then
            echo -e "${YELLOW}HTML report (with failures): $TEST_RESULTS_DIR/report.html${NC}"
        fi
        
        return 1
    fi
}

# Main execution
main() {
    echo -e "${GREEN}OCI Distribution Spec Conformance Test Runner${NC}"
    echo "=============================================="
    
    # Build conformance tests
    build_conformance_tests
    
    # Start octoserve
    start_octoserve
    
    # Run conformance tests
    if run_conformance_tests; then
        echo -e "${GREEN}All conformance tests passed! 🎉${NC}"
        exit 0
    else
        echo -e "${RED}Some conformance tests failed. Check the report for details.${NC}"
        exit 1
    fi
}

# Show usage if requested
if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    cat << EOF
OCI Distribution Spec Conformance Test Runner

Usage: $0 [options]

Environment Variables:
  OCTOSERVE_PORT                    Port for octoserve (default: 5000)
  OCTOSERVE_HOST                    Host for octoserve (default: localhost)
  OCI_NAMESPACE                     Test namespace (default: test/repo)
  OCI_CROSSMOUNT_NAMESPACE          Cross-mount namespace (default: test/other)
  OCI_REPORT_DIR                    Results directory (default: test-results)
  
  Test Configuration:
  OCI_TEST_PULL                     Enable pull tests (default: 1)
  OCI_TEST_PUSH                     Enable push tests (default: 1)
  OCI_TEST_CONTENT_DISCOVERY        Enable content discovery tests (default: 1)
  OCI_TEST_CONTENT_MANAGEMENT       Enable content management tests (default: 1)
  OCI_HIDE_SKIPPED_WORKFLOWS        Hide skipped workflows in report (default: 1)
  OCI_DEBUG                         Enable debug output (default: 0)
  OCI_DELETE_MANIFEST_BEFORE_BLOBS  Delete order (default: 1)

Examples:
  # Run all tests
  $0
  
  # Run only pull and push tests
  OCI_TEST_CONTENT_DISCOVERY=0 OCI_TEST_CONTENT_MANAGEMENT=0 $0
  
  # Run with debug output
  OCI_DEBUG=1 $0
  
  # Use custom port
  OCTOSERVE_PORT=8080 $0

EOF
    exit 0
fi

# Run main function
main "$@"