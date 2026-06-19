# Specification Update Proposal

## Owner

@vireshnavalli

## Summary

Add comprehensive common error responses for 4xx and 5xx HTTP status codes to all API routes
following RFC 9457 (Problem Details for HTTP APIs). This standardizes error handling across the
margo specification and provides consistent error information to API consumers, including
machine-parseable stable type values and retry semantics.

## Reason for proposal

RFC 9457 defines a standard format for problem details in HTTP responses, enabling consistent error
handling across REST APIs. Implementing this across all margo API routes provides:

1. **Standardization** - Consistent error response format across all endpoints
2. **API Consumer Experience** - Clear, predictable error information
3. **RFC Compliance** - Adherence to industry standard (RFC 9457)
4. **Better Error Handling** - Enables robust client-side error handling with standard fields
5. **Retry Semantics** - Clients can implement consistent UX and automation using `Retry-After`,
   `retryable`, and `backoffStrategy` fields
6. **Stable Type URIs** - Machine-parseable stable `type` values enable programmatic error handling
7. **Specification Clarity** - Addresses specification issue #153

## Requirements alignment acknowledgement

This SUP aligns with margo's commitment to following industry standards and best practices for REST
API design. RFC 9457 is an IETF-approved standard for problem details in HTTP APIs, ensuring our
specification remains modern and interoperable.

