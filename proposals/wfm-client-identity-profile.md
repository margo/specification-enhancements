# WFM Client Identity Profile and Margo Management Interface Update

- [WFM Client Identity Profile and Margo Management Interface Update](#wfm-client-identity-profile-and-margo-management-interface-update)
  - [Owner](#owner)
  - [Summary](#summary)
  - [Reason for proposal](#reason-for-proposal)
  - [Requirements alignment acknowledgement](#requirements-alignment-acknowledgement)
  - [Technical proposal](#technical-proposal)
    - [1. Scope and Structure](#1-scope-and-structure)
    - [2. Terminology](#2-terminology)
    - [3. WFM Client Identity Model](#3-wfm-client-identity-model)
      - [3.1 WFM server identity](#31-wfm-server-identity)
      - [3.2 WFM Client identity format](#32-wfm-client-identity-format)
      - [3.3 Identity representations](#33-identity-representations)
      - [3.4 Recognition by the WFM](#34-recognition-by-the-wfm)
    - [4. Bootstrap and Enrollment](#4-bootstrap-and-enrollment)
      - [4.1 Bootstrap method identifier](#41-bootstrap-method-identifier)
      - [4.2 Actor model](#42-actor-model)
      - [4.3 WFM binding-assertion request endpoint](#43-wfm-binding-assertion-request-endpoint)
      - [4.4 Enrollment request](#44-enrollment-request)
      - [4.5 WFM Client Binding Assertion](#45-wfm-client-binding-assertion)
      - [4.6 Enrollment Subject Identifier derivation](#46-enrollment-subject-identifier-derivation)
      - [4.7 MIS validation rules](#47-mis-validation-rules)
      - [4.8 Stateless WFM candidate issuance](#48-stateless-wfm-candidate-issuance)
    - [5. Lifecycle Rules (Standalone)](#5-lifecycle-rules-standalone)
      - [5.1 Lifecycle phases](#51-lifecycle-phases)
      - [5.2 Standalone topology](#52-standalone-topology)
      - [5.3 Renewal and disconnected operation](#53-renewal-and-disconnected-operation)
    - [6. Authentication and Authorization at the WFM API](#6-authentication-and-authorization-at-the-wfm-api)
      - [6.1 Authentication model](#61-authentication-model)
      - [6.2 Canonical caller identity](#62-canonical-caller-identity)
      - [6.3 Authorization](#63-authorization)
    - [7. Application to the Margo Management Interface](#7-application-to-the-margo-management-interface)
      - [7.1 Endpoints removed](#71-endpoints-removed)
      - [7.2 Endpoint redefined](#72-endpoint-redefined)
      - [7.3 Endpoints retained, with `{clientId}` path parameter dropped](#73-endpoints-retained-with-clientid-path-parameter-dropped)
    - [8. Security and Operational Considerations](#8-security-and-operational-considerations)
      - [8.1 Trust between MIS and WFM](#81-trust-between-mis-and-wfm)
      - [8.2 Replay and DoS controls](#82-replay-and-dos-controls)
    - [9. Typical Workflows (Informative)](#9-typical-workflows-informative)
      - [9.1 Standalone device](#91-standalone-device)
  - [Alternatives considered (optional)](#alternatives-considered-optional)
    - [Reusing the device identity directly](#reusing-the-device-identity-directly)
    - [Treating the WFM Client as only a generic workload identity](#treating-the-wfm-client-as-only-a-generic-workload-identity)
    - [Keeping RFC 9421 HTTP Message Signatures](#keeping-rfc-9421-http-message-signatures)
    - [Requiring MIS-to-WFM validation calls](#requiring-mis-to-wfm-validation-calls)
  - [Rejection reason](#rejection-reason)


## Owner

[@matlec](https://github.com/matlec)

## Summary

This SUP defines the initial **WFM Client Identity Profile** for Margo, building on the **[Margo Identity and Authorization Framework (MIAF)](./margo-identity-and-authorization-framework.md)**, and updates the **Margo Management Interface for Workload Fleet Management** to consume MIAF-aligned identities. The WFM Client gets its own Trust Domain-scoped identity, distinct from the Edge Compute Device identity.

It replaces three PR1 mechanisms: the WFM-assigned `clientId` carried in URL paths, the per-WFM onboarding and certificate-distribution endpoints (`POST /api/v1/onboarding` and `GET /api/v1/onboarding/certificate`), and the RFC 9421 HTTP Message Signatures security scheme that was used for client authentication. Caller identity becomes the authenticated SPIFFE ID carried in a WFM Client X.509-SVID presented via mTLS, issued by the **Margo Identity Service (MIS)** with no runtime MIS-to-WFM interaction.

This SUP covers deployments where the WFM Client credential is held by exactly one entity — a single edge device, a single-node cluster, or a gateway box that manages other devices behind it. Deployments where the credential is shared across several cooperating nodes that take over from each other — typically a multi-node Kubernetes cluster with leader failover — are out of scope and deferred to a follow-up SUP (see [`deferred/miaf-multi-holder-identities-and-cluster-topology.md`](./deferred/miaf-multi-holder-identities-and-cluster-topology.md)).

## Reason for proposal

PR1 identifies a WFM Client through a WFM-assigned `clientId`, provisions trust through a WFM-specific root CA, and authenticates with server-side TLS plus RFC 9421 HTTP Message Signatures. That model worked for PR1's scope, but it has a few structural problems that surface once the Trust Domain has a single identity model.

A PR1 client identity is local to one specific WFM and isn't portable across the Trust Domain. Each WFM also publishes its own root CA for clients to fetch, which duplicates what MIAF's Trust Bundle already does at the Trust Domain level. Keeping the PR1 onboarding endpoints alongside MIAF would force every WFM to maintain two parallel onboarding paths.

PR1's caller identity is also split across three places: a `{clientId}` URL parameter, the TLS layer, and an RFC 9421 application-layer signature. Implementations have to keep all three in sync, and the PR1 implementation experience showed this split as a recurring source of confusion — when the path asserts identity A and the credential presents identity B, it isn't always clear which one wins.

RFC 9421 itself isn't a limitation — it was chosen for PR1 specifically to support client authentication in environments with a TLS-terminating proxy in the path, where mTLS isn't feasible end-to-end. The mismatch under MIAF is that RFC 9421 carries client identity in a way that doesn't connect to the SPIFFE model. This SUP makes mTLS with the WFM Client X.509-SVID the baseline (since MIAF assumes end-to-end mTLS where feasible) and routes the proxy use case to a deferred JWT-SVID profile (see [`deferred/miaf-non-mtls-environments.md`](./deferred/miaf-non-mtls-environments.md)) that delivers the same SPIFFE identity over a bearer credential.

Finally, PR1 doesn't separate device identity from software-client identity, while MIAF makes that separation core to its model.

This SUP defines a distinct WFM Client Identity Profile built on MIAF and updates the Margo Management Interface to consume it. The device identity remains the authentication foundation for initial issuance, with a separate logical identity for the WFM Client itself, bound to one specific WFM and recognizable without MIS lookups.

## Requirements alignment acknowledgement

This SUP addresses [margo/specification#146 — *Complete WFM client onboarding strategy following the MIAF SUP finalization*](https://github.com/margo/specification/issues/146). Its acceptance criteria are met as follows:

- **Complete WFM Client onboarding strategy** — WFM Client SPIFFE ID, binding-assertion bootstrap, MIAF enrollment endpoint integration, and the resulting management-interface authentication model (§§3–7).
- **Integration with the approved device identity strategy** ([#127](https://github.com/margo/specification/issues/127)) — the bootstrap method authenticates the candidate WFM Client using a current MIAF device SVID ([§4.2](#42-actor-model)).
- **No obstruction to hands-free onboarding** — the bootstrap method registry is extensible per MIAF; FDO and similar mechanisms are tracked as deferred SUPs.
- **Gateway functionality** — the gateway WFM Client identity is a single-holder identity covered by this SUP's primitive (see [Binding Subject](#2-terminology)). Multi-node cluster topology is deferred to [`deferred/miaf-multi-holder-identities-and-cluster-topology.md`](./deferred/miaf-multi-holder-identities-and-cluster-topology.md).

## Technical proposal

### 1. Scope and Structure

This SUP has two layers, the second building on the first.

1. The **WFM Client Identity Profile** — the first normative profile for WFM Client identities under MIAF. It reuses MIAF terminology, APIs, and the Edge Compute Device Identity Profile by reference, and fills in the WFM Client-specific pieces MIAF left for follow-up work:
   - how WFM servers themselves are named, so WFM Clients can be tied to a specific WFM;
   - the SPIFFE path convention for WFM Client identities;
   - the mechanism that binds a WFM Client identity to one specific WFM;
   - how initial issuance is bootstrapped from an existing device identity;
   - lifecycle rules for the standalone topology; and
   - authentication and authorization at the WFM API.

2. The **Margo Management Interface update** — applying the new identity profile to the existing PR1 management-interface specification: redefining `POST /api/v1/onboarding` as the WFM-side binding-assertion request endpoint, removing `GET /api/v1/onboarding/certificate`, dropping `{clientId}` from URL paths, replacing RFC 9421 HTTP Message Signatures with mTLS, and migration guidance. Specified in [§7](#7-application-to-the-margo-management-interface).

### 2. Terminology

All MIAF terminology is reused by reference unless specialized here.

#### WFM Server Identity <!-- omit from toc -->

The Trust Domain-scoped identity of a WFM namespace, expressed as a SPIFFE URI and represented by an X.509-SVID. It is the normative issuer identity used to bind WFM Client identities (canonical format in [§3.1](#31-wfm-server-identity)).

#### Logical WFM Client Identity <!-- omit from toc -->

The stable, verifiable identity of a WFM Client relationship within a Trust Domain, expressed as a SPIFFE URI and represented by an X.509-SVID (canonical format in [§3.2](#32-wfm-client-identity-format)).

#### WFM Client Binding Assertion <!-- omit from toc -->

A short-lived signed assertion issued by a WFM, or by a signer authorized for that WFM by Trust Domain policy, authorizing MIS to issue or re-issue a specific Logical WFM Client Identity (claim semantics in [§4.5](#45-wfm-client-binding-assertion)).

#### WFM Identifier (`wfm-id`) <!-- omit from toc -->

A stable path-safe identifier naming the WFM namespace under which a WFM Client identity is issued.

The `wfm-id`:

- **MUST** be unique within the Trust Domain;
- **MUST** consist only of RFC 3986 unreserved characters and **MUST** be URL-safe;
- **SHOULD** be assigned by the operator deploying the WFM into the Trust Domain (rather than chosen unilaterally by the WFM vendor) so that operators can prevent namespace collisions in multi-vendor deployments; and
- **MUST** be stable for the life of the WFM Server identity it names. Rebinding a `wfm-id` to a different WFM Server identity is not defined by this profile and **MUST NOT** be performed silently; replacement requires a new `wfm-id`.

#### Client Handle (`client-handle`) <!-- omit from toc -->

A stable path-safe identifier naming one logical WFM Client relationship within the namespace of a specific WFM.

The `client-handle`:

- **MUST** be stable for repeated assertions referring to the same logical WFM Client relationship;
- **MUST** be unique within the namespace of the issuing WFM; and
- **MUST** contain only RFC 3986 unreserved characters.

#### Binding Subject <!-- omit from toc -->

The stable subject the WFM uses to decide whether repeated issuance requests refer to the same logical WFM Client relationship. For standalone deployments, it is typically the device LDI or another operator-chosen installation subject on that device. For gateway deployments under v0, it is typically an operator-assigned gateway installation identifier rather than the gateway's device LDI, so the WFM Client identity persists across gateway hardware replacement (which v0 treats as a fresh enrollment with a new LDI) without forcing re-onboarding of the sub-device fleet. Once the deferred device-replacement SUP (see [`deferred/miaf-device-replacement.md`](./deferred/miaf-device-replacement.md)) lands, the device LDI becomes equally viable for gateways — LDI rebinding will preserve the gateway's SPIFFE ID across hardware replacement — and the choice between the two becomes an operator-style preference between the centralized replacement-ticket workflow and plain re-enrollment. Multi-node cluster topology requires a multi-holder identity primitive and is deferred to [`deferred/miaf-multi-holder-identities-and-cluster-topology.md`](./deferred/miaf-multi-holder-identities-and-cluster-topology.md).

### 3. WFM Client Identity Model

This section defines what a WFM Client identity looks like on the wire and how a WFM recognizes one. The WFM Server identity comes first (since it is the issuer that ties WFM Client identities to a specific WFM), then the SPIFFE path convention for WFM Client identities themselves, then the SVID representation, and finally what a WFM has to do to recognize a valid WFM Client at request time.

#### 3.1 WFM server identity

This SUP defines the following SPIFFE path convention for the baseline WFM Server Identity:

```text
spiffe://<trust-domain>/margo/wfm/<wfm-id>
```

A WFM participating in this profile:

- **MUST** hold a valid WFM Server Identity, or use a signer explicitly authorized by Trust Domain policy for that WFM Server Identity;
- **MUST** use that identity namespace when issuing WFM Client Binding Assertions; and
- **MUST** use the same `wfm-id` in both its own SPIFFE ID and the SPIFFE IDs of the WFM Clients it binds.

The WFM Server is a **non-device principal** under MIAF and enrolls using the same APIs and lifecycle as any other principal. v0 MIAF defines only the device Factory Certificate bootstrap method; non-device bootstrap methods (in particular the [Operator-issued Enrollment Token method](./deferred/miaf-enrollment-token-bootstrap-method.md)) are deferred. Until such a method is registered, WFM Server identities are expected to be operator-pre-provisioned. **Operator pre-provisioning** means minting a WFM Server X.509-SVID directly from the Trust Domain CA materials and configuring the corresponding signer-policy entry at MIS ([§8.1](#81-trust-between-mis-and-wfm)). A delegated signer authorized in place of the WFM Server is also a non-device principal under MIAF and follows the same path.

#### 3.2 WFM Client identity format

This SUP defines the following SPIFFE path convention for WFM Client identities:

```text
spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<client-handle>
```

The `wfm-id` and `client-handle` path segments:

- **MUST** be URL-safe and consist only of RFC 3986 unreserved characters;
- **MUST NOT** be interpreted by the WFM Client as structured business data unless separately documented by the deployment; and
- **SHOULD** be treated as opaque by the WFM Client.

This identity is the canonical WFM Client identity within the Trust Domain.

#### 3.3 Identity representations

The **X.509-SVID** is the only representation for WFM Client issuance and authentication, using the existing MIAF profile URI `https://margo.org/profiles/spiffe/x509-svid/v1`. A JWT-SVID representation may be added later (see [`deferred/miaf-non-mtls-environments.md`](./deferred/miaf-non-mtls-environments.md)).

#### 3.4 Recognition by the WFM

A conformant WFM **MUST** recognize a WFM Client from the authenticated SPIFFE ID alone. Specifically, the WFM **MUST**:

1. validate the presented SVID using the Trust Bundle for the Trust Domain;
2. extract the authenticated SPIFFE ID;
3. verify that the `wfm-id` in the SPIFFE path corresponds to a `wfm-id` for which this WFM is the issuing authority; and
4. authorize the request using local policy keyed by that WFM Client identity.

The WFM **MUST NOT** require a synchronous lookup to MIS to recognize a WFM Client identity represented by a valid SVID.

### 4. Bootstrap and Enrollment

A candidate WFM Client gets its identity in two steps. It first asks the WFM for a short-lived signed assertion authorizing the issuance, then it presents that assertion to MIS to obtain its WFM Client SVID. This section specifies both halves of the flow — the WFM-side endpoint that hands out the assertion ([§4.3](#43-wfm-binding-assertion-request-endpoint)), the MIS enrollment request that consumes it ([§4.4](#44-enrollment-request)), the assertion's claim shape ([§4.5](#45-wfm-client-binding-assertion)), and how MIS validates everything ([§4.7](#47-mis-validation-rules)). Throughout, the candidate authenticates with its existing device SVID; the WFM Client SVID it ends up holding is what authenticates it to the WFM thereafter.

#### 4.1 Bootstrap method identifier

This SUP defines one WFM Client bootstrap method:

```text
urn:margo:bootstrap:wfm-client-binding:v1
```

#### 4.2 Actor model

The authenticated actor is a candidate WFM Client running on a platform that already holds a valid device identity under MIAF. The candidate WFM Client **MUST** authenticate to MIS using **mTLS** with a valid current device X.509-SVID. The WFM Client Binding Assertion is conveyed in the enrollment request body. How the candidate obtains that assertion from the WFM is specified in [§4.3](#43-wfm-binding-assertion-request-endpoint).

#### 4.3 WFM binding-assertion request endpoint

A WFM **MUST** expose:

```text
POST /api/v1/onboarding
```

for candidate WFM Clients to obtain a WFM Client Binding Assertion (claim shape in [§4.5](#45-wfm-client-binding-assertion)). This endpoint is the runtime delivery channel for the assertion; it is the WFM-side counterpart to the MIS enrollment request in [§4.4](#44-enrollment-request).

##### Authentication <!-- omit from toc -->

The candidate **MUST** authenticate to the WFM using **mTLS** with a valid current device X.509-SVID issued under MIAF's Edge Compute Device Identity Profile. The WFM **MUST** validate the device SVID against the Trust Bundle for its Trust Domain.

##### Request body <!-- omit from toc -->

JSON. All fields are optional; an empty object `{}` is valid:

```json
{
  "clientHandle": "<path-safe identifier>"
}
```

- `clientHandle` — when present, the candidate's suggested handle. The WFM **MAY** honor or override it according to local policy. When absent, the WFM **MUST** derive the handle from a stable local subject (see [§4.8](#48-stateless-wfm-candidate-issuance)).

##### Response <!-- omit from toc -->

`201 Created`, JSON:

```json
{
  "bindingAssertion": "<compact-jws>"
}
```

The returned assertion **MUST** satisfy the claim rules in [§4.5](#45-wfm-client-binding-assertion).

##### Authorization <!-- omit from toc -->

The WFM **MUST** authorize the request using local policy keyed by the authenticated device SPIFFE ID. The WFM **MAY** deny issuance of a binding assertion to a device whose SVID is valid but not authorized to enroll as a client of this WFM.

##### Errors <!-- omit from toc -->

WFMs **MUST** return error responses in RFC 9457 Problem Details format per the MIAF [Appendix B](./margo-identity-and-authorization-framework.md#appendix-b-error-responses-normative) conventions — `Content-Type: application/problem+json`, `status` field matching the HTTP status code, and `Retry-After` response header on `429`. Error conditions for this endpoint:

| Condition | HTTP Status | `type` URI | `title` |
| :-------- | :---------- | :--------- | :------ |
| Malformed body or `clientHandle` syntactically invalid | 400 | `about:blank` | Bad Request |
| Missing, expired, or untrusted device SVID | 401 | `about:blank` | Unauthorized |
| Device authenticated but not authorized by local policy | 403 | `https://margo.org/docs/errors/wfm-client-onboarding-forbidden` | Onboarding Forbidden |
| `clientHandle` semantically rejected (e.g., conflicts with an existing binding) | 422 | `https://margo.org/docs/errors/wfm-client-handle-conflict` | Client Handle Conflict |
| Rate-limited (see [§8.2](#82-replay-and-dos-controls)) | 429 | `https://margo.org/docs/errors/too-many-requests` | Too Many Requests |

##### Idempotency <!-- omit from toc -->

Repeated requests from the same authenticated device **SHOULD** yield assertions referring to the same `clientHandle` (with refreshed `iat`, `exp`, and `jti`), unless local policy decides the relationship has been retired or rebound to a different binding subject.

In standalone re-issuance after device replacement ([§5.2](#52-standalone-topology)), the candidate calling `POST /api/v1/onboarding` is on a different device than the original holder. Whether the WFM issues an assertion for the existing `clientHandle` (preserving the logical client identity) or a new `clientHandle` is determined by local WFM policy and the binding subject the operator chose at original issuance — the protocol does not constrain this decision.

#### 4.4 Enrollment request

Initial issuance and re-issuance use the generic MIAF endpoint `POST /api/v1/identities`, with `svidProfileUri` set to `https://margo.org/profiles/spiffe/x509-svid/v1` and `bootstrapCredential` set to:

```json
{
  "method": "urn:margo:bootstrap:wfm-client-binding:v1",
  "proof": {
    "bindingAssertion": "<compact-jws>"
  }
}
```

The `svidRequest` object uses the same CSR structure defined by MIAF for X.509-SVID issuance.

#### 4.5 WFM Client Binding Assertion

The WFM Client Binding Assertion is a JSON Web Token ([RFC 7519](https://datatracker.ietf.org/doc/html/rfc7519)) in compact serialization. Its JOSE header **MUST** include `typ: "application/wfm-client-binding+jwt"` and `x5c` (per [RFC 7515 §4.1.6](https://datatracker.ietf.org/doc/html/rfc7515#section-4.1.6)) carrying the signer's current X.509-SVID chain, so the assertion is self-contained. The signing key is the private key whose public counterpart is in the leaf certificate of `x5c`; MIS retrieves the verification key directly from that leaf certificate after validating the chain against the Trust Bundle. The following claims and semantics are normative.

| Claim | Requirement |
| :--- | :--- |
| `iss` | **MUST** be a SPIFFE ID. Either the WFM Server Identity `spiffe://<trust-domain>/margo/wfm/<wfm-id>` itself, or the SPIFFE ID of a delegated signer explicitly authorized by Trust Domain policy for that WFM Server Identity. |
| `sub` | **MUST** equal `iss`. |
| `aud` | **MUST** equal the MIS identity issuance endpoint URL — that is, `<margoIdentityServiceBaseUri>/api/v1/identities`, where `margoIdentityServiceBaseUri` is the value advertised in the MIAF [Discovery Document](./margo-identity-and-authorization-framework.md#discovery-document-endpoint) for this Trust Domain. Operator-chosen alternative audience values are not permitted. |
| `iat` | **MUST** be present. |
| `exp` | **MUST** be present and **MUST NOT** exceed 5 minutes after `iat`. |
| `jti` | **MUST** be unique per assertion. |
| `wfmId` | **MUST** equal the `wfm-id` encoded in the resulting SPIFFE path. |
| `clientHandle` | **MUST** equal the `client-handle` encoded in the resulting SPIFFE path. |

The signer of the binding assertion:

- **MUST** be authorized by Trust Domain policy to issue WFM Client binding assertions for the referenced `wfm-id`;
- **MUST** be bound by that policy to the corresponding WFM Server Identity `spiffe://<trust-domain>/margo/wfm/<wfm-id>`;
- **MUST** be validated by MIS without requiring a runtime call to the WFM; and
- **MUST** sign using algorithms permitted by the MIAF cryptographic requirements.

#### 4.6 Enrollment Subject Identifier derivation

For this bootstrap method, MIS **MUST** derive the Enrollment Subject Identifier as:

```text
SHA-256("wfm-client-binding:v1" || 0x00 || wfmId || 0x00 || clientHandle)
```

encoded as lowercase hexadecimal.

The ESI is intentionally independent of the current holder device, so the same logical WFM Client identity may persist across standalone rebinding when policy permits. Future SUPs (see [`deferred/miaf-multi-holder-identities-and-cluster-topology.md`](./deferred/miaf-multi-holder-identities-and-cluster-topology.md)) extend this to multi-node cluster leader failover.

#### 4.7 MIS validation rules

**Precondition:** Before processing a `urn:margo:bootstrap:wfm-client-binding:v1` enrollment request, MIS **MUST** possess validated trust material for the WFM Server Identity (or its delegated signer) named by the assertion's `iss` claim. That trust material is established by the WFM Server's MIAF enrollment or operator pre-provisioning (see [§3.1](#31-wfm-server-identity)) together with the Trust Domain signer policy that authorizes the corresponding `wfm-id` namespace (see [§8.1](#81-trust-between-mis-and-wfm)). MIS **MUST** reject a binding assertion whose `iss` is not covered by this trust material.

For `urn:margo:bootstrap:wfm-client-binding:v1`, MIS **MUST** then perform all of the following:

1. validate the caller's current device identity according to MIAF;
2. validate the WFM Client Binding Assertion by: (a) validating the `x5c` chain against the Trust Bundle; (b) verifying that the leaf certificate's SPIFFE ID matches the `iss` claim; (c) verifying the JWT signature with the leaf certificate's public key;
3. validate audience, expiry, and replay protection for the assertion;
4. verify that the signer is authorized by local policy to issue binding assertions for the asserted `wfm-id` and is bound to the corresponding WFM Server Identity namespace;
5. verify that the `wfm-id` and `clientHandle` satisfy this profile's path rules;
6. derive the ESI from `wfmId` and `clientHandle`;
7. map that ESI to the SPIFFE ID `spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<client-handle>`; and
8. issue a WFM Client X.509-SVID for that SPIFFE ID.

#### 4.8 Stateless WFM candidate issuance

A WFM **MAY** issue binding assertions for candidate clients without persisting candidate state, by deriving `clientHandle` deterministically from a stable local subject and signing the assertion statelessly. Whether the WFM persists local authorization state before or after first authenticated use is deployment-specific and does not affect interoperability.

### 5. Lifecycle Rules (Standalone)

This section walks through the lifecycle of a WFM Client identity in a standalone deployment — initial enrollment, day-to-day use, renewal, re-issuance after device replacement, and retirement.

#### 5.1 Lifecycle phases

- **Enrollment:** initial issuance of the WFM Client identity.
- **Active:** the WFM Client uses its current SVID to authenticate to the WFM.
- **Renewal:** the WFM Client refreshes its SVID using the standard MIAF renewal flow.
- **Re-issuance / rebinding:** the WFM Client repeats enrollment with a fresh binding assertion for the same `wfm-id` and `client-handle`.
- **Revocation / termination:** the identity is retired by policy or trust-domain controls.

The WFM Client Binding Assertion is the only WFM-specific authorization artifact for issuance and re-issuance; this SUP does not define a separate operator-ticket mechanism.

#### 5.2 Standalone topology

For a standalone device:

- there is exactly one WFM Client relationship for the logical client instance on that device;
- initial issuance **MUST** be requested by a holder of a valid device SVID;
- the binding subject **MAY** be the device LDI or another stable operator-chosen installation subject; and
- re-issuance **MAY** later occur on a different device if the WFM issues a fresh binding assertion for the same `clientHandle`.

#### 5.3 Renewal and disconnected operation

Renewal uses the standard MIAF renewal endpoint with the current WFM Client identity. Renewal does **not** require a fresh WFM Client Binding Assertion: possession of the current WFM Client SVID and the standard MIAF renewal flow are sufficient. This profile does **not** guarantee operation beyond the lifetime of the currently valid credential.

### 6. Authentication and Authorization at the WFM API

Once a WFM Client holds its SVID, every subsequent call to the WFM API is authenticated by mTLS with that SVID — no `clientId` in the URL, no application-layer signatures. This section pins down the authentication mechanism, says how the WFM derives the caller's identity from the credential, and describes how authorization decisions sit on top.

#### 6.1 Authentication model

WFM API authentication **MUST** use **mTLS with the WFM Client X.509-SVID**. A JWT-SVID bearer mode for proxy environments may be added later (see [`deferred/miaf-non-mtls-environments.md`](./deferred/miaf-non-mtls-environments.md)).

#### 6.2 Canonical caller identity

The authenticated WFM Client SPIFFE ID is the canonical caller identity. WFM API revisions:

- **MUST NOT** require a path parameter such as `{clientId}` as the primary caller-identity selector;
- **SHOULD** derive the logical caller identity from the authenticated SPIFFE ID; and
- **MAY** retain path parameters for business resources (deployment identifiers, bundle digests, etc.).

#### 6.3 Authorization

The WFM **MUST** authorize requests using local policy keyed by the authenticated WFM Client identity. Policy may consider the exact SPIFFE ID, the `wfm-id` namespace, deployment-specific `clientHandle` metadata, or whether the WFM currently accepts requests from that client. The WFM **MAY** deny requests from a still-valid credential based on local policy.

When a WFM denies a request from a still-valid credential under such a policy, it **SHOULD** respond with `403 Forbidden` and an RFC 9457 Problem Details body using the `wfm-client-relationship-retired` type, so that operators and clients can distinguish policy-driven denials from authentication failures:

```json
{
  "type": "https://margo.org/docs/errors/wfm-client-relationship-retired",
  "title": "Client Relationship Retired",
  "status": 403,
  "detail": "The WFM Client relationship has been retired by local policy."
}
```

| Condition | HTTP Status | `type` URI | `title` |
| :-------- | :---------- | :--------- | :------ |
| Client relationship retired by local policy | 403 | `https://margo.org/docs/errors/wfm-client-relationship-retired` | Client Relationship Retired |

### 7. Application to the Margo Management Interface

This section specifies how the WFM Client Identity Profile changes the existing PR1 Margo Management Interface for Workload Fleet Management.

#### 7.1 Endpoints removed

The following PR1 endpoint is **removed** by this SUP:

- `GET /api/v1/onboarding/certificate` — removed. Trust anchors are retrieved via the MIAF [Trust Bundle Retrieval Endpoint](./margo-identity-and-authorization-framework.md#trust-bundle-retrieval-endpoint), located through the MIAF [Discovery Document](./margo-identity-and-authorization-framework.md#discovery-document-endpoint).

#### 7.2 Endpoint redefined

`POST /api/v1/onboarding` is retained as a path but **redefined** as the WFM-side binding-assertion request endpoint. The PR1 semantics — submitting a client X.509 certificate and receiving a WFM-assigned `clientId` — are removed in their entirety. The new request shape, authentication, response, and error semantics are normatively specified in [§4.3](#43-wfm-binding-assertion-request-endpoint).

WFMs **MUST NOT** accept the PR1 onboarding request shape. PR1-shaped requests will fail authentication, since PR1 used `PayloadSignature` rather than mTLS with a device SVID, and **MUST** be rejected with `401 Unauthorized`.

The MIAF [Enrollment and Identity Issuance Endpoint](./margo-identity-and-authorization-framework.md#enrollment-and-identity-issuance-endpoint) at MIS performs identity issuance using the binding-assertion bootstrap method defined in [§4](#4-bootstrap-and-enrollment); the redefined `POST /api/v1/onboarding` at the WFM is the runtime channel that delivers a fresh binding assertion to the candidate before that MIS call.

#### 7.3 Endpoints retained, with `{clientId}` path parameter dropped

The following PR1 endpoints are retained with the same business behavior, but the `{clientId}` path parameter is **removed**. Caller identity for every retained endpoint is derived from the authenticated WFM Client SPIFFE ID per [§6](#6-authentication-and-authorization-at-the-wfm-api).

| PR1 endpoint | Updated endpoint |
| :--- | :--- |
| `POST /api/v1/clients/{clientId}/capabilities` | `POST /api/v1/capabilities` |
| `PUT /api/v1/clients/{clientId}/capabilities` | `PUT /api/v1/capabilities` |
| `GET /api/v1/clients/{clientId}/bundles/{digest}` | `GET /api/v1/bundles/{digest}` |
| `GET /api/v1/clients/{clientId}/deployments` | `GET /api/v1/deployments` |
| `GET /api/v1/clients/{clientId}/deployments/{deploymentId}/{digest}` | `GET /api/v1/deployments/{deploymentId}/{digest}` |
| `POST /api/v1/clients/{clientId}/deployments/{deploymentId}/status` | `POST /api/v1/deployments/{deploymentId}/status` |

WFMs **MUST** authorize each request using local policy keyed by the authenticated WFM Client SPIFFE ID, per [§6.3](#63-authorization).

The PR1 `PayloadSignature` security scheme (RFC 9421 HTTP Message Signatures) is **removed in its entirety**. Authentication for every retained endpoint **MUST** use mTLS using the WFM Client X.509-SVID per [§6.1](#61-authentication-model). WFMs **MUST NOT** accept requests authenticated by `PayloadSignature`.

### 8. Security and Operational Considerations

#### 8.1 Trust between MIS and WFM

MIS and WFM do not require synchronous runtime interaction. Trust is established through a shared Trust Domain, the WFM Server Identity (or a policy-bound delegated signer), Trust Domain policy authorizing the `wfm-id` namespace, and MIS validation of binding assertions using locally available trust material.

##### Trust Domain signer policy (informative) <!-- omit from toc -->

The "Trust Domain policy authorizing the WFM namespace" referenced above is, in practice, a Trust-Domain-level mapping from each `wfm-id` to one or more permitted signer identities authorized to issue WFM Client Binding Assertions for that namespace. A signer identity is a SPIFFE ID, expressed either as:

- the WFM Server's SPIFFE ID itself (the most common case); or
- a delegated signer's SPIFFE ID.

The wire format and storage of this policy are deployment-specific. Typical implementations provision the policy at MIS via configuration files, an authenticated administrative interface, or deployment tooling. On every binding-assertion validation, MIS resolves the assertion's `iss` claim against this policy and rejects assertions whose `iss` is not authorized for the asserted `wfm-id`.

> **Illustrative example (informative, not a wire format):**
>
> ```yaml
> wfm_signer_policy:
>   - wfm_id: factory-blue-line-1
>     authorized_signers:
>       - spiffe://factory.example/margo/wfm/factory-blue-line-1
>   - wfm_id: vendor-x-managed-fleet-7
>     authorized_signers:
>       - spiffe://factory.example/margo/wfm/vendor-x-managed-fleet-7
>       - spiffe://factory.example/svc/vendor-x-binding-signer
> ```
>
> Two `wfm-id` namespaces are shown. The first authorizes only the WFM Server itself as the binding-assertion signer. The second additionally authorizes a delegated signer SPIFFE ID, supporting deployments where the WFM offloads assertion signing to a dedicated component.

#### 8.2 Replay and DoS controls

- MIS **MUST** reject replayed binding assertions using the `jti` claim. The replay cache **MUST** be effective across all MIS instances that share an issuance authority within the Trust Domain — single-instance MIS deployments **MAY** keep it process-local; HA deployments **MUST** make it cluster-wide. The cache **MUST** retain `jti` values at least until each assertion's `exp` has passed.
- MIS **MUST** rate-limit WFM Client enrollment and renewal requests.
- WFMs issuing assertions for candidate clients **SHOULD** keep assertion TTLs short to limit abuse and replay windows.

### 9. Typical Workflows (Informative)

The numbered steps below walk through the standalone enrollment flow end-to-end.

#### 9.1 Standalone device

1. The device enrolls under the Edge Compute Device Identity Profile and obtains a device X.509-SVID.
2. The candidate WFM Client calls `POST /api/v1/onboarding` on the WFM, authenticated by that device X.509-SVID.
3. The WFM mints a WFM Client Binding Assertion for `wfm-id = X` and `client-handle = Y` and returns it to the candidate.
4. The candidate WFM Client calls `POST /api/v1/identities` at MIS, authenticated by the same device X.509-SVID, with the binding assertion in the enrollment request body.
5. MIS validates the assertion and issues `spiffe://<trust-domain>/margo/wfm/X/client/Y`.
6. The WFM Client authenticates to the WFM using that SVID.

## Alternatives considered (optional)

### Reusing the device identity directly

Rejected because it collapses two principals with different lifecycles, authorization scopes, and topology behavior into one.

### Treating the WFM Client as only a generic workload identity

A design where the WFM Client holds a generic workload SPIFFE ID (e.g., `spiffe://<trust-domain>/margo/wfm-client/<id>`) without encoding the target WFM in the path was rejected. Encoding `wfm-id` in the SPIFFE path provides three benefits:

- **Defense-in-depth at the credential layer.** A credential issued under `spiffe://<trust-domain>/margo/wfm/A/client/...` cannot impersonate a client of WFM B even if WFM B's local policy is corrupted or stale. A generic workload identity defers all WFM-binding enforcement to local policy and silently fails open if that policy is wrong.
- **MIS-level multi-tenancy.** MIS structurally enforces "this binding assertion comes from the signer authorized for this `wfm-id`" without per-client policy state. A generic workload identity would require MIS per-client policy or push binding decisions outside MIAF.
- **Audit clarity.** The SPIFFE ID alone names both the WFM namespace and the client relationship; logs and authorization decisions are unambiguous without external joins.

### Keeping RFC 9421 HTTP Message Signatures

Keeping RFC 9421 alongside or in addition to mTLS would preserve the PR1 split identity model (`clientId`, path parameters, WFM-local certificate handling) instead of converging on MIAF. The credential presented at TLS is the same X.509-SVID that would also key the RFC 9421 signing, so per-request payload signing adds implementation cost without incremental security gain in deployments where mTLS is end-to-end. Where TLS is terminated at a proxy, the JWT-SVID profile (deferred to [`deferred/miaf-non-mtls-environments.md`](./deferred/miaf-non-mtls-environments.md)) is the mechanism that replaces RFC 9421.

### Requiring MIS-to-WFM validation calls

Requiring MIS-to-WFM validation calls would increase coupling and create a new availability dependency. Offline validation of a short-lived signed binding assertion is sufficient.

## Rejection reason

Not applicable.
