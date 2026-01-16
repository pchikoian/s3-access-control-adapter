# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Build the gateway
make build

# Run tests
make test

# Run tests with coverage
make test-coverage

# Start local development environment (LocalStack + Gateway)
make docker-up

# Stop local development environment
make docker-down

# Run the gateway locally (requires configs/gateway.yaml)
make run

# Format code
make fmt

# Download dependencies
make deps
```

## Project Structure

```
├── cmd/gateway/main.go           # Application entry point
├── internal/
│   ├── auth/                     # Authentication (SigV4 validation, credential store)
│   ├── policy/                   # IAM-like policy engine (default deny)
│   ├── proxy/                    # HTTP handler, S3 client, request parsing
│   ├── audit/                    # JSON audit logging
│   ├── config/                   # YAML configuration loading
│   └── errors/                   # Error types and S3 XML error responses
├── configs/                      # Sample configuration files
│   ├── gateway.yaml              # Server and AWS settings
│   ├── credentials.yaml          # Client credentials
│   └── policies.yaml             # IAM-like policies
└── docker-compose.yaml           # LocalStack + Gateway
```

## Core Architecture

```
Client (S3 SDK) --> Gateway Proxy --> AWS S3
                        |
                   IAM-like Policy
                   Enforcement +
                   Audit Logging
```

### Request Flow

1. **Parse Request**: Extract bucket, key, action from HTTP request
2. **Authenticate**: Validate AWS SigV4 signature against stored credentials
3. **Check Tenant Boundary**: Verify bucket matches client's allowed scopes
4. **Evaluate Policy**: Check IAM-like policies (default deny)
5. **Proxy to S3**: Forward request using gateway's AWS credentials
6. **Audit Log**: Record decision with all required fields

## Key Design Principles

1. **Default Deny**: No request allowed unless explicitly granted by policy
2. **Explicit Deny Precedence**: Deny statements override any Allow
3. **Tenant Boundaries**: Clients can only access buckets matching their scopes
4. **Seamless Client Experience**: Clients only change endpoint URL

## Configuration

### credentials.yaml
```yaml
credentials:
  - accessKey: "AKIAIOSFODNN7EXAMPLE"
    secretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    clientId: "service-a"
    tenantId: "tenant-001"
    policies: ["tenant-001-full-access"]
    scopes: ["tenant-001-*"]  # Bucket patterns for tenant boundary
```

### policies.yaml
```yaml
policies:
  - name: "tenant-001-full-access"
    statements:
      - effect: Allow
        actions: ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"]
        resources: ["arn:aws:s3:::tenant-001-*", "arn:aws:s3:::tenant-001-*/*"]
```

## Error Codes

- `DENY_TENANT_BOUNDARY`: Resource outside client's assigned scope
- `DENY_POLICY`: Action not permitted by policy
- `DENY_AUTH_FAILED`: Signature validation failed
- `DENY_INVALID_RESOURCE`: Invalid bucket or key

## Testing

Run a single test:
```bash
go test -v ./internal/policy -run TestPolicyEngine_DefaultDeny
```

Run tests for a package:
```bash
go test -v ./internal/policy/...
```
