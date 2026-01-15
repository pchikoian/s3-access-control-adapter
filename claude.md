# claude.md — S3 Access Control Adapter (Option 2: Capability-Issuing Gateway)

## Purpose
We are migrating from MinIO (with our own IAM-like access control overlay) to AWS S3.
We will NOT proxy data through our gateway. Instead, the gateway acts as:
- **PDP (Policy Decision Point)**: evaluates our custom access-control rules
- **Capability issuer**: mints short-lived, tightly-scoped access for S3
Clients then access S3 directly using:
- **Presigned URLs** (preferred for simple GET/PUT), and/or
- **STS AssumeRole credentials with session policy** (preferred for multi-op sessions)

The adapter must preserve our existing semantics as much as possible while leveraging AWS primitives.

---

## Non-goals
- Do not implement custom authorization inside S3 (not possible).
- Do not build a full reverse proxy for S3 data plane.
- Do not rely on long-lived credentials on clients.

---

## High-level Architecture
### Control plane flow
1. Client calls **Access Gateway** with:
   - user identity (JWT/OIDC), tenant/project context, intended operation(s), object key(s)
2. Gateway authenticates and authorizes via our internal policy engine
3. If allowed, Gateway issues **capability**:
   - Presigned URL(s) OR STS session credentials
4. Client uses capability to access S3 directly
5. Gateway logs decision + capability issuance for auditing

### Data plane flow
- Client -> S3 directly (no gateway in the data path)

---

## Key Concepts & Mapping
### Our access-control model
We define:
- Subject: user/service identity
- Context: tenant/project/workspace
- Resource: bucket + object key (and optionally tags/metadata)
- Action: list/get/put/delete/head/multipart/restore
- Conditions: time, IP/network, attributes, quotas, workflows, etc.

### Mapping to AWS
We enforce using a combination of:
1. **STS AssumeRole + Session Policy** (stronger, multi-operation)
2. **Presigned URLs** (simple, easy, tight TTL)
3. Bucket policies are guardrails only (deny-by-default, allow via roles, enforce encryption, block public, etc.)

---

## Choosing Capability Type
### Use Presigned URL when:
- Single operation (GET or PUT) on a specific object key
- Short TTL acceptable (e.g., 1–10 minutes)
- Client is browser/mobile and we want simplest integration

### Use STS AssumeRole when:
- The client needs multiple operations (list + get, multipart upload, delete)
- SDK integration expects AWS credentials
- We need richer constraints (prefix scope, actions set) via session policy

Hybrid is allowed:
- For uploads: issue multipart-related STS creds
- For downloads: presign GET

---

## Security Requirements (Must)
1. **Default deny**: gateway must deny unless explicitly allowed by policy engine.
2. **Least privilege**: capabilities must be scoped to:
   - exact bucket(s)
   - key prefix or exact key(s)
   - minimal actions
   - short TTL
3. **Short TTL**:
   - Presigned URLs: 60s–10m (prefer <= 5m)
   - STS credentials: 15m–60m (prefer <= 30m)
4. **Guardrails** in S3:
   - Block Public Access enabled
   - Bucket policy enforces TLS (`aws:SecureTransport`)
   - Require encryption-at-rest (SSE-S3 or SSE-KMS)
   - Optional: deny non-approved principals
5. **No long-lived secrets** on clients.
6. **Audit logs**:
   - Log every decision (allow/deny) with request context
   - Log every capability issuance with a correlation id
   - Include: subject, tenant, resource, action, TTL, reason, policy version, request id
7. **Replay & leakage mitigation**:
   - Keep TTL short
   - Prefer exact-key presign (not broad prefix)
   - For sensitive ops, consider “download token” indirection (client fetches presign per download)

---

## Important Limitations (Acknowledge)
- Presigned URLs and STS creds cannot be instantly revoked; revocation is bounded by TTL.
- Fine-grained per-request checks after issuance are not possible without proxying.
- Avoid issuing overly broad list permissions; list semantics can leak object names.

Mitigation strategy:
- Keep TTL short
- Consider per-object issuance
- Use prefix partitioning (`tenantId/projectId/...`) and only ever grant within that partition

---

## API Design (Gateway)
### Endpoints (suggested)
1. `POST /v1/capabilities/presign`
   - Input: `{ action: "GET"|"PUT", bucket, key, contentType?, contentLength?, checksum?, ttlSeconds? }`
   - Output: `{ url, headers?, expiresAt, requestId }`