**Related Feature(s):** [Issue #153 - margo/specification](https://github.com/margo/specification/issues/153)

## Technical proposal

### RFC 9457 Error Response Structure

Implement the following standard error response format based on RFC 9457, including extension
members (§3.5) for retry semantics and field-level validation errors:

```yaml
ProblemDetail:
  type: object
  description: >
    RFC 9457 Problem Details for HTTP APIs.
    Returned with Content-Type: application/problem+json.
    See https://www.rfc-editor.org/rfc/rfc9457
  required:
    - type
    - title
    - status
  properties:
    type:
      type: string
      format: uri
      description: >
        URI reference that identifies the problem type.
        MUST be one of the stable registered URIs defined in the
        Stable Type URI Registry section below.
        Clients MUST use this field for programmatic error handling,
        NOT the status code or title.
      example: https://margo.org/problems/invalid-certificate
    title:
      type: string
      description: Short human-readable summary of the problem type.
      example: Invalid Certificate
    status:
      type: integer
      description: HTTP status code.
      example: 400
    detail:
      type: string
      description: Human-readable explanation specific to this occurrence of the problem.
      example: Certificate is not valid 
    instance:
      type: string
      format: uri
      description: URI reference identifying the specific occurrence of the problem.
      example: /api/v1/onboarding
    retryable:
      type: boolean
      description: >
        Whether the client MAY retry the request.
        If true, client SHOULD respect the Retry-After response header when present.
      example: false
    retryAfterSeconds:
      type: integer
      description: >
        Advisory retry delay in seconds.
        The authoritative value is the Retry-After response header when present.
        This field is advisory only.
      example: 30
    backoffStrategy:
      type: string
      enum: [none, fixed, exponential]
      description: >
        Recommended backoff strategy for retrying.
        none: do not retry.
        fixed: retry after retryAfterSeconds.
        exponential: use exponential backoff starting at retryAfterSeconds.
      example: exponential
    errors:
      type: array
      description: >
        Optional list of field-level validation errors.
        SHOULD be present on 422 Unprocessable Entity responses.
      items:
        type: object
        properties:
          field:
            type: string
            description: Field path that caused the validation error.
            example: properties.roles
          message:
            type: string
            description: Human-readable validation error message.
            example: must contain at least one valid role
  additionalProperties: true
  # additionalProperties: true allows RFC 9457 §3.5 extension members
```

---

### Stable Type URI Registry

The following `type` URI values are stable and MUST be used by all implementations.
Clients MUST use the `type` field for programmatic error handling.

> **Note on URI Dereferenceability:** Per RFC 9457 §4.2, `type` URIs SHOULD be
> dereferenceable — returning a human-readable page describing the problem type.
> The URIs listed below currently return 404 as the error catalogue page at
> `https://margo.org/problems/` has not yet been published. This is a known
> limitation and a hosted error catalogue is planned as a follow-up documentation
> task based on this SUP review (Do we really need to host URIs or not). The URIs remain valid stable identifiers regardless of dereferenceability.
> Clients MUST NOT fetch these URIs at runtime; they are identifiers only.


| Type URI | HTTP Status | Description |
|---|---|---|
| `https://margo.org/problems/invalid-certificate` | 400 | Certificate is not valid |
| `https://margo.org/problems/invalid-digest-header` | 400 | Missing or invalid content-digest header |
| `https://margo.org/problems/signature-verification-failed` | 401 | Payload signature verification failed |
| `https://margo.org/problems/certificate-not-trusted` | 403 | Client certificate not trusted or revoked |
| `https://margo.org/problems/not-found` | 404 | Requested resource does not exist |
| `https://margo.org/problems/conflict` | 409 | Request conflicts with current resource state |
| `https://margo.org/problems/not-acceptable` | 406 | Server cannot produce requested media type |
| `https://margo.org/problems/semantic-error` | 422 | Request body contains a semantic error |
| `https://margo.org/problems/rate-limit-exceeded` | 429 | Rate limit exceeded |
| `https://margo.org/problems/internal-error` | 500 | Unexpected server error |
| `https://margo.org/problems/not-implemented` | 501 | Feature not yet implemented |
| `https://margo.org/problems/service-unavailable` | 503 | Server temporarily unable to handle request |

---

### Retry Semantics

Transient failures MUST communicate retry information as follows:

| Response | `Retry-After` Header | `retryable` | `backoffStrategy` |
|---|---|---|---|
| `429 Too Many Requests` | REQUIRED | `true` | `exponential` |
| `503 Service Unavailable` | REQUIRED | `true` | `exponential` |
| `500 Internal Server Error` | RECOMMENDED | `true` | `exponential` |
| All other errors | NOT applicable | `false` | `none` |

Clients MUST:
- Respect the `Retry-After` header value and MUST NOT retry before it elapses
- Use `retryable` field to determine if retry is appropriate
- Apply `backoffStrategy` when retrying

---

### Common Error Responses

All API routes must include standardized error responses:

#### 4xx Client Errors

- **400 Bad Request**: Invalid request parameters or body
- **401 Unauthorized**: Missing or invalid authentication / signature verification failed
- **403 Forbidden**: Authenticated but certificate not trusted or client rejected
- **404 Not Found**: Requested resource does not exist
- **406 Not Acceptable**: Server cannot produce response matching Accept header
- **409 Conflict**: Request conflicts with current state
- **422 Unprocessable Entity**: Validation error with semantic meaning — MUST include `errors[]`
- **429 Too Many Requests**: Rate limit exceeded — MUST include `Retry-After` header

#### 5xx Server Errors

- **500 Internal Server Error**: Unexpected server error — SHOULD include `Retry-After` header
- **501 Not Implemented**: Feature not yet implemented
- **503 Service Unavailable**: Server temporarily unable to handle request — MUST include
  `Retry-After` header

---

### OpenAPI Specification Updates

All endpoints in the OpenAPI specification must include applicable error responses using
`$ref` to the shared `ProblemDetail` schema:

```yaml
---
openapi: 3.0.3
info:
  title: Margo Workload Management API
  version: 1.0.0
  description: 
    API for managing workloads on Margo-compliant edge devices.
    Includes the APIs for exchanging desired state and current state.
    Communication is secured using server-side TLS (TLS 1.3 preferred),
    and payloads are signed using X.509 certificates.

servers:
  - url: https://wfm.margo.org/
    description: Workload Fleet Manager API

security:
  - PayloadSignature: []

paths:
  /api/v1/onboarding/certificate:
    get:
      summary: Download Root CA certificate
      security: []
      responses:
        '200':
          description: Root CA certificate
          content:
            application/json:
              schema:
                type: object
                properties:
                  certificate:
                    type: string
                    description: Base64-encoded certificate text
  /api/v1/onboarding:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [apiVersion, kind, certificate]
              properties:
                apiVersion:
                  type: string
                  description: API version identifier
                kind:
                  type: string
                  enum: [OnboardingRequest]
                  description: Resource kind
                certificate:
                  description: Base64-encoded client certificate
                  type: string
        required: true
      responses:
        '201':
          content:
            application/json:
              schema:
                properties:
                  clientId:
                    type: string
                type: object
          description: New client onboarded successfully.
        '400':
          description: Bad Request
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '403':
          description: Forbidden — Certificate not trusted or client rejected
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
      security:
        - PayloadSignature: []
      summary: Complete onboarding with client certificate
  /api/v1/clients/{clientId}/capabilities/{deviceId}:
    post:
      summary: Report device capabilities
      security:
        - PayloadSignature: []
      parameters:
        - name: clientId
          in: path
          required: true
          schema:
            type: string
        - name: deviceId
          in: path
          required: true
          schema:
            $ref: '#/components/schemas/DeviceId'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/DeviceCapabilitiesManifest'
      responses:
        '201':
          description: Capabilities reported successfully
        '400':
          description: Bad Request
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '401':
          description: Unauthorized — Signature verification failed
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '403':
          description: Forbidden — Certificate not trusted or client rejected
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '404':
          description: Not Found
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '422':
          description: Unprocessable Entity — Semantic error in request body
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
    put:
      summary: Update device capabilities (Update)
      security:
        - PayloadSignature: []
      parameters:
        - name: clientId
          in: path
          required: true
          schema:
            type: string
        - name: deviceId
          in: path
          required: true
          schema:
            $ref: '#/components/schemas/DeviceId'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/DeviceCapabilitiesManifest'
      responses:
        '201':
          description: Capabilities reported successfully
        '400':
          description: Bad Request
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '401':
          description: Unauthorized — Signature verification failed
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '403':
          description: Forbidden — Certificate not trusted or client rejected
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '404':
          description: Not Found
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '422':
          description: Unprocessable Entity — Semantic error in request body
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
    delete:
      summary: Remove device (Unregister)
      security:
        - PayloadSignature: []
      parameters:
        - name: clientId
          in: path
          required: true
          schema:
            type: string
        - name: deviceId
          in: path
          required: true
          schema:
            $ref: '#/components/schemas/DeviceId'
      responses:
        '204':
          description: Device capabilities removed successfully
        '400':
          description: Bad Request
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '401':
          description: Unauthorized — Signature verification failed
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '403':
          description: Forbidden — Certificate not trusted or client rejected
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '404':
          description: Not Found
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
  /api/v1/clients/{clientId}/bundles/{digest}:
    get:
      summary: Retrieve bundle information for a specific device and digest
      security:
        - PayloadSignature: []
      parameters:
        - name: clientId
          in: path
          required: true
          schema:
            type: string
          description: Unique identifier of the device-client
        - name: digest
          in: path
          required: true
          schema:
            type: string
          description: Content-addressable digest of the bundle archive. MUST conform to the 'digest' attribute in the Digest Specification and MUST equal the digest computed over the exact sequence of bytes (per Exact Bytes Rule) in the HTTP 200 OK response body. If the server cannot produce content whose digest matches this value it MUST return 404 Not Found.
        - in: header
          name: If-None-Match
          required: false
          schema:
            type: string
          description: Quoted ETag (same as digest) previously returned for this bundle.
      responses:
        '200':
          description: Bundle archive (immutable)
          headers:
            ETag:
              schema:
                type: string
              description: New ETag for the returned manifest
            Cache-Control:
              schema:
                type: string
              description: public, max-age=31536000, immutable
          content:
            application/vnd.margo.bundle.v1+tar+gzip:
              schema:
                type: string
                format: binary
                description: Gzip-compressed tar containing one YAML file per deployment.
        '304':
          description: Representation not modified
        '400':
          description: Bad Request
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '404':
          description: Not Found
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '500':
          description: Internal Server Error
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
  /api/v1/clients/{clientId}/deployments:
    get:
      summary: Retrieve the complete desired state for all workloads assigned to a device
      security:
        - PayloadSignature: []
      parameters:
        - name: clientId
          in: path
          required: true
          schema:
            type: string
          description: The unique identifier of the Edge Compute Device making the request
        - name: If-None-Match
          in: header
          required: false
          schema:
            type: string
          description: >
             ETag value of the last successfully synced manifest. The ETag is returned to the client from the /deployments endpoint, it is the digest of the state manifest.
        - name: Accept
          in: header
          required: false
          schema:
            type: string
          description: >
            Indicates which manifest formats the client supports. Supported
            values: application/vnd.margo.manifest.v1+json.
      responses:
        '200':
          description: Manifest returned in the negotiated format
          headers:
            Content-Type:
              schema:
                type: string
              description: Format of the returned manifest
            ETag:
              schema:
                type: string
              description: New ETag for the returned manifest
          content:
            application/vnd.margo.manifest.v1+json:
              schema:
                $ref: '#/components/schemas/UnsignedAppStateManifest'
        '304':
          description: Not Modified - Manifest has not changed
        '406':
          description: Not Acceptable - Server cannot generate a response matching the Accept header
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '500':
          description: Internal Server Error
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
  /api/v1/clients/{clientId}/deployments/{deploymentId}/{digest}:
    get:
      summary: Retrieve an individual ApplicationDeployment YAML file
      security:
        - PayloadSignature: []
      description: >
        This endpoint is used by the client to fetch the YAML for a single ApplicationDeployment after it has processed a new State Manifest and identified a small number of new or updated deployments. This allows for highly efficient, incremental updates without needing to download the full bundle.
        To make individual workload retrievals race-free and cache-friendly, this endpoint is content-addressable: the digest of the expected YAML is part of the URL. This guarantees immutability of the fetched resource and prevents a time-of-check / time-of-use race where a deployment changes between manifest retrieval and content fetch.
      parameters:
        - name: clientId
          in: path
          required: true
          schema:
            type: string
          description: Unique identifier of the Edge Compute Device
        - name: deploymentId
          in: path
          required: true
          schema:
            type: string
          description: Unique identifier for the application deployment
        - name: digest
          in: path
          required: true
          schema:
            type: string
          description: >
            Content-addressable digest of the ApplicationDeployment YAML file. MUST conform to the Digest Specification and MUST equal the digest computed over the exact sequence of bytes (per Exact Bytes Rule) in the HTTP 200 OK response body. If the server cannot produce content whose digest matches this value it MUST return 404 Not Found.
        - name: If-None-Match
          in: header
          required: false
          schema:
            type: string
          description: >
            Optional ETag for caching. The ETag is returned to the client from the /deployments endpoint, it is the digest of the state manifest.
        - name: Accept-Encoding
          in: header
          required: false
          schema:
            type: string
          description: Indicates supported compression formats (e.g., gzip, br)
      responses:
        '200':
          description: >
            The response body is the raw ApplicationDeployment YAML file (Content-Type: application/yaml). The content MUST match the {digest} path segment; the server MUST return 404 if it does not have the exact digest referenced.
          headers:
            Content-Type:
              schema:
                type: string
              description: application/yaml
            ETag:
              schema:
                type: string
              description: >
                The ETag is returned to the client from the /deployments endpoint, it is the digest of the state manifest.
            Cache-Control:
              schema:
                type: string
              description: public, max-age=31536000, immutable
            Vary:
              schema:
                type: string
              description: Accept-Encoding
          content:
            application/yaml:
              schema:
                type: string
                description: Raw YAML content of the ApplicationDeployment
        '304':
          description: Not Modified
        '404':
          description: Not Found
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '500':
          description: Internal Server Error
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
  /api/v1/clients/{clientId}/deployments/{deploymentId}/status:
    post:
      summary: Report deployment status
      security:
        - PayloadSignature: []
      parameters:
        - name: clientId
          in: path
          required: true
          schema:
            type: string
        - name: deploymentId
          in: path
          required: true
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/DeploymentStatusManifest'
      responses:
        '200':
          description: The deployment status was added, or updated, successfully.
        '400':
          description: Bad Request
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '401':
          description: Unauthorized — Signature verification failed
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '403':
          description: Forbidden — Certificate not trusted or client rejected
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '422':
          description: Unprocessable Entity — Semantic error in request body
          content:
            application/problem+json:
              schema:
                $ref: '#/components/schemas/ProblemDetail'
        '500':
          description: Internal Server Error
          content:
            application/problem+json:
              schema:
                $ref: "#/components/schemas/ProblemDetail"
components:
  securitySchemes:
    # TODO: fix this as we are following RFC 9421, instead of a custom signature header field
    PayloadSignature:
      type: apiKey
      in: header
      name: X-Payload-Signature
      description: >
        Base64-encoded payload signature using SHA-256 and device certificate.
        Format: public_key;digital_signature
  schemas:
    ProblemDetail:
      type: object
      description: >
        RFC 9457 Problem Details for HTTP APIs. Returned with Content-Type:
        application/problem+json. See `https://www.rfc-editor.org/rfc/rfc9457`
      required:
        - type
        - title
        - status
      properties:
        type:
          type: string
          format: uri
          description: >
            URI reference that identifies the problem type. Consumers can
            dereference this URI for human-readable documentation.
          example: "`https://margo.org/problems/invalid-certificate`"
        title:
          type: string
          description: Short human-readable summary of the problem type.
          example: Invalid Certificate
        status:
          type: integer
          description: HTTP status code.
          example: 400
        detail:
          type: string
          description: >
            Human-readable explanation specific to this occurrence of the
            problem.
          example: Certificate signature verification failed
        instance:
          type: string
          format: uri
          description: |
            URI reference identifying the specific occurrence of the problem.
          example: /api/v1/onboarding
      additionalProperties: true
    ManifestVersion:
      type: number
      description: >
        Monotonically increasing unsigned 64-bit integer in the inclusive range [1, 2^64-1].
        Prevents rollback attacks. The first manifest MUST use 1.
    DeploymentBundleRef:
      type: object
      nullable: true
      description: >
        Describes a single archive containing all ApplicationDeployment documents. If there are zero deployments (deployments array is empty) the property MUST be present with the value null (it MUST NOT be omitted).
      properties:
        mediaType:
          type: string
          description: >
            MUST be application/vnd.margo.bundle.v1+tar+gzip; a gzip-compressed tar whose root contains one or more ApplicationDeployment YAML files. If there are zero deployments then bundle MUST be null (an empty archive MUST NOT be served). The archive MUST contain exactly the set of YAML files referenced by deployments.
        digest:
          type: string
          description: >
            The digest of the bundle archive. MUST equal the digest computed over the exact sequence of bytes (per Exact Bytes Rule) in the bundle endpoint's HTTP 200 OK response body.
        sizeBytes:
          type: number
          description: >
            Unsigned 64-bit advisory estimate of the decoded payload length in bytes for the bundle archive. Provided for bandwidth estimation and update planning. MUST NOT be used for integrity; digest verification remains mandatory.
        url:
          type: string
          description: >
            Content-addressable retrieval endpoint of the form /api/v1/clients/{clientId}/bundles/{digest} where {digest} equals bundle.digest.
    DeploymentManifestRef:
      type: object
      description: >
        Reference to a deployment manifest with content addressing and integrity verification.
      required:
        - deploymentId
        - digest
        - url
      properties:
        deploymentId:
          type: string
          description: >
            Unique identifier for the application deployment.
        digest:
          type: string
          description: >
            The digest of the individual ApplicationDeployment YAML file. MUST equal the digest computed over the exact sequence of bytes (per Exact Bytes Rule) in that deployment endpoint's HTTP 200 OK response body.
        sizeBytes:
          type: number
          description: >
            Unsigned 64-bit advisory estimate of the decoded payload length in bytes for the deployment YAML. Provided for planning or progress display. MUST NOT be used for integrity; digest verification remains mandatory.
        url:
          type: string
          description: >
            Content-addressable endpoint of the form /api/v1/clients/{clientId}/deployments/{deploymentId}/{digest}. The {digest} MUST equal deployments[].digest; the referenced resource is immutable
    UnsignedAppStateManifest:
      type: object
      required:
        - manifestVersion
        - bundle
        - bundle.mediaType
        - bundle.digest
        - bundle.url
        - deployments
      properties:
        manifestVersion:
          $ref: '#/components/schemas/ManifestVersion'
        bundle:
          $ref: '#/components/schemas/DeploymentBundleRef'
        deployments:
          type: array
          description: A list of deployment object references for the device. The reference contains some meta info and reference to the url where the deployment is available.
          items:
            $ref: '#/components/schemas/DeploymentManifestRef'
    DeviceCapabilitiesManifest:
      type: object
      required: [apiVersion, kind, properties]
      properties:
        apiVersion:
          type: string
        kind:
          type: string
          enum: [DeviceCapabilitiesManifest]
        properties:
          type: object
          required: [id, vendor, modelNumber, serialNumber, roles]
          properties:
            id:
              $ref: '#/components/schemas/DeviceId'
            vendor:
              type: string
            modelNumber:
              type: string
            serialNumber:
              type: string
            roles:
              type: array
              items:
                type: string
                enum: [Standalone Cluster, Cluster Leader, Standalone Device, Gateway]
            resources:
              type: object
              required: [cpu, memory, storage, peripherals, interfaces]
              properties:
                cpu:
                  type: object
                  required: [cores]
                  properties:
                    cores:
                      type: number
                    architecture:
                      type: string
                      enum: [amd64, arm64, arm]
                memory:
                  type: string
                storage:
                  type: string
                peripherals:
                  type: array
                  items:
                    $ref: '#/components/schemas/DevicePeripheral'
                interfaces:
                  type: array
                  items:
                    $ref: '#/components/schemas/DeviceCommunicationInterface'
    DeviceId:
      # format: "{id}[/{id}[/{id}...]]"
      # Top-level id is required and must include only unreserved characters as specified in RFC3986.
      # Subsequent ids are only used when referencing child devices, and must include only unreserved characters as specified in RFC3986 when present.
      type: string
      pattern: '^[A-Za-z0-9._~-]+(\/[A-Za-z0-9._~-]+)*$'
    DeviceId_with_asterisk:
      # format: "{id}[/{id}[/{id}...]/*]"
      # Top-level id is required and must include only unreserved characters as specified in RFC3986.
      # Subsequent ids are only used when referencing child devices, and must include only unreserved characters as specified in RFC3986 when present.
      type: string
      pattern: '^[A-Za-z0-9._~-]+(\/[A-Za-z0-9._~-]+)*(\/\*)?$'

    ComponentStatus:
      type: object
      required: [name, state]
      properties:
        name:
          type: string
        state:
          type: string
          enum: [pending, installing, installed, failed, removing, removed]
        error:
          type: object
          properties:
            code:
              type: string
            source:
              type: string
            message:
              type: string

    DeploymentStatusManifest:
      type: object
      required: [apiVersion, kind, deploymentId, status, components]
      properties:
        apiVersion:
          type: string
        kind:
          type: string
          enum: [DeploymentStatusManifest]
        deploymentId:
          type: string
        deviceId:
          $ref: '#/components/schemas/DeviceId'
        status:
          type: object
          required: [state]
          properties:
            state:
              type: string
              enum: [pending, installing, installed, failed, removing, removed]
            error:
              type: object
              properties:
                code:
                  type: string
                source:
                  type: string
                message:
                  type: string
        components:
          type: array
          items:
            $ref: '#/components/schemas/ComponentStatus'

    DevicePeripheral:
      type: object
      required: [type]
      properties:
        type:
          type: string
          enum: [gpu, display, camera, microphone, speaker]
        manufacturer:
          type: string
        model:
          type: string

    DeviceCommunicationInterface:
      type: object
      required: [type]
      properties:
        type:
          type: string
          enum: [ethernet, wifi, cellular, bluetooth, usb, canbus, rs232]
    # app deployment struct added here for ease of programming, the code generators will generate the structs
    # for the actual app deployment yaml and parsing would be easy to do
    appDeploymentManifest:
      type: object
      description: Application Deployment manifest
      required: [apiVersion, kind, metadata, spec]
      properties:
        apiVersion:
          type: string
          default: margo.org
          description: API version
        kind:
          type: string
          default: ApplicationDeployment
          description: Resource kind
        id:
          type: string
          description: Unique identifier for the application deployment
        metadata:
          $ref: '#/components/schemas/appDeploymentMetadata'
        spec:
          $ref: '#/components/schemas/appDeploymentSpec'
    appDeploymentMetadata:
      type: object
      required: [annotations, name, namespace, deviceId]
      properties:
        name:
          type: string
          description: Name of the resource
        namespace:
          type: string
          description: Namespace of the resource
        deviceId:
          $ref: '#/components/schemas/DeviceId_with_asterisk'
          description: Device ID of the target device for the deployment
        labels:
          type: object
          additionalProperties: { type: string }
          description: Labels for the resource
    helmApplicationDeploymentProfileComponent:
      type: object
      description: Helm Application Deployment Profile Component
      required: [name, properties]
      properties:
        name:
          type: string
          description: Name of the component
        properties:
          type: object
          required: [repository]
          properties:
            repository:
              type: string
              description: Repository of the component
            revision:
              type: string
              description: Revision of the component
            timeout:
              type: string
              description: Timeout for the component
            wait:
              type: boolean
              description: Wait for the component to be ready
    composeApplicationDeploymentProfileComponent:
      type: object
      description: Compose Application Deployment Profile Component
      required: [name, properties]

      properties:
        name:
          type: string
          description: Name of the component
        properties:
          type: object
          required: [packageLocation]
          properties:
            packageLocation:
              type: string
              description: The URL indicating the Compose package's location. It should be a direct path to the compose.yaml or compose.yaml file archived in tar.gz
            keyLocation:
              type: string
              description: Key location of the component
            timeout:
              type: string
              description: Timeout for the component
            wait:
              type: boolean
              description: Wait for the component to be ready
    appDeploymentProfile:
      type: object
      description: Application Deployment Profile
      required: [type, components]
      properties:
        type:
          type: string
          enum: ["helm", "compose"]
          description: Type of deployment profile
        components:
          type: array
          items:
            oneOf:
              - $ref: '#/components/schemas/helmApplicationDeploymentProfileComponent'
              - $ref: '#/components/schemas/composeApplicationDeploymentProfileComponent'
          description: Components of the deployment profile
    appParameterTarget:
      type: object
      description: Application Parameter Target
      required: [pointer, components]
      properties:
        pointer:
          type: string
          description: Pointer to the parameter
        components:
          type: array
          items:
            type: string
          description: Components of the parameter
    appParameterValue:
      type: object
      description: Application Parameter Value
      required: [value, targets]
      properties:
        value:
          description: Value of the parameter
          additionalProperties: true
          x-go-type: interface{}
        targets:
          type: array
          items:
            $ref: '#/components/schemas/appParameterTarget'
          description: Targets of the parameter
    appDeploymentParams:
      type: object
      description: Application Parameters
      additionalProperties:
        $ref: '#/components/schemas/appParameterValue'
    appDeploymentSpec:
      type: object
      description: Application Deployment specification
      required: [applicationId, deploymentProfile]
      properties:
        applicationId:
          type: string
          description: >-
            An identifier for the application.
            The id is used to help create unique identifiers where required, such as namespaces.
            The id must be lower case letters and numbers and MAY contain dashes.
            Uppercase letters, underscores and periods MUST NOT be used.
            The id MUST NOT be more than 200 characters.
            The applicationId MUST match the associated application description's top-level "id" attribute.
          pattern: "^[-a-z0-9]{1,200}$"
        deploymentProfile:
          $ref: '#/components/schemas/appDeploymentProfile'
          description: Deployment profile
        parameters:
          $ref: '#/components/schemas/appDeploymentParams'
          description: Parameters for the deployment
```

---

### Content-Type

All error responses MUST use the `application/problem+json` media type as specified in RFC 9457.
Servers MUST NOT return `application/json` for error responses.

---

## Technical Acceptance Criteria

- **AC1**: All error responses MUST use `Content-Type: application/problem+json`
- **AC2**: `type` field MUST be a stable URI from the Stable Type URI Registry above
- **AC3**: `type`, `title`, and `status` are REQUIRED fields on all error responses
- **AC4**: `429` and `503` responses MUST include `Retry-After` response header
- **AC5**: `500` responses SHOULD include `Retry-After` response header
- **AC6**: `retryable` field MUST be present on all error responses
- **AC7**: `backoffStrategy` field MUST be present on all error responses
- **AC8**: `422` responses MUST include `errors[]` array with field-level validation details
- **AC9**: All endpoints in the OpenAPI spec MUST declare all applicable error responses
- **AC10**: Clients MUST use `type` field for programmatic error handling, NOT `status` or `title`
- **AC11**: Clients MUST respect `Retry-After` header and MUST NOT retry before it elapses

---

## Alternatives considered

1. **Custom error response format** - Rejected because it would not conform to industry standards
   and reduce interoperability with client libraries that expect RFC 9457 format.

2. **Minimal error responses** - Rejected because it provides insufficient error context for API
   consumers to properly handle and debug issues.

3. **Different error standard (e.g., HAL, JSON:API)** - Rejected in favor of RFC 9457 as it is
   the most recent IETF standard specifically designed for HTTP APIs.

4. **Inline error responses per endpoint** - Rejected in favour of shared `$ref` components to
   avoid duplication and ensure consistency across all endpoints.

5. **No retry semantics** - Rejected because edge devices operating in constrained or intermittent
   network environments require clear retry guidance to implement consistent automation behaviour.

## Related PRs

- margo/specification PR #188: "feat: adds rfc9457 details to openapi spec"
