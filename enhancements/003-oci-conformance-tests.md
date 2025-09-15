# Enhancement 003: OCI Conformance Test Integration

## Enhancement Information

- **Enhancement Number**: 003
- **Title**: OCI Conformance Test Integration
- **Status**: Draft
- **Authors**: Development Team
- **Creation Date**: 2025-09-15
- **Last Updated**: 2025-09-15

## Summary

Integrate the official OCI Distribution Specification conformance test suite to provide automated validation that octoserve correctly implements the OCI Distribution Specification. This enhancement will create simple tooling to run the existing conformance tests against octoserve, ensuring compliance with the specification and providing confidence that the registry works correctly with real OCI clients.

The implementation will leverage the conformance test suite already present in the `distribution-spec/` directory and create simple scripts and build targets to make running these tests straightforward for developers and CI/CD pipelines.

## Problem Statement

Currently, octoserve lacks automated end-to-end testing that validates compliance with the OCI Distribution Specification. This creates several issues:

- **Compliance Uncertainty**: No automated way to verify that octoserve correctly implements the OCI spec
- **Regression Risk**: Changes could break OCI compatibility without being detected
- **Integration Confidence**: Unclear if the registry works correctly with real container tools
- **Manual Testing Burden**: Developers must manually test with tools like Podman/Docker
- **CI/CD Gaps**: No automated conformance validation in continuous integration

Without official conformance testing, there's risk that octoserve may have subtle compliance issues that only surface when used with specific OCI clients or workflows.

## Goals

- **Automated Conformance Validation**: Run official OCI conformance tests against octoserve
- **Simple Developer Experience**: Single command to run full conformance test suite
- **CI/CD Integration**: Enable conformance testing in continuous integration pipelines
- **Comprehensive Coverage**: Test all major OCI workflows (pull, push, discovery, management)
- **Zero External Dependencies**: Use existing conformance tests in the repository
- **Test Lifecycle Management**: Automatically start/stop octoserve for testing
- **Clear Reporting**: Generate readable test reports and failure diagnostics
- **Fast Feedback**: Provide quick subset tests for rapid development cycles

## Non-Goals

- **Custom Test Development**: Not creating new tests, only integrating existing ones
- **Test Framework Changes**: Not modifying the conformance test suite itself
- **Performance Testing**: Conformance tests focus on correctness, not performance
- **Multi-Registry Testing**: Only testing single octoserve instance
- **Authentication Testing**: Basic auth only, not complex auth scenarios
- **Storage Backend Testing**: Focus on specification compliance, not storage variants

## Proposal

### Overview

Create a simple integration layer that builds and runs the official OCI Distribution Specification conformance tests against a locally running octoserve instance. The solution will include build targets, shell scripts for lifecycle management, and proper configuration for local testing environments.

### Design Details

#### Repository Structure

```
.
├── scripts/
│   ├── test-conformance.sh          # Main test runner script
│   └── build-conformance.sh         # Build conformance test binary
├── Makefile                         # Build targets for conformance testing
└── test-results/                    # Generated test reports (gitignored)
    ├── junit.xml
    └── report.html
```

#### Test Runner Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    test-conformance.sh                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   Setup     │  │    Run      │  │      Cleanup        │  │
│  │   Environment│  │ Conformance │  │                     │  │
│  │             │  │    Tests    │  │                     │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
       │                    │                    │
       ▼                    ▼                    ▼
┌─────────────┐  ┌─────────────────────┐  ┌─────────────┐
│   Start     │  │    Execute          │  │   Stop      │
│ octoserve   │  │ ./conformance.test  │  │ octoserve   │
│ Background  │  │ (4 test workflows)  │  │ Cleanup     │
└─────────────┘  └─────────────────────┘  └─────────────┘
```

#### Test Configuration

The conformance tests will be configured for local testing:

```bash
# Server Configuration
export OCI_ROOT_URL="http://localhost:5000"
export OCI_NAMESPACE="test/repo"
export OCI_CROSSMOUNT_NAMESPACE="test/other"

# Test Workflows
export OCI_TEST_PULL=1
export OCI_TEST_PUSH=1
export OCI_TEST_CONTENT_DISCOVERY=1
export OCI_TEST_CONTENT_MANAGEMENT=1

# Test Settings
export OCI_HIDE_SKIPPED_WORKFLOWS=1
export OCI_DEBUG=0
export OCI_DELETE_MANIFEST_BEFORE_BLOBS=1
```

#### Build Integration

Makefile targets for easy testing:

```makefile
# Build conformance test binary
build-conformance:
	cd distribution-spec/conformance && go test -c

