# claude.md — S3 Access Control Adapter (Gateway Proxy with IAM-like Policy, Secret Key + Password)

## Purpose
We are migrating from MinIO (with our own IAM-like access control overlay) to AWS S3. 
The goal is for the client to continue using the same S3-compatible SDK without needing to change the endpoint configuration.

We will implement a **gateway proxy** where:
- The **gateway** is responsible for authenticating the client, verifying the client’s identity, and enforcing **IAM-like policies** for access control.
- The **client** will authenticate using **the same S3 IAM policies** (via secret key and password) to access the gateway.
- The **gateway** will proxy all S3 requests to AWS S3 using AWS credentials that the gateway possesses.
- **Data** will flow through the gateway, ensuring access control policies are applied and access is logged for auditing purposes.

The client will only need to change the endpoint to the gateway URL, without any changes to their existing S3 access code or authentication process.

---

## Non-goals
- We will **not** use STS or session-based credentials.
- The client will use the **same IAM-like policy** for authentication that they already use with AWS S3.
- The goal is for the client to be unaware of the underlying changes, with only the endpoint needing to be updated.

---

## High-level Architecture
### Control plane flow
1. **Client** uses their existing **secret key and password** for authentication, just as they do with AWS S3.
2. The **gateway** verifies the client identity using **IAM-like policies** (like AWS IAM roles and policies).
3. If authorized, the **Gateway** proxies the request to AWS S3 using the gateway’s AWS credentials.
4. The **Gateway** logs the access, including all policy decisions, for auditing purposes.

### Data plane flow
- **Data** flows directly through the **Gateway** to AWS S3 (client continues to use the SDK or tools without endpoint changes in their code).

---

## Key Concepts & Mapping
### IAM-like access control policies
We define access control using **IAM-like policies**:
- **Actions**: Permissions to perform operations on S3 (e.g., `s3:GetObject`, `s3:PutObject`, `s3:ListBucket`, `s3:DeleteObject`).
- **Resources**: The specific S3 buckets and object keys.
- **Conditions**: Optional conditions like IP range, time-based access, or specific metadata.
- **Principals**: The client or service identity, which is authenticated via secret key + password (same as AWS S3).

### Mapping to AWS
We enforce using the following:
1. **IAM-like policies** that map directly to AWS S3 permissions and conditions.
2. **Gateway** authenticates the client, checks the IAM-like policy, and then proxies the request to AWS S3.
3. **Base guardrails**: Use **Bucket Policies** and **IAM roles** on AWS to enforce basic access control rules.

---

## IAM-like Policy Enforcement
### Policy Components
1. **Actions**: Define the operations allowed on S3 resources.
2. **Resources**: Define which S3 resources (buckets, object keys) the client can interact with.
3. **Conditions**: (Optional) Limit the action to specific conditions like IP range or time window.
4. **Principals**: Clients are identified by their secret key and password, and are associated with IAM-like roles that grant them permissions.

---

## Security Requirements (Must)
1. **Seamless Experience**: The client will continue to authenticate using their **existing AWS IAM-like policy** (via secret key and password), ensuring **no change in behavior** for the client. The only change will be the endpoint to the gateway.
2. **Default Deny**: The gateway must ensure that no request is allowed unless explicitly granted by IAM-like policy.
3. **Minimal Access**: Access to S3 resources must be limited strictly to the client's designated scope (e.g., tenant, project).
4. **End-to-End Encryption**: All data in transit will be encrypted using TLS. Data stored on S3 will be encrypted (either SSE-S3 or SSE-KMS).
5. **Strong Authentication**: Clients authenticate using **the same IAM-like credentials** they use with AWS S3 (via secret key and password). This authentication method is seamless and does not require any changes to the client’s code.
6. **Audit and Monitoring**: The gateway will log every request, including the client identity, resource accessed, action taken, and the decision (allow/deny). Logs will be stored securely for auditing purposes.
7. **Replay & Leakage Prevention**: Enforce short TTLs for presigned URLs if used. The gateway will validate every request to ensure that the client is authorized to access the requested resource, based on the IAM-like policies.

