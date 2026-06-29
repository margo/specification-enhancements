# Specification Update Proposal

## Owner

@vireshnavalli

## Summary

Add comprehensive common error responses for 4xx and 5xx HTTP status codes to all API routes
following [RFC 9457 Problem Details for HTTP APIs](https://datatracker.ietf.org/doc/html/rfc9457) . This standardizes error handling across the
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
members (§3.2) for retry semantics and field-level validation errors:

```yaml
ProblemDetail:
  type: object
  description: >
    RFC 9457 Problem Details for HTTP APIs.
    Returned with Content-Type: application/problem+json.
    See https://www.rfc-editor.org/rfc/rfc9457

    Extension members (RFC 9457 §3.2) are permitted via additionalProperties.
    Vendors MAY add custom fields alongside standard margo fields.
    Vendor-specific problem types MUST use their own URI namespace —
    the https://margo.org/problems/ namespace is reserved for this specification.
  required:
    - type
    - title
    - status
  properties:
    type:
      type: string
      format: uri
      description: >
        URI reference identifying the problem type.

        Margo implementations MUST use a URI from the Stable Type URI Registry
        (https://margo.org/problems/*).

        Vendor implementations MAY use their own URI namespace
        (e.g. https://vendor.example.com/problems/sensor-fault).
        Vendor URIs MUST NOT use the https://margo.org/problems/ namespace.

        Clients MUST use this field for programmatic error handling,
        NOT the status code or title.
        Clients encountering an unknown type URI SHOULD fall back to
        title, detail, and status for display and handling.
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
  # additionalProperties: true permits RFC 9457 §3.2 extension members.
  # Margo extension fields: retryable, retryAfterSeconds, backoffStrategy, errors
  # Vendor extension examples:
  #   x-vendor-component: "sensor-subsystem"
  #   x-vendor-error-code: "ERR_4291"
  # Vendor extension fields SHOULD be prefixed with x- to avoid
  # collision with future margo-defined fields.

```

---

### Stable Type URI Registry

The following `type` URI values are stable and MUST be used by all implementations.
Clients MUST use the `type` field for programmatic error handling.


**Registry Governance**

> **Note:** RFC 9457 does not define versioning, deprecation, or extension namespace
> rules — these are intentionally left to the implementing specification.
> The following rules apply to the margo problem type registry:

**URI Stability and Versioning**

Type URIs are **permanent stable identifiers and are intentionally unversioned**,
consistent with RFC 9457 design intent and industry practice (Google APIs, Stripe, AWS).
The URI identifies the error *concept*, not the API version.

- `https://margo.org/problems/invalid-certificate` means "the certificate was invalid"
  in v1, v2, and all future versions — the concept does not change between API versions
- If an error concept changes significantly, a **new URI is added** and the old one
  **deprecated** — both remain valid during the transition period
- Existing URIs MUST NOT be repurposed or have their semantics changed
- Clients MUST treat each `type` URI as an opaque stable string

**Vendor Extensions**

RFC 9457 §3.2 explicitly supports vendor-specific problem type URIs — there is no
central registry. Suppliers MAY define additional problem types using their own URI
namespace:

- Vendor URIs MUST use the supplier's own domain
  (e.g. `https://vendor.example.com/problems/sensor-fault`)
- Vendor URIs MUST NOT use the `https://margo.org/problems/` namespace,
  which is reserved for this specification
- Clients encountering an unknown `type` URI SHOULD fall back to using
  `title` and `detail` fields for display, and `status` for HTTP-level handling

**Deprecations**

- Deprecated URIs are marked with `(deprecated)` in this registry
- Deprecated URIs remain valid for a minimum of two major specification versions
- A replacement URI MUST be listed alongside any deprecated URI



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

All endpoints MUST declare applicable error responses using `$ref` to the shared
`ProblemDetail` schema.

#### Pattern — error response declaration

```yaml
# Every applicable error response follows this pattern:
'400':
  description: Bad Request
  content:
    application/problem+json:
      schema:
        $ref: '#/components/schemas/ProblemDetail'
```

#### Pattern — 429/503 with required `Retry-After` header

```yaml
'429':
  description: Too Many Requests
  headers:
    Retry-After:
      required: true
      schema:
        type: integer
      description: Seconds the client MUST wait before retrying.
  content:
    application/problem+json:
      schema:
        $ref: '#/components/schemas/ProblemDetail'

'503':
  description: Service Unavailable
  headers:
    Retry-After:
      required: true
      schema:
        type: integer
      description: Seconds before WFM is expected to be available again.
  content:
    application/problem+json:
      schema:
        $ref: '#/components/schemas/ProblemDetail'
```

#### Example — `/api/v1/onboarding` POST with all applicable errors

```yaml
/api/v1/onboarding:
  post:
    responses:
      '201':
        description: New client onboarded successfully.
      '400':
        description: Bad Request — invalid certificate format.
        content:
          application/problem+json:
            schema:
              $ref: '#/components/schemas/ProblemDetail'
      '403':
        description: Forbidden — certificate not trusted or client rejected.
        content:
          application/problem+json:
            schema:
              $ref: '#/components/schemas/ProblemDetail'
      '429':
        description: Too Many Requests.
        headers:
          Retry-After:
            required: true
            schema:
              type: integer
        content:
          application/problem+json:
            schema:
              $ref: '#/components/schemas/ProblemDetail'
      '500':
        description: Internal Server Error.
        content:
          application/problem+json:
            schema:
              $ref: '#/components/schemas/ProblemDetail'
      '503':
        description: Service Unavailable.
        headers:
          Retry-After:
            required: true
            schema:
              type: integer
        content:
          application/problem+json:
            schema:
              $ref: '#/components/schemas/ProblemDetail'
```

#### `ProblemDetail` schema — defined once in `components/schemas`

```yaml
components:
  schemas:
    ProblemDetail:
      type: object
      required: [type, title, status]
      properties:
        type:
          type: string
          format: uri
          example: https://margo.org/problems/invalid-certificate
        title:
          type: string
          example: Invalid Certificate
        status:
          type: integer
          example: 400
        detail:
          type: string
          example: Certificate is not valid .
        instance:
          type: string
          format: uri
          example: /api/v1/onboarding
        retryable:
          type: boolean
          example: false
        retryAfterSeconds:
          type: integer
          example: 30
        backoffStrategy:
          type: string
          enum: [none, fixed, exponential]
          example: exponential
        errors:
          type: array
          items:
            type: object
            properties:
              field:
                type: string
              message:
                type: string
      additionalProperties: true
```

---

### Content-Type

All error responses MUST use the `application/problem+json` media type as specified in RFC 9457.
Servers MUST NOT return `application/json` for error responses.

---

## Technical Acceptance Criteria

- **AC1**: All error responses SHOULD use `Content-Type: application/problem+json`
- **AC2**: `type` field SHOULD be a stable URI from the Stable Type URI Registry above
- **AC3**: `type`, `title`, and `status` are REQUIRED fields on all error responses
- **AC4**: `429` and `503` responses SHOULD include `Retry-After` response header
- **AC5**: `500` responses SHOULD include `Retry-After` response header
- **AC6**: `retryable` field SHOULD be present on all error responses
- **AC7**: `backoffStrategy` field SHOULD be present on all error responses
- **AC8**: `422` responses SHOULD include `errors[]` array with field-level validation details
- **AC9**: All endpoints in the OpenAPI spec SHOULD declare all applicable error responses
- **AC10**: Clients SHOULD use `type` field for programmatic error handling, NOT `status` or `title`
- **AC11**: Clients SHOULD respect `Retry-After` header and MUST NOT retry before it elapses

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
