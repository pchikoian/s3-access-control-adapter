# Test Scripts

This directory contains test scripts for the S3 Access Control Gateway.

## Available Scripts

### test-all.sh
Comprehensive test runner that runs unit tests, builds the project, and optionally runs integration tests.

```bash
# Run unit tests only
./scripts/test-all.sh

# Run unit tests with coverage report
./scripts/test-all.sh -c

# Run unit tests + integration tests
./scripts/test-all.sh -i

# Run all tests with coverage and verbose output
./scripts/test-all.sh -i -c -v

# Show help
./scripts/test-all.sh -h
```

### test-integration.sh
Full integration test suite that validates:
- Authentication with SigV4 signatures
- Policy enforcement (allow/deny scenarios)
- Tenant boundary enforcement
- Different access levels (full access vs read-only)
- Default deny behavior

**Requirements:**
- Docker and docker-compose running
- LocalStack and Gateway services started (`make docker-up`)

```bash
# Run integration tests
./scripts/test-integration.sh

# With custom gateway URL
GATEWAY_URL=http://localhost:9000 ./scripts/test-integration.sh
```

### test-smoke.sh
Quick smoke test that verifies basic gateway functionality:
- Gateway accessibility
- Object upload/download
- Bucket listing
- Object deletion
- Basic tenant boundary check

```bash
# Run smoke test
./scripts/test-smoke.sh

# With custom gateway URL
GATEWAY_URL=http://localhost:9000 ./scripts/test-smoke.sh
```

### localstack-init.sh
Initialization script for LocalStack that creates test buckets and sample data. This script is automatically executed when LocalStack starts via docker-compose.

## Quick Start

1. **Start the environment:**
   ```bash
   make docker-up
   ```

2. **Run smoke test to verify setup:**
   ```bash
   ./scripts/test-smoke.sh
   ```

3. **Run full test suite:**
   ```bash
   ./scripts/test-all.sh -i
   ```

## Test Scenarios Covered

### Authentication Tests
- Valid credentials (Tenant 001, Tenant 002)
- Invalid access keys
- Invalid secret keys
- SigV4 signature validation

### Policy Enforcement Tests
- **Tenant 001 Full Access:**
  - Can upload, download, delete, and list objects in tenant-001-* buckets
  - Cannot access tenant-002-* buckets (tenant boundary)
  - Cannot access shared buckets (tenant boundary)

- **Tenant 001 Read-Only:**
  - Can read and list objects in tenant-001-* buckets
  - Cannot upload objects (explicit deny)
  - Cannot delete objects (explicit deny)

- **Tenant 002 Full Access:**
  - Can upload, download, delete, and list objects in tenant-002-* buckets
  - Cannot access tenant-001-* buckets (tenant boundary)

### Tenant Boundary Tests
- Clients can only access buckets matching their scope patterns
- Cross-tenant access is denied
- Shared bucket access is denied unless explicitly in scope

## Test Data

The integration tests use credentials and policies from:
- `configs/credentials.yaml` - Test client credentials
- `configs/policies.yaml` - IAM-like access policies

Test buckets created by LocalStack:
- `tenant-001-data` - Primary bucket for tenant 001
- `tenant-001-uploads` - Secondary bucket for tenant 001
- `tenant-002-data` - Primary bucket for tenant 002
- `shared-bucket` - Shared bucket (access denied by default)

## Troubleshooting

### Gateway not accessible
Ensure Docker services are running:
```bash
docker-compose ps
make docker-up
```

### Tests failing
1. Check gateway logs:
   ```bash
   docker-compose logs gateway
   ```

2. Check LocalStack logs:
   ```bash
   docker-compose logs localstack
   ```

3. Verify test buckets exist:
   ```bash
   aws s3 ls --endpoint-url http://localhost:4566
   ```

### Clean environment
```bash
make docker-down
docker volume rm s3-access-control-adapter_localstack_data
make docker-up
```

## CI/CD Integration

For CI/CD pipelines, use the test-all.sh script:

```yaml
# Example GitHub Actions
- name: Run tests
  run: ./scripts/test-all.sh -i -c
```

The script exits with code 0 on success and non-zero on failure, making it suitable for automated testing.