---

## Key API Design (Gateway)
Since the client is using **the same IAM-like policies** for authentication (just as they do with AWS S3), there are no new endpoints for authentication. The gateway will simply proxy requests and enforce the same access policies that are already in place with AWS S3.

### Authentication
- The **client** authenticates using **secret key + password**, just as they would with AWS S3.
- The **gateway** verifies the client’s identity by matching the IAM policies associated with the secret key.
- **IAM policies** are enforced by the gateway, ensuring that only authorized requests are forwarded to AWS S3.

### Data Proxying
- The **gateway** handles all **S3 requests** on behalf of the client.
- For `PUT` or `GET` actions, the gateway:
  - Verifies the client’s IAM-like policy.
  - Forwards the request to AWS S3 (using the gateway’s AWS credentials).
  - Returns the response back to the client.

### Bucket Policy Guardrails
- Use **AWS Bucket Policies** for baseline access control (e.g., block public access, enforce encryption).
- The gateway can enforce additional IAM-like policies beyond what AWS’s native IAM policies provide.

---

## Security & Privacy
1. **Data Encryption**: All data in transit will be encrypted using TLS, and data stored on S3 will be encrypted (SSE-S3 or SSE-KMS).
2. **Access Control**:
   - Ensure clients can only access resources within their tenant/project scope.
   - Deny access by default unless explicitly granted by IAM-like policy.
3. **Logging & Auditing**: Log all requests for full visibility of who accessed what data and when.

### Boundary & Permissions
- For every request, the gateway will verify that the resource is within the client’s allowed tenant/project prefix.
- The client will never have the ability to access resources outside their assigned boundary.

---

## Auditing & Observability
- **Access Logs**: Log all access requests (successful and failed) with:
  - **clientId** (user identity or service)
  - **resource** (bucket + object)
  - **action** (list, get, put, etc.)
  - **decision** (allow/deny)
  - **timestamp** and **requestId**

- **Logging**: Ensure access to sensitive resources is logged.
  - Logs should be stored securely and centrally for audit and forensic analysis.

---

## Error Handling
- Deny by default with clear reason codes:
  - `DENY_TENANT_BOUNDARY`: The requested resource is outside the client’s assigned tenant/project.
  - `DENY_POLICY`: The requested action is not permitted based on IAM-like policy.
  - `DENY_INVALID_RESOURCE`: The requested object or key does not exist or is invalid.

- Avoid revealing sensitive information in error messages (e.g., not exposing whether a key exists or not).

---

## Testing Requirements
### Unit Tests
- Ensure the gateway properly verifies the IAM-like policy and forwards requests to S3 only if allowed.
- Test for proper logging of access decisions (success/failure).
- Verify that the gateway correctly handles the secret key and password authentication.
- Mock AWS S3 for integration tests to verify requests are being forwarded properly.

### Integration Tests
- Test the full request flow: client -> gateway -> S3, ensuring that only authorized clients can access their resources.
- Verify that the client can change the endpoint without needing to modify any code.

### Security Tests
- Test unauthorized access attempts to ensure the gateway denies requests outside the client’s boundaries.
- Test encryption and transport security to ensure data is not leaked in transit.

---

## Local Development Setup
- **Set up a local environment** where the **gateway** can be tested with AWS S3 emulation (e.g., using a localstack or similar tool).
- Ensure the **gateway** is capable of handling both the authentication and proxying of requests to S3 with IAM-like policies enforced.

---

## Deliverables Expected From Claude
- Gateway API handlers for authentication, presign, and authorization
- IAM-like policy enforcement logic in the gateway
- AWS S3 proxy integration (data flows through the gateway to S3)
- Full audit logging system for access control decisions
- Unit tests and boundary/security tests to validate the entire flow
- Local development environment setup for testing

