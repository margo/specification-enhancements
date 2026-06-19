# Specification Update Proposal

## Owner

@vireshnavalli

## Summary

Add comprehensive common error responses for 4xx and 5xx HTTP status codes to all API routes following RFC 9457 (Problem Details for HTTP APIs). This standardizes error handling across the margo specification and provides consistent error information to API consumers.

## Reason for proposal

RFC 9457 defines a standard format for problem details in HTTP responses, enabling consistent error handling across REST APIs. Implementing this across all margo API routes provides:

1. **Standardization** - Consistent error response format across all endpoints
2. **API Consumer Experience** - Clear, predictable error information
3. **RFC Compliance** - Adherence to industry standard (RFC 9457)
4. **Better Error Handling** - Enables robust client-side error handling with standard fields
5. **Specification Clarity** - Addresses specification issue #153

## Requirements alignment acknowledgement

This SUP aligns with margo's commitment to following industry standards and best practices for REST API design. RFC 9457 is an IETF-approved standard for problem details in HTTP APIs, ensuring our specification remains modern and interoperable.

**Related Feature(s):** [Issue #153 - margo/specification](https://github.com/margo/specification/issues/153)

## Technical proposal

### RFC 9457 Error Response Structure

Implement the following standard error response format based on RFC 9457:

```yaml
ProblemDetails:
  type: object
  properties:
    type:
      type: string
      format: uri
      description: A URI reference identifying the problem type
      example: "https://api.margo.io/errors/invalid-request"
    title:
      type: string
      description: A short, human-readable summary of the problem
      example: "Invalid Request"
    status:
      type: integer
      description: The HTTP status code
      example: 400
    detail:
      type: string
      description: A human-readable explanation of the specific occurrence
      example: "Missing required field 'name'"
    instance:
      type: string
      format: uri
      description: A URI reference identifying the specific occurrence
```

### Common Error Responses

All API routes must include standardized error responses:

#### 4xx Client Errors

- **400 Bad Request**: Invalid request parameters or body
- **401 Unauthorized**: Missing or invalid authentication
- **403 Forbidden**: Authenticated but insufficient permissions
- **404 Not Found**: Requested resource does not exist
- **409 Conflict**: Request conflicts with current state
- **422 Unprocessable Entity**: Validation error with semantic meaning

#### 5xx Server Errors

- **500 Internal Server Error**: Unexpected server error
- **501 Not Implemented**: Feature not yet implemented
- **503 Service Unavailable**: Server temporarily unable to handle request

### OpenAPI Specification Updates

All endpoints in the OpenAPI specification must include:

```yaml
responses:
  '400':
    description: Bad Request
    content:
      application/problem+json:
        schema:
          $ref: '#/components/schemas/ProblemDetails'
  '401':
    description: Unauthorized
    content:
      application/problem+json:
        schema:
          $ref: '#/components/schemas/ProblemDetails'
  '403':
    description: Forbidden
    content:
      application/problem+json:
        schema:
          $ref: '#/components/schemas/ProblemDetails'
  '404':
    description: Not Found
    content:
      application/problem+json:
        schema:
          $ref: '#/components/schemas/ProblemDetails'
  '500':
    description: Internal Server Error
    content:
      application/problem+json:
        schema:
          $ref: '#/components/schemas/ProblemDetails'
  '503':
    description: Service Unavailable
    content:
      application/problem+json:
        schema:
          $ref: '#/components/schemas/ProblemDetails'
```

### Content-Type

Error responses should use the `application/problem+json` media type as specified in RFC 9457.

## Alternatives considered

1. **Custom error response format** - Rejected because it would not conform to industry standards and reduce interoperability with client libraries that expect RFC 9457 format.

2. **Minimal error responses** - Rejected because it provides insufficient error context for API consumers to properly handle and debug issues.

3. **Different error standard (e.g., HAL, JSON:API)** - Rejected in favor of RFC 9457 as it is the most recent IETF standard specifically designed for HTTP APIs.

## Related PRs

- margo/specification PR #188: "feat: adds rfc9457 details to openapi spec"