2. `POST /v1/capabilities/sts`
   - Input: `{ actions: ["s3:GetObject",...], bucket, prefixOrKeys, ttlSeconds?, sessionName? }`
   - Output: `{ accessKeyId, secretAccessKey, sessionToken, expiresAt, region, requestId }`

3. `POST /v1/authorize` (optional, for “check only”)
   - Input: `{ subject, action, resource, context }`
   - Output: `{ decision: "allow"|"deny", reason, policyVersion }`

### AuthN/AuthZ
- Gateway verifies JWT/OIDC and maps to internal subject.
- Authorization is done by our policy engine (PDP).
- Gateway never accepts “bucket/key” without validating tenant boundary rules.

---

## S3 Key Layout (Strong Recommendation)
Enforce deterministic partitioning:
- `s3://<bucket>/<tenantId>/<projectId>/<resourceType>/<objectId>`

Never grant access outside the caller’s tenant prefix.

---

## IAM & STS Implementation Details
### Roles
- One or more “access roles” per environment/bucket.
- Gateway assumes a role that can assume downstream roles if needed.

### Session policy (generated per request)
- Must include:
  - `Action`: minimal set
  - `Resource`: `arn:aws:s3:::bucket/prefix*` or explicit keys
- Prefer explicit keys where possible; otherwise prefix with tight partition boundary.

### Optional constraints
- If using SSE-KMS, include KMS permissions and enforce encryption headers.

---

## Presign Implementation Details
- Presign with SigV4, include:
  - Method: GET/PUT
  - Bucket + Key
  - Expiration
- For PUT, optionally require:
  - `Content-Type`
  - `x-amz-server-side-encryption`
  - `x-amz-checksum-*` / `Content-MD5`
- Reject requests that try to presign a key outside allowed prefix.

---

## Auditing & Observability
### Correlation
- Every request has `requestId` (UUID).
- Propagate into logs and returned payload.
- Include upstream trace ids if available.

### Events
Emit structured events:
- `authz_decision`
- `capability_issued`
- `capability_denied`
- `capability_error`

### Recommended fields
- subjectId, tenantId, projectId
- action(s)
- bucket, key/prefix
- ttlSeconds, expiresAt
- decision, denyReason
- policyVersion, policyHash
- clientIp, userAgent (if available)
- awsRequestId (if available)

---

## Error Handling
- Deny by default with clear reason codes:
  - `DENY_TENANT_BOUNDARY`
  - `DENY_POLICY`
  - `DENY_INVALID_RESOURCE`
  - `DENY_UNSUPPORTED_ACTION`
- Avoid leaking sensitive info in error messages to clients.
- Log full diagnostic detail server-side.

---

## Testing Requirements
### Unit tests
- Policy decision mapping -> session policy generation
- Boundary checks: cannot escape prefix via path tricks (`..`, URL encoding)
- TTL enforcement
- For presign: required headers are enforced in presigned signature

### Integration tests (can be mocked if needed)
- STS assume role + S3 access with issued creds works
- Presigned PUT then GET works within allowed scope
- Denied scope cannot access

### Security tests
- Attempt key traversal / encoding bypass
- Attempt wildcard broadening
- Attempt list leakage outside prefix

---

## Implementation Guidelines for Claude
When generating code:
1. Keep modules small and testable:
   - `authn/` (JWT validation)
   - `authz/` (policy engine adapter)
   - `capabilities/` (presign + sts)
   - `aws/` (S3/STS clients)
   - `audit/` (structured logging/events)
2. Centralize “tenant boundary validation” in one place and reuse everywhere.
3. Avoid embedding AWS credentials in config; use workload identity (IRSA) or instance profile.
4. Use typed DTOs for inputs/outputs; validate all inputs strictly.
5. Ensure no sensitive tokens are logged.

---

## Open Questions (Default Answers)
If the repo doesn’t specify:
- TTL defaults: presign 300s, sts 1800s
- Encryption: enforce SSE-S3 or SSE-KMS (prefer SSE-KMS if compliance requires)
- List: disallow by default unless explicitly needed and constrained to prefix
- Delete: disallow by default unless explicitly required

---

## Deliverables Expected From Claude
- Gateway API handlers for presign and STS
- Policy compilation logic (internal decision -> AWS constraints)
- Robust validation + audit logging
- Unit tests and boundary/security tests
- Example bucket policy guardrails (as IaC snippets) if requested