# Run full conformance test suite
test-conformance: build-conformance
	./scripts/test-conformance.sh

# Run quick subset (pull/push only)
test-conformance-quick: build-conformance
	OCI_TEST_CONTENT_DISCOVERY=0 OCI_TEST_CONTENT_MANAGEMENT=0 \
	./scripts/test-conformance.sh

# Clean up test artifacts
clean-conformance:
	rm -rf test-results/
	rm -f distribution-spec/conformance/conformance.test
```

### Implementation Phases

1. **Phase 1: Basic Infrastructure (Week 1)**
   - Create enhancement document
   - Create scripts directory and basic test runner
   - Add Makefile targets
   - Build conformance test binary

2. **Phase 2: Test Lifecycle Management (Week 1)**
   - Implement server start/stop logic
   - Add proper cleanup and error handling
   - Configure environment variables
   - Test basic workflow execution

3. **Phase 3: Integration and Refinement (Week 2)**
   - Add test result reporting
   - Create quick test variants
   - Add documentation
   - Test CI/CD integration scenarios

## Alternative Approaches

### Custom Test Suite
**Considered**: Writing custom end-to-end tests
**Rejected**: The official conformance tests already provide comprehensive coverage and ensure real OCI compliance

### Docker Compose Setup
**Considered**: Using docker-compose for test environment
**Rejected**: Adds complexity and external dependencies; simple script approach is more maintainable

### GitHub Actions Only
**Considered**: Only supporting GitHub Actions for testing
**Rejected**: Local testing capability is essential for development workflow

### Test Framework Integration
**Considered**: Integrating conformance tests into Go test framework
**Rejected**: Conformance tests are designed to run as standalone binary; integration would be complex

## Testing Strategy

- **Conformance Test Validation**: The conformance tests themselves provide comprehensive testing
- **Script Testing**: Manual validation of script lifecycle management and error handling
- **CI Integration Testing**: Verify the tests can run in GitHub Actions environment
- **Multiple Scenario Testing**: Test with different configuration options and workflows
- **Failure Scenario Testing**: Ensure proper cleanup when tests fail or are interrupted

## Security Considerations

- **Local Testing Only**: Tests run against localhost, no external network access required
- **Temporary Data**: Test data is created in temporary directories and cleaned up
- **No Persistent State**: Tests don't modify permanent registry data
- **Isolated Environment**: Each test run uses fresh registry state
- **No Authentication Secrets**: Tests use basic or no authentication

## Performance Impact

- **No Runtime Impact**: Conformance tests are development/CI tools only
- **Build Time**: Additional ~30 seconds to build conformance test binary
- **Test Duration**: Full conformance suite takes ~2-5 minutes depending on hardware
- **Disk Usage**: Temporary test data ~10MB, cleaned up after tests
- **Network Impact**: Tests only use localhost networking

## Backward Compatibility

- **No Breaking Changes**: Conformance testing is purely additive
- **Optional Feature**: Tests are opt-in via make targets
- **Existing Workflows**: No impact on existing development or deployment workflows
- **Tool Compatibility**: Tests validate that octoserve works with standard OCI tools

## Migration Path

No migration required - this is a purely additive feature for testing. Developers can:

1. Start using `make test-conformance` immediately
2. Gradually integrate into CI/CD pipelines
3. Use quick tests during development for faster feedback

## Documentation Requirements

- **Developer Documentation**: How to run conformance tests locally
- **CI/CD Documentation**: Integration examples for GitHub Actions
- **Troubleshooting Guide**: Common test failures and solutions
- **Test Report Guide**: Understanding conformance test output

## Future Considerations

- **Performance Testing**: Add performance benchmarks to conformance testing
- **Multi-Architecture Testing**: Test on different platforms (ARM, x86)
- **Storage Backend Testing**: Run conformance tests with different storage backends
- **Authentication Testing**: Expand to test different authentication methods
- **Continuous Monitoring**: Automated conformance testing on every commit
- **Badge Integration**: Add conformance status badges to README

## Implementation Timeline

- **Week 1**: Basic infrastructure and test runner implementation
- **Week 2**: Integration refinement and documentation

**Total Duration**: 2 weeks
**Milestone Deliverables**: Working `make test-conformance` command with comprehensive OCI compliance validation

## References

- [OCI Distribution Specification](https://github.com/opencontainers/distribution-spec)
- [OCI Conformance Test Suite](https://github.com/opencontainers/distribution-spec/tree/main/conformance)
- [Reggie OCI Client Library](https://github.com/bloodorangeio/reggie)
- [Ginkgo Testing Framework](https://onsi.github.io/ginkgo/)
- [Docker Distribution Registry](https://github.com/distribution/distribution)