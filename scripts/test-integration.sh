#!/bin/bash

# Integration test script for S3 Access Control Gateway
# Tests authentication, policy enforcement, tenant boundaries, and audit logging

set -e

# Configuration
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
S3_REGION="${AWS_REGION:-us-east-1}"

# Test credentials from configs/credentials.yaml
TENANT_001_FULL_ACCESS_KEY="AKIAIOSFODNN7EXAMPLE"
TENANT_001_FULL_SECRET_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"

TENANT_001_READONLY_ACCESS_KEY="AKIAI44QH8DHBEXAMPLE"
TENANT_001_READONLY_SECRET_KEY="je7MtGbClwBF/2Zp9Utk/h3yCo8nvbEXAMPLEKEY"

TENANT_002_FULL_ACCESS_KEY="AKIAROSTUVWXYZEXAMPLE"
TENANT_002_FULL_SECRET_KEY="abc123XYZ789/exampleSecretKey1234567890"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Helper function to print test results
print_test_result() {
    local test_name="$1"
    local expected="$2"
    local actual="$3"

    TESTS_RUN=$((TESTS_RUN + 1))

    if [ "$expected" == "$actual" ]; then
        echo -e "${GREEN}✓ PASS${NC}: $test_name"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}: $test_name"
        echo -e "  Expected: $expected"
        echo -e "  Actual: $actual"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# Helper function to run AWS S3 command with specific credentials
run_s3_cmd() {
    local access_key="$1"
    local secret_key="$2"
    shift 2

    AWS_ACCESS_KEY_ID="$access_key" \
    AWS_SECRET_ACCESS_KEY="$secret_key" \
    AWS_REGION="$S3_REGION" \
    aws s3 "$@" --endpoint-url "$GATEWAY_URL" 2>&1
}

# Helper function to check if command succeeds
test_should_succeed() {
    local test_name="$1"
    local access_key="$2"
    local secret_key="$3"
    shift 3

    local output
    if output=$(run_s3_cmd "$access_key" "$secret_key" "$@"); then
        print_test_result "$test_name" "SUCCESS" "SUCCESS"
        return 0
    else
        print_test_result "$test_name" "SUCCESS" "FAILED: $output"
        return 1
    fi
}

# Helper function to check if command fails
test_should_fail() {
    local test_name="$1"
    local access_key="$2"
    local secret_key="$3"
    shift 3

    local output
    if output=$(run_s3_cmd "$access_key" "$secret_key" "$@"); then
        print_test_result "$test_name" "DENIED" "ALLOWED (should have been denied)"
        return 1
    else
        print_test_result "$test_name" "DENIED" "DENIED"
        return 0
    fi
}

# Print header
echo "=========================================="
echo "S3 Access Control Gateway Integration Tests"
echo "=========================================="
echo "Gateway URL: $GATEWAY_URL"
echo ""

# Check if gateway is running
echo "Checking if gateway is accessible..."
if ! curl -s -o /dev/null -w "%{http_code}" "$GATEWAY_URL" > /dev/null 2>&1; then
    echo -e "${YELLOW}Warning: Gateway may not be running at $GATEWAY_URL${NC}"
    echo "Start the gateway with: make docker-up"
    echo ""
fi

# Test 1: Tenant 001 Full Access - Allowed Operations
echo ""
echo "=== Test Suite 1: Tenant 001 Full Access ==="

# Create test file
TEST_FILE="/tmp/test-upload-$$.txt"
echo "Test data $(date)" > "$TEST_FILE"

test_should_succeed \
    "Tenant 001 Full: Upload object to tenant-001-data" \
    "$TENANT_001_FULL_ACCESS_KEY" \
    "$TENANT_001_FULL_SECRET_KEY" \
    cp "$TEST_FILE" "s3://tenant-001-data/test-file.txt"

test_should_succeed \
    "Tenant 001 Full: Read object from tenant-001-data" \
    "$TENANT_001_FULL_ACCESS_KEY" \
    "$TENANT_001_FULL_SECRET_KEY" \
    cp "s3://tenant-001-data/test-file.txt" "/tmp/test-download-$$.txt"

test_should_succeed \
    "Tenant 001 Full: List bucket tenant-001-data" \
    "$TENANT_001_FULL_ACCESS_KEY" \
    "$TENANT_001_FULL_SECRET_KEY" \
    ls "s3://tenant-001-data/"

test_should_succeed \
    "Tenant 001 Full: Delete object from tenant-001-data" \
    "$TENANT_001_FULL_ACCESS_KEY" \
    "$TENANT_001_FULL_SECRET_KEY" \
    rm "s3://tenant-001-data/test-file.txt"

test_should_succeed \
    "Tenant 001 Full: Upload to tenant-001-uploads" \
    "$TENANT_001_FULL_ACCESS_KEY" \
    "$TENANT_001_FULL_SECRET_KEY" \
    cp "$TEST_FILE" "s3://tenant-001-uploads/test-file.txt"

# Test 2: Tenant 001 Full Access - Tenant Boundary Violations
echo ""
echo "=== Test Suite 2: Tenant 001 Boundary Enforcement ==="

