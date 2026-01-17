#!/bin/bash

# Quick smoke test for S3 Access Control Gateway
# Runs basic sanity checks to verify the gateway is working

set -e

# Configuration
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
S3_REGION="${AWS_REGION:-us-east-1}"

# Test credentials (Tenant 001 Full Access)
ACCESS_KEY="AKIAIOSFODNN7EXAMPLE"
SECRET_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "S3 Gateway Smoke Test"
echo "=========================================="
echo "Gateway: $GATEWAY_URL"
echo ""

# Check gateway health
echo -n "1. Checking gateway accessibility... "
if curl -s -f "$GATEWAY_URL" > /dev/null 2>&1 || curl -s "$GATEWAY_URL" 2>&1 | grep -q "SignatureDoesNotMatch\|AccessDenied\|InvalidRequest" ; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗ Gateway not accessible${NC}"
    exit 1
fi

# Test file
TEST_FILE="/tmp/smoke-test-$$.txt"
echo "Smoke test $(date)" > "$TEST_FILE"

# Test upload
echo -n "2. Testing object upload... "
if AWS_ACCESS_KEY_ID="$ACCESS_KEY" \
   AWS_SECRET_ACCESS_KEY="$SECRET_KEY" \
   AWS_REGION="$S3_REGION" \
   aws s3 cp "$TEST_FILE" "s3://tenant-001-data/smoke-test.txt" \
   --endpoint-url "$GATEWAY_URL" > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC}"
    rm -f "$TEST_FILE"
    exit 1
fi

# Test download
echo -n "3. Testing object download... "
DOWNLOAD_FILE="/tmp/smoke-download-$$.txt"
if AWS_ACCESS_KEY_ID="$ACCESS_KEY" \
   AWS_SECRET_ACCESS_KEY="$SECRET_KEY" \
   AWS_REGION="$S3_REGION" \
   aws s3 cp "s3://tenant-001-data/smoke-test.txt" "$DOWNLOAD_FILE" \
   --endpoint-url "$GATEWAY_URL" > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC}"
    rm -f "$TEST_FILE" "$DOWNLOAD_FILE"
    exit 1
fi

# Test list
echo -n "4. Testing bucket list... "
if AWS_ACCESS_KEY_ID="$ACCESS_KEY" \
   AWS_SECRET_ACCESS_KEY="$SECRET_KEY" \
   AWS_REGION="$S3_REGION" \
   aws s3 ls "s3://tenant-001-data/" \
   --endpoint-url "$GATEWAY_URL" > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC}"
    rm -f "$TEST_FILE" "$DOWNLOAD_FILE"
    exit 1
fi

# Test delete
echo -n "5. Testing object delete... "
if AWS_ACCESS_KEY_ID="$ACCESS_KEY" \
   AWS_SECRET_ACCESS_KEY="$SECRET_KEY" \
   AWS_REGION="$S3_REGION" \
   aws s3 rm "s3://tenant-001-data/smoke-test.txt" \
   --endpoint-url "$GATEWAY_URL" > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC}"
    rm -f "$TEST_FILE" "$DOWNLOAD_FILE"
    exit 1
fi

# Test tenant boundary (should fail)
echo -n "6. Testing tenant boundary enforcement... "
if AWS_ACCESS_KEY_ID="$ACCESS_KEY" \
   AWS_SECRET_ACCESS_KEY="$SECRET_KEY" \
   AWS_REGION="$S3_REGION" \
   aws s3 ls "s3://tenant-002-data/" \
   --endpoint-url "$GATEWAY_URL" > /dev/null 2>&1; then
    echo -e "${RED}✗ (Should have been denied)${NC}"
    rm -f "$TEST_FILE" "$DOWNLOAD_FILE"
    exit 1
else
    echo -e "${GREEN}✓${NC}"
fi

# Cleanup
rm -f "$TEST_FILE" "$DOWNLOAD_FILE"

echo ""
echo -e "${GREEN}=========================================="
echo "All smoke tests passed!"
echo "==========================================${NC}"
exit 0
