# Specification Update Proposal

## Owner

[@matlec](https://github.com/matlec) (currently deferred — owner to be confirmed at promotion)

## Summary

Defines the MIS Revocation List endpoint and the JSON-based revocation model under MIAF. Allows verifiers to discover revoked SVIDs in advance of natural expiry. Originally drafted as part of the active [MIAF SUP](../margo-identity-and-authorization-framework.md); deferred for PR 2 in favor of relying on short SVID lifetimes (recommended <= 90 days for online devices) for revocation-by-expiry. Purely additive — adding a `revocationListUri` field to the discovery document later does not break existing clients, since the discovery document is extensible and clients are required to ignore unknown fields.

## Reason for proposal

Short-lived SVIDs handle most revocation needs by attrition; an explicit revocation list endpoint becomes valuable when long-lived credentials, suspected compromise, or cross-component invalidation timelines exceed natural expiry windows. Standardizing the endpoint and JSON revocation list format ensures every conformant verifier can consume revocation state consistently.

## Requirements alignment acknowledgement

This SUP extends the active [MIAF SUP](../margo-identity-and-authorization-framework.md). Detailed feature linkage and Owner are TBD at promotion.

## Technical proposal

### 1. Revocation List Endpoint

This endpoint provides a **machine-readable list of revoked SVIDs** within the Trust Domain maintained by the Margo Identity Service (MIS).

This SUP standardizes publication and verifier consumption of revocation state through this endpoint. The privileged workflow by which an operator, fleet tool, or MIS implementation decides to revoke or terminate an identity remains deployment-specific and is out of scope.

Clients and services use it to check SVID status and enforce revocation, without relying solely on traditional PKI mechanisms such as CRLs or OCSP.

| Item | Value |
| :--- | :---- |
| **Endpoint** | `GET /api/v1/revocations` |
| **Authentication** | None (public resource, HTTPS required) |
| **Headers** | `Accept: application/json` |
| **Body schema (request)** | None |
| **Body schema (response)** | See below |
| **Responses** | `200 OK` - list of revoked SVIDs<br>`304 Not Modified` - list unchanged<br>`404 Not Found` - revocation list unavailable |
| **Errors** | RFC 9457 Problem Details as per [Appendix B](../margo-identity-and-authorization-framework.md#appendix-b-error-responses-normative) |

**Response body schema (`200 OK`, `application/json`):**

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `lastUpdated` | string (ISO 8601) | Y | UTC timestamp when the list was last generated. Used by clients to detect updates. |
| `revokedSvids` | array of object | Y | Array of revoked SVID records. Clients **MUST** treat this list as authoritative for the trust domain. |
| `revokedSvids.certFingerprintSha256` | string | Y | Lowercase hexadecimal representation of the SHA-256 digest of the DER-encoded **leaf X.509 SVID certificate** (no prefixes or delimiters). Consumers **MUST** use this value as the primary identifier when checking revocation status. |
| `revokedSvids.serialNumber` | string | Y | Uppercase hexadecimal representation of the X.509 certificate serial number, without prefixes or delimiters. |
| `revokedSvids.revokedAt` | string (ISO 8601) | Y | UTC timestamp when the SVID was revoked. Consumers **SHOULD** ignore entries with timestamps in the future. |

This list covers **X.509 SVIDs** (identified by the leaf certificate SHA-256 fingerprint). Future SUPs that register additional SVID profiles (for example, a JWT-SVID profile) may extend or replace this representation as appropriate to those profiles.

**Revocation matching rules:**

- Consumers **MUST** compute the SHA-256 digest of the DER-encoded **leaf** X.509 SVID certificate they are validating, encode it as lowercase hex (no delimiters), and compare it to `certFingerprintSha256` entries.
- Consumers **MUST NOT** use `serialNumber` as the sole revocation identifier, because serial numbers are not globally unique across issuers.

> **Informative:**
> The MIS **SHOULD** support standard caching headers (`ETag`, `Last-Modified`) to allow efficient synchronization. Clients **SHOULD** periodically poll this endpoint or subscribe to a trust-bundle update mechanism to maintain up-to-date revocation data.

#### Example: Revocation List Retrieval

**Example request:**

```http
GET /api/v1/revocations
Accept: application/json
```

**Example response (`200 OK`):**

```jsonc
{
  "lastUpdated": "2025-10-25T14:12:31Z",
  "revokedSvids": [
    {
      "certFingerprintSha256": "2f4c3d9a7b1e0c6d8f2a1b9c0d7e6f5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c0d9e",
      "serialNumber": "8F12A4C9D23E41B1",
      "revokedAt": "2025-10-20T09:33:45Z"
    },
    {
      "certFingerprintSha256": "9c1d0e2f3a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5",
      "serialNumber": "A74E91F1B8CC4092",
      "revokedAt": "2025-10-21T17:58:11Z"
    }
  ]
}
```

**Example response (`304 Not Modified`):**

```http
HTTP/1.1 304 Not Modified
ETag: "revlist-6f82bcd"
Last-Modified: Sat, 25 Oct 2025 14:12:31 GMT
```

> **Informative:**
> Clients and services validating SVIDs should periodically refresh this list to ensure that revoked identities cannot be used for authentication. Deployments that implement OAuth2 integration as described in the [MIAF OAuth2 Bridge SUP](./miaf-oauth2-bridge.md) **SHOULD** also consult this list when mapping SVIDs into other token forms.

### 2. Revocation Model

The Margo Identity Service (MIS) **MUST** maintain a consistent revocation model to ensure that compromised or decommissioned identities cannot be used for authentication.

1. **Short-lived credentials (primary containment)**

   - X.509 SVID lifetimes **MUST** be limited according to [Cryptographic Requirements](../margo-identity-and-authorization-framework.md#cryptographic-requirements) and profile-specific guidance.
   - Clients and servers **MUST** reject expired SVIDs and **MUST NOT** cache authorization decisions beyond the SVID lifetime.

2. **Margo-native revocation list (secondary defense)**

    - MIS **MUST** maintain a JSON-based revocation list of revoked SVIDs for the Trust Domain.
    - Consumers **MUST** match revoked X.509 SVIDs using the leaf certificate SHA-256 fingerprint (`certFingerprintSha256`).
    - Clients **SHOULD** use HTTP caching semantics (`ETag`, `If-None-Match`, `Last-Modified`, `If-Modified-Since`) when supported by MIS to minimize bandwidth.

3. **Standard PKI revocation (optional)**

   - Deployments integrating with external CAs (see [MIS Deployment Modes](../margo-identity-and-authorization-framework.md#mis-deployment-modes-informative)) **MAY** also rely on standard PKI revocation mechanisms (for example, OCSP, CRLs).
   - These are **optional** extensions for hybrid or enterprise environments.

> **Scalability Note (Informative):**
> For the scale of early adoption and GA (tens of thousands of devices), a cached JSON list provides the best balance of reliability, simplicity, and offline resilience. For hyper-scale deployments (millions of devices), future versions of this specification **MAY** introduce differential updates (e.g., CRLite) or OCSP support.

## Alternatives considered

TBD

## Rejection reason