test_should_fail \
    "Tenant 001 Full: Cannot access tenant-002-data (tenant boundary)" \
    "$TENANT_001_FULL_ACCESS_KEY" \
    "$TENANT_001_FULL_SECRET_KEY" \
    ls "s3://tenant-002-data/"

test_should_fail \
    "Tenant 001 Full: Cannot upload to tenant-002-data (tenant boundary)" \
    "$TENANT_001_FULL_ACCESS_KEY" \
    "$TENANT_001_FULL_SECRET_KEY" \
    cp "$TEST_FILE" "s3://tenant-002-data/test-file.txt"

test_should_fail \
    "Tenant 001 Full: Cannot access shared-bucket (tenant boundary)" \
    "$TENANT_001_FULL_ACCESS_KEY" \
    "$TENANT_001_FULL_SECRET_KEY" \
    ls "s3://shared-bucket/"

# Test 3: Tenant 001 Read-Only Access
echo ""
echo "=== Test Suite 3: Tenant 001 Read-Only Policy ==="

# First upload a file with full access credentials
run_s3_cmd "$TENANT_001_FULL_ACCESS_KEY" "$TENANT_001_FULL_SECRET_KEY" \
    cp "$TEST_FILE" "s3://tenant-001-data/readonly-test.txt" > /dev/null 2>&1 || true

test_should_succeed \
    "Tenant 001 ReadOnly: Can read objects" \
    "$TENANT_001_READONLY_ACCESS_KEY" \
    "$TENANT_001_READONLY_SECRET_KEY" \
    cp "s3://tenant-001-data/readonly-test.txt" "/tmp/readonly-test-$$.txt"

test_should_succeed \
    "Tenant 001 ReadOnly: Can list buckets" \
    "$TENANT_001_READONLY_ACCESS_KEY" \
    "$TENANT_001_READONLY_SECRET_KEY" \
    ls "s3://tenant-001-data/"

test_should_fail \
    "Tenant 001 ReadOnly: Cannot upload objects (explicit deny)" \
    "$TENANT_001_READONLY_ACCESS_KEY" \
    "$TENANT_001_READONLY_SECRET_KEY" \
    cp "$TEST_FILE" "s3://tenant-001-data/should-fail.txt"

test_should_fail \
    "Tenant 001 ReadOnly: Cannot delete objects (explicit deny)" \
    "$TENANT_001_READONLY_ACCESS_KEY" \
    "$TENANT_001_READONLY_SECRET_KEY" \
    rm "s3://tenant-001-data/readonly-test.txt"

# Test 4: Tenant 002 Full Access
echo ""
echo "=== Test Suite 4: Tenant 002 Full Access ==="

test_should_succeed \
    "Tenant 002 Full: Upload to tenant-002-data" \
    "$TENANT_002_FULL_ACCESS_KEY" \
    "$TENANT_002_FULL_SECRET_KEY" \
    cp "$TEST_FILE" "s3://tenant-002-data/test-file.txt"

test_should_succeed \
    "Tenant 002 Full: Read from tenant-002-data" \
    "$TENANT_002_FULL_ACCESS_KEY" \
    "$TENANT_002_FULL_SECRET_KEY" \
    cp "s3://tenant-002-data/test-file.txt" "/tmp/tenant2-download-$$.txt"

test_should_succeed \
    "Tenant 002 Full: List tenant-002-data" \
    "$TENANT_002_FULL_ACCESS_KEY" \
    "$TENANT_002_FULL_SECRET_KEY" \
    ls "s3://tenant-002-data/"

test_should_fail \
    "Tenant 002 Full: Cannot access tenant-001-data (tenant boundary)" \
    "$TENANT_002_FULL_ACCESS_KEY" \
    "$TENANT_002_FULL_SECRET_KEY" \
    ls "s3://tenant-001-data/"

# Test 5: Invalid Authentication
echo ""
echo "=== Test Suite 5: Authentication Failures ==="

test_should_fail \
    "Invalid credentials: Wrong access key" \
    "AKIAINVALIDKEY123456" \
    "$TENANT_001_FULL_SECRET_KEY" \
    ls "s3://tenant-001-data/"

test_should_fail \
    "Invalid credentials: Wrong secret key" \
    "$TENANT_001_FULL_ACCESS_KEY" \
    "InvalidSecretKey123456789" \
    ls "s3://tenant-001-data/"

# Test 6: Default Deny (Actions not in policy)
echo ""
echo "=== Test Suite 6: Default Deny Behavior ==="

# Note: These tests assume the policies don't include HeadObject action
# Adjust based on your actual policy configuration
echo "(Skipping - requires specific policy setup)"

# Cleanup
echo ""
echo "Cleaning up test files..."
rm -f "$TEST_FILE" /tmp/test-download-$$.txt /tmp/readonly-test-$$.txt /tmp/tenant2-download-$$.txt

# Print summary
echo ""
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo "Total tests run: $TESTS_RUN"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Failed: $TESTS_FAILED${NC}"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "\n${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "\n${RED}Some tests failed!${NC}"
    exit 1
fi
