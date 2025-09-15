#!/bin/bash

# Build script for OCI Distribution Spec conformance tests

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Building OCI Distribution Spec conformance tests...${NC}"

# Check if we're in the right directory
if [[ ! -d "distribution-spec/conformance" ]]; then
    echo "Error: distribution-spec/conformance directory not found"
    echo "Make sure you're running this from the octoserve root directory"
    exit 1
fi

# Change to conformance directory
cd distribution-spec/conformance

# Check Go version
go_version=$(go version | awk '{print $3}' | sed 's/go//')
required_version="1.17"

if ! printf '%s\n%s\n' "$required_version" "$go_version" | sort -V -C; then
    echo "Error: Go $required_version or higher is required"
    echo "Current version: $go_version"
    exit 1
fi

# Build the test binary
echo "Building conformance test binary..."
go test -c

if [[ -f "conformance.test" ]]; then
    echo -e "${GREEN}Conformance test binary built successfully!${NC}"
    echo "Binary location: distribution-spec/conformance/conformance.test"
else
    echo "Error: Failed to build conformance test binary"
    exit 1
fi

cd ../..
echo -e "${GREEN}Build complete!${NC}"