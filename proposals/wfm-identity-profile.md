# Margo WFM Identity Profile and Management Interface Update

- [Margo WFM Identity Profile and Management Interface Update](#margo-wfm-identity-profile-and-management-interface-update)
  - [Owner](#owner)
  - [Summary](#summary)
  - [Reason for proposal](#reason-for-proposal)
  - [Requirements alignment acknowledgement](#requirements-alignment-acknowledgement)
  - [Technical proposal](#technical-proposal)
    - [1. Scope and Structure](#1-scope-and-structure)
    - [2. Terminology](#2-terminology)
    - [3. Identity Model](#3-identity-model)
      - [WFM identity](#wfm-identity)
      - [WFM Client identity](#wfm-client-identity)
      - [Identity representation](#identity-representation)
      - [Recognition by the WFM](#recognition-by-the-wfm)
      - [Recognition by the WFM Client](#recognition-by-the-wfm-client)
    - [4. Provisioning (PR2)](#4-provisioning-pr2)
      - [WFM provisioning](#wfm-provisioning)
      - [WFM Client provisioning](#wfm-client-provisioning)
    - [5. Lifecycle and Operator Playbook](#5-lifecycle-and-operator-playbook)
    - [6. Authentication and Authorization at the WFM API](#6-authentication-and-authorization-at-the-wfm-api)
      - [Authentication model](#authentication-model)
      - [Canonical caller identity](#canonical-caller-identity)
      - [Authorization](#authorization)
      - [Scope: traffic-inspecting proxies](#scope-traffic-inspecting-proxies)
    - [7. Application to the Margo Management Interface](#7-application-to-the-margo-management-interface)
      - [Endpoints removed](#endpoints-removed)
      - [Security scheme removed](#security-scheme-removed)
      - [Endpoints retained, with `{clientId}` dropped](#endpoints-retained-with-clientid-dropped)
    - [8. Security Considerations](#8-security-considerations)
    - [9. Roadmap (Informative)](#9-roadmap-informative)
  - [Alternatives considered](#alternatives-considered)
    - [Treating the WFM Client as a generic workload identity](#treating-the-wfm-client-as-a-generic-workload-identity)
    - [Keeping RFC 9421 HTTP Message Signatures](#keeping-rfc-9421-http-message-signatures)
    - [Web PKI for the WFM TLS certificate](#web-pki-for-the-wfm-tls-certificate)
  - [Rejection reason](#rejection-reason)

## Owner

[@matlec](https://github.com/matlec)

## Summary

This SUP defines the initial **Margo WFM Identity Profile** (building on the **[Margo Identity and Authorization Framework (MIAF)](./margo-identity-and-authorization-framework.md)**) — covering both the **WFM identity** and the **WFM Client identity** — and updates the **Margo Management Interface for Workload Fleet Management** to consume MIAF-aligned identities. A WFM gets a Trust-Domain-scoped identity that anchors its namespace; a WFM Client gets a Trust-Domain-scoped identity named under the WFM that issues it.

The change replaces three PR1 mechanisms:

- the WFM-assigned `clientId` carried in URL paths;
- the per-WFM onboarding and certificate-distribution endpoints (`POST /api/v1/onboarding` and `GET /api/v1/onboarding/certificate`); and
- the RFC 9421 HTTP Message Signatures security scheme used for client authentication.

Authentication is mutual: a WFM Client presents its X.509-SVID and validates the WFM's SVID; caller identity at the WFM API is the authenticated WFM Client SPIFFE ID carried via mTLS.

PR2 ships the identity model and the mTLS authentication surface. Automated enrollment — for both WFMs and WFM Clients — is deferred to PR3; see [§1](#1-scope-and-structure) for how PR2 implementations operate in the meantime, and the [Roadmap](#9-roadmap-informative) for what comes later.

## Reason for proposal

PR1 onboarding has each Client present its own X.509 certificate to the WFM, which pins that certificate (typically by fingerprint) and assigns a `clientId`. The WFM exposes its own TLS server-side trust anchor at `GET /api/v1/onboarding/certificate` for Clients to fetch and trust. Subsequent Client requests are authenticated by server-side TLS plus RFC 9421 HTTP Message Signatures keyed to the Client's certificate. That worked for PR1's scope, but it runs into structural problems under MIAF's Trust-Domain-scoped identity model.

PR1's mechanisms are entirely per-WFM: the `clientId` namespace, the TLS trust anchor, and the onboarding flow all live at the WFM rather than at the Trust Domain. WFM Client identities are not portable across the Trust Domain; the WFM's per-endpoint trust anchor duplicates what MIAF's Trust Bundle does Trust-Domain-wide; and the onboarding flow exists only to bootstrap Clients into that per-WFM trust regime, with no equivalent need under MIAF.

The WFM itself is also affected: PR1 gives it no Trust-Domain-scoped identity. This SUP names the WFM with a Trust-Domain-scoped SPIFFE ID and recognizes it as a first-class principal under MIAF — putting WFMs and WFM Clients on the same identity footing under one Trust Bundle.

Caller identity is also split across three places: a `{clientId}` URL parameter (the asserted identity), an RFC 9421 signature with the Client's certificate (the cryptographic proof), and the WFM's locally-stored pinning between cert fingerprint and `clientId` (the binding that reconciles the two). Implementations have to keep all three in sync, and in practice this split has been a recurring source of confusion — when the path asserts identity A and the credential presents identity B, it isn't always clear which one wins.

This SUP folds those three into one: a distinct Margo WFM Identity Profile under MIAF, with caller identity drawn from the authenticated SPIFFE ID.

## Requirements alignment acknowledgement

This SUP partially addresses [margo/specification#146 — *Complete WFM client onboarding strategy following the MIAF SUP finalization*](https://github.com/margo/specification/issues/146):

- **WFM identity** — Trust-Domain-scoped WFM SPIFFE ID convention; WFM Client recognition of the WFM (§§3, 6).
- **WFM Client identity and recognition** — WFM Client SPIFFE ID convention and recognition by the WFM (§§3, 6).
- **Management-interface authentication** — mTLS with WFM Client X.509-SVID; `{clientId}` removed; PR1 onboarding endpoints and `PayloadSignature` removed (§§6, 7).
- **Complete onboarding strategy** — *deferred to PR3*. PR2 ships the identity model and authentication model; PR3 picks the automated enrollment protocol. Operator-pre-provisioning is the PR2 baseline and persists into PR3.
- **Integration with the approved device identity strategy** ([#127](https://github.com/margo/specification/issues/127)) — *deferred*, alongside the device identity profile (see the MIAF [Roadmap](./margo-identity-and-authorization-framework.md#8-roadmap-and-forward-extensibility-informative)).
- **Gateway functionality** — a gateway acting as the WFM Client for the devices it manages is covered by this SUP.

## Technical proposal

### 1. Scope and Structure

This SUP has two layers:

1. The **Margo WFM Identity Profile** — the first normative profile for WFM identities under MIAF, covering both the **WFM** and the **WFM Client**. It fills in the WFM-specific pieces MIAF left open: how WFMs are named and recognized, the SPIFFE path for WFM Client identities, the lifecycle applied to both, and authentication and authorization at the WFM API.
2. The **Margo Management Interface update** — applying the new identity profile to the PR1 management interface: removing the onboarding and certificate-distribution endpoints, dropping `{clientId}` from URL paths, and replacing RFC 9421 HTTP Message Signatures with mTLS. Specified in [§7](#7-application-to-the-margo-management-interface).

In PR2, both WFM and WFM Client SVIDs are **operator-pre-provisioned**: the operator mints SVIDs with the right SPIFFE paths and installs them on each principal through its existing provisioning channel; each WFM is configured with the set of WFM Client identities it accepts, and each WFM Client is configured with the WFM identity it expects to connect to. PR3 will add automated enrollment on top of this foundation, and operator-pre-provisioning remains a valid path in PR3 for deployments that prefer it.

### 2. Terminology

All MIAF terminology is reused by reference unless specialized here.

#### WFM Identity (`wfm-id`) <!-- omit from toc -->

The Trust Domain-scoped identity of a WFM, expressed as a SPIFFE URI of the form `spiffe://<trust-domain>/margo/wfm/<wfm-id>` and represented by an X.509-SVID (canonical format and recognition rules in [§3](#3-identity-model)). It anchors the namespace under which WFM Client identities are issued. The `wfm-id` segment:

- **MUST** be unique within the Trust Domain;
- **MUST** consist only of [RFC 3986](https://datatracker.ietf.org/doc/html/rfc3986) unreserved characters;
- **SHOULD** be assigned by the operator deploying the WFM into the Trust Domain (not unilaterally by the WFM vendor), so operators can prevent namespace collisions in multi-vendor deployments; and
- **MUST** be stable for the life of the WFM identity it names. Rebinding a `wfm-id` to a different WFM identity is not defined by this profile and **MUST NOT** be performed silently; replacement requires a new `wfm-id`.

Operators **MAY** assign one shared `wfm-id` across multiple WFM instances to present a single logical identity, or distinct `wfm-id`s per instance for finer-grained per-instance [lifecycle management](#5-lifecycle-and-operator-playbook).

#### WFM Client Identity (`wfm-client-id`) <!-- omit from toc -->

The stable, verifiable identity of a WFM Client relationship within a Trust Domain, expressed as a SPIFFE URI of the form `spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>` and represented by an X.509-SVID (canonical format and recognition rules in [§3](#3-identity-model)). The `wfm-client-id` segment:

- **MUST** be stable for the lifetime of the relationship;
- **MUST** be unique within the issuing WFM's namespace; and
- **MUST** consist only of [RFC 3986](https://datatracker.ietf.org/doc/html/rfc3986) unreserved characters.

### 3. Identity Model

This section says what WFM and WFM Client identities look like on the wire and how each is recognized. The WFM identity comes first (it's the namespace that ties WFM Client identities to a specific WFM), then the WFM Client identity, then their representation, then recognition at request time — from each direction.

#### WFM identity

A WFM identity is a SPIFFE ID of the form:

```text
spiffe://<trust-domain>/margo/wfm/<wfm-id>
```

A WFM participating in this profile:

- **MUST** hold a valid WFM X.509-SVID; and
- **MUST** use the same `wfm-id` in its own SPIFFE ID and in the SPIFFE IDs of the WFM Clients it accepts.

The WFM is a principal under MIAF. In PR2, it obtains its SVID through the operator's provisioning channel (see [§4](#4-provisioning-pr2)).

#### WFM Client identity

A WFM Client identity is a SPIFFE ID of the form:

```text
spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>
```

The `wfm-id` and `wfm-client-id` segments:

- **MUST** consist only of [RFC 3986](https://datatracker.ietf.org/doc/html/rfc3986) unreserved characters; and
- **MUST** be treated as opaque by the WFM Client (no parsing or structural assumptions) unless the deployment separately documents otherwise.

This SPIFFE ID is the canonical WFM Client identity within the Trust Domain.

#### Identity representation

In PR2, **X.509-SVID** is the only representation for WFM Client authentication. A JWT-SVID representation is deferred — whether MIAF adopts JWT-SVIDs at all is a PR3 decision (see the MIAF [Roadmap](./margo-identity-and-authorization-framework.md#8-roadmap-and-forward-extensibility-informative)).

#### Recognition by the WFM

A WFM **MUST** recognize a WFM Client from the authenticated SPIFFE ID alone. For each request, the WFM **MUST**:

1. validate the presented SVID against the Trust Domain's Trust Bundle;
2. extract the SPIFFE ID from the URI SAN;
3. verify that the `wfm-id` in the path belongs to this WFM's namespace; and
4. authorize the request using local policy keyed on that WFM Client identity.

#### Recognition by the WFM Client

A WFM Client **MUST** recognize the WFM it connects to from the authenticated SPIFFE ID alone. For each connection, the WFM Client **MUST**:

1. validate the presented WFM SVID against the Trust Domain's Trust Bundle;
2. extract the SPIFFE ID from the URI SAN;
3. verify that the SPIFFE ID has the form `spiffe://<trust-domain>/margo/wfm/<wfm-id>` and that `<wfm-id>` matches the operator-configured target WFM identifier for this Client; and
4. abort the connection if any of these checks fails.

Step 3 mirrors the WFM-side `wfm-id` namespace check. Without it, a WFM Client could authenticate upstream to a different WFM than its operator intended, even though the WFM SVID is validly issued by the Trust Domain.

### 4. Provisioning (PR2)

WFM and WFM Client SVIDs are both **operator-pre-provisioned** in PR2. The MIAF [Operator Provisioning Playbook](./margo-identity-and-authorization-framework.md#operator-provisioning-playbook-pr2) applies; the SPIFFE-path and acceptance-policy specifics for each principal type are below.

#### WFM provisioning

For each WFM, the operator:

1. chooses a `wfm-id` for the WFM namespace;
2. mints an X.509-SVID with URI SAN `spiffe://<trust-domain>/margo/wfm/<wfm-id>`, conforming to MIAF's [X.509 SVID Profile](./margo-identity-and-authorization-framework.md#x509-svid-profile) and [Cryptographic Requirements](./margo-identity-and-authorization-framework.md#cryptographic-requirements);
3. installs the SVID (and private key material if generated centrally) on the WFM; and
4. configures every WFM Client that will connect to this WFM with the `wfm-id` as the operator-configured target identifier per [Recognition by the WFM Client](#recognition-by-the-wfm-client).

#### WFM Client provisioning

For each WFM Client, the operator:

1. chooses a `wfm-id` (for the target WFM, matching the WFM's `wfm-id`) and a `wfm-client-id` (for this client relationship);
2. mints an X.509-SVID with URI SAN `spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>`, conforming to MIAF's [X.509 SVID Profile](./margo-identity-and-authorization-framework.md#x509-svid-profile) and [Cryptographic Requirements](./margo-identity-and-authorization-framework.md#cryptographic-requirements);
3. installs the SVID (and private key material if generated centrally) on the principal; and
4. adds the new `wfm-client-id` — or full SPIFFE ID — to the target WFM's accepted-client policy, so the WFM will authorize requests from this client per [Authorization](#authorization).

PR3 will introduce automated enrollment for both principals. Operator-pre-provisioning as specified here remains valid in PR3 for deployments that prefer it. See the [Roadmap](#9-roadmap-informative).

### 5. Lifecycle and Operator Playbook

The MIAF [Lifecycle Vocabulary](./margo-identity-and-authorization-framework.md#lifecycle-vocabulary) applies to both WFM and WFM Client identities. Only the **Active** phase has a fully normative protocol surface in PR2 — Clients authenticate to WFMs via mTLS using their X.509-SVIDs per [§6](#6-authentication-and-authorization-at-the-wfm-api), and validate the WFM SVID per [Recognition by the WFM Client](#recognition-by-the-wfm-client). The other phases are operator-driven:

| Phase | WFM | WFM Client |
| :---- | :--------- | :--------- |
| Enrollment | Mint SVID with URI SAN `spiffe://<trust-domain>/margo/wfm/<wfm-id>`; install on the WFM. | Mint SVID with URI SAN `spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>`; install on principal; add `wfm-client-id` to the WFM's accepted-client policy. |
| Renewal | Mint replacement SVID (same SPIFFE ID) before expiry; install on the WFM. | Mint replacement SVID (same SPIFFE ID) before expiry; install on principal. |
| Revocation | Rotate the Trust Bundle to invalidate the issuing CA (also invalidates the WFM Clients issued under that CA). See the MIAF [Operator Revocation Playbook](./margo-identity-and-authorization-framework.md#operator-revocation-playbook-pr2). | Remove `wfm-client-id` from the WFM's accepted-client policy. For mass revocation, rotate the Trust Bundle. |
| Re-issuance | Mint a new SVID with the same SPIFFE ID; install on the replacement WFM. | Mint a new SVID (same or new `wfm-client-id`, per operator policy); install on the replacement principal; update the WFM's accepted-client policy if the identifier changed. |

WFM "revocation" is heavier-handed than WFM Client revocation because there's no Client-side accepted-server allowlist comparable to the WFM's accepted-client policy. Operators reissue the WFM SVID (keeping the same SPIFFE ID) in most cases; Trust Bundle rotation is the cryptographically-enforced revocation path.

### 6. Authentication and Authorization at the WFM API

Once a WFM Client holds its SVID, every call to the WFM API is authenticated by mTLS with that SVID — no `clientId` in the URL, no application-layer signatures.

#### Authentication model

WFM API authentication **MUST** use **mTLS**, with both sides presenting an X.509-SVID. The WFM Client **MUST** validate the WFM SVID per [Recognition by the WFM Client](#recognition-by-the-wfm-client). The WFM **MUST** validate the presented WFM Client SVID against the Trust Bundle for the Trust Domain per the MIAF [X.509 SVID Profile](./margo-identity-and-authorization-framework.md#x509-svid-profile) and [Certificate Validation](./margo-identity-and-authorization-framework.md#certificate-validation) rules.

The remainder of this section specifies the WFM-side handling of the authenticated WFM Client. A complementary mechanism for environments where end-to-end mTLS is operationally infeasible is on the PR3 [Roadmap](#9-roadmap-informative).

#### Canonical caller identity

The authenticated WFM Client SPIFFE ID is the canonical caller identity. WFM API revisions:

- **MUST NOT** require a path parameter such as `{clientId}` as the primary caller-identity selector;
- **SHOULD** derive the logical caller identity from the authenticated SPIFFE ID; and
- **MAY** retain path parameters for business resources (deployment identifiers, bundle digests, etc.).

#### Authorization

The WFM **MUST** authorize each request using local policy keyed on the authenticated WFM Client identity. Policy may consider the SPIFFE ID, the `wfm-id` namespace, deployment-specific `wfm-client-id` metadata, or whether the WFM currently accepts requests from that client. The WFM **MAY** deny a request from a still-valid credential.

When the WFM denies a still-valid credential, it **SHOULD** respond with `403 Forbidden` and an [RFC 9457](https://datatracker.ietf.org/doc/html/rfc9457) Problem Details body using the `wfm-client-relationship-retired` type, so operators and clients can tell policy-driven denials apart from authentication failures:

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

#### Scope: traffic-inspecting proxies

The MIAF [Scope: traffic-inspecting proxies](./margo-identity-and-authorization-framework.md#scope-traffic-inspecting-proxies) statement applies to the WFM API. In short: TLS-offload topologies are supported (the proxy validates the client SVID and forwards the identity to the backend, e.g., via the [RFC 9440](https://datatracker.ietf.org/doc/html/rfc9440) `Client-Cert` header). Traffic-inspecting proxies in the inline path are not supported — operators **MUST** exempt Margo mTLS endpoints from inspection. PR3 will address deployments where exemption is operationally infeasible.

### 7. Application to the Margo Management Interface

This is a **clean cut** from the PR1 management interface — PR1-shaped requests are rejected, not deprecated.

#### Endpoints removed

These PR1 endpoints are removed entirely:

- `POST /api/v1/onboarding` — removed. PR2 has no automated WFM Client onboarding protocol; SVIDs are operator-pre-provisioned per [§4](#4-provisioning-pr2).
- `GET /api/v1/onboarding/certificate` — removed. Trust anchors come from the MIAF [Trust Bundle Retrieval Endpoint](./margo-identity-and-authorization-framework.md#trust-bundle-retrieval-endpoint).

WFMs **MUST** reject requests to these endpoints with `404 Not Found`.

#### Security scheme removed

The PR1 `PayloadSignature` security scheme ([RFC 9421](https://datatracker.ietf.org/doc/html/rfc9421) HTTP Message Signatures) is removed. Authentication for every retained endpoint **MUST** use mTLS with the WFM Client X.509-SVID per [Authentication model](#authentication-model). WFMs **MUST** reject requests authenticated via `PayloadSignature` with `401 Unauthorized`.

#### Endpoints retained, with `{clientId}` dropped

The following PR1 endpoints keep their business behavior; the `{clientId}` path parameter is removed. Caller identity comes from the authenticated WFM Client SPIFFE ID per [§6](#6-authentication-and-authorization-at-the-wfm-api).

| PR1 endpoint | Updated endpoint |
| :--- | :--- |
| `POST /api/v1/clients/{clientId}/capabilities` | `POST /api/v1/capabilities` |
| `PUT /api/v1/clients/{clientId}/capabilities` | `PUT /api/v1/capabilities` |
| `GET /api/v1/clients/{clientId}/bundles/{digest}` | `GET /api/v1/bundles/{digest}` |
| `GET /api/v1/clients/{clientId}/deployments` | `GET /api/v1/deployments` |
| `GET /api/v1/clients/{clientId}/deployments/{deploymentId}/{digest}` | `GET /api/v1/deployments/{deploymentId}/{digest}` |
| `POST /api/v1/clients/{clientId}/deployments/{deploymentId}/status` | `POST /api/v1/deployments/{deploymentId}/status` |

WFMs **MUST** authorize each request via local policy keyed on the authenticated WFM Client SPIFFE ID (per [Authorization](#authorization)) and **MUST** reject requests on the PR1-shaped URLs (those that include `{clientId}`) with `404 Not Found`.

### 8. Security Considerations

WFM and WFM Client identities inherit MIAF's threats and mitigations (see the MIAF [Security Considerations](./margo-identity-and-authorization-framework.md#7-security-considerations)). The threats below are specific to this profile and to the PR1 management-interface migration.

| Threat | Description | Mitigation |
| :--- | :--- | :--- |
| **Wrong-WFM impersonation** | A WFM Client connects to a WFM whose SVID is validly issued by the Trust Domain but whose `wfm-id` does not match the Client's intended target — the Client authenticates upstream to the wrong WFM. | The WFM Client **MUST** verify the `wfm-id` in the WFM's SPIFFE path against its operator-configured target before authenticating — see [Recognition by the WFM Client](#recognition-by-the-wfm-client). |
| **Wrong-WFM Client acceptance** | A WFM accepts an SVID whose `wfm-id` belongs to a different WFM's namespace, treating the bearer as a client of itself. | The WFM **MUST** verify the `wfm-id` in the SPIFFE path against its own namespace before authorizing — see [Recognition by the WFM](#recognition-by-the-wfm). |
| **Stale accepted-client policy** | An entry remains in a WFM's accepted-client list after the relationship is retired, allowing a still-valid credential to keep accessing the API. | Operators remove `wfm-client-id` entries promptly per the Revocation row of [§5](#5-lifecycle-and-operator-playbook). The WFM **MAY** also deny the request with the `wfm-client-relationship-retired` Problem Details type per [Authorization](#authorization). |
| **PR1 protocol leak** | A WFM accepts PR1-shaped requests (with `{clientId}` paths or `PayloadSignature`), bypassing the MIAF authentication model. | WFMs **MUST** reject PR1-shaped requests with `404 Not Found` (for `{clientId}` paths and removed onboarding endpoints) or `401 Unauthorized` (for `PayloadSignature`) per [§7](#7-application-to-the-margo-management-interface). |

### 9. Roadmap (Informative)

PR2 is written to stay forward-compatible with however a later release resolves each of the following. Most items below are scoped to PR3; the device-identity-related items may land later. See also the [MIAF roadmap](./margo-identity-and-authorization-framework.md#8-roadmap-and-forward-extensibility-informative).

- **Automated WFM and WFM Client enrollment.** The protocol(s) by which a WFM obtains its initial SVID and by which a WFM Client obtains its initial SVID without manual operator installation. Candidate mechanisms are surveyed in the MIAF [Roadmap](./margo-identity-and-authorization-framework.md#8-roadmap-and-forward-extensibility-informative) and in [`miaf-pr3-inputs/`](./miaf-pr3-inputs/). Operator-pre-provisioning (the PR2 baseline) remains a valid path in PR3 for both.
- **Relationship to device identity.** Whether device identity is a foundation for WFM Client identity or a peer profile (one of several bootstrap mechanisms). This decision is gated on a device identity profile being specified, which may happen in PR3 or in a later release alongside DFM work. PR2 stays silent on this relationship in either case.
- **JWT-SVID representation for WFM Clients.** Tied to the MIAF JWT-SVID decision in PR3.
- **Authentication for traffic-inspecting-proxy environments.** Where exemption from inspection is operationally infeasible. Candidates: an HTTP message-signature profile keyed to the X.509-SVID, or a JWT-SVID bearer credential.

## Alternatives considered

### Treating the WFM Client as a generic workload identity

A design where the WFM Client holds a generic workload SPIFFE ID (e.g., `spiffe://<trust-domain>/margo/wfm-client/<id>`) without encoding the target WFM was rejected. Encoding `wfm-id` in the SPIFFE path buys three things:

- **Defense in depth.** A credential issued under `spiffe://<td>/margo/wfm/A/client/...` cannot impersonate a client of WFM B even if WFM B's local policy is corrupted or stale. A generic workload identity defers all WFM-binding enforcement to local policy and silently fails open if that policy is wrong.
- **Operator clarity.** The operator-pre-provisioning workflow is easier to audit: which WFM a client belongs to is readable directly from its SPIFFE ID, with no out-of-band metadata.
- **Log clarity.** The SPIFFE ID names both the WFM namespace and the client relationship; logs and authorization decisions are unambiguous without external joins.

### Keeping RFC 9421 HTTP Message Signatures

Keeping RFC 9421 HTTP Message Signatures alongside mTLS in PR2 was considered and rejected for this profile's scope.

PR1 used RFC 9421 because TLS to the WFM was server-authenticated only — there was no client authentication at the TLS layer, so the client's identity had to come from somewhere else, and a signed payload at the application layer was a reasonable answer. The signature carried the client's identity end-to-end, including across any TLS-terminating element in the path. That solved a real problem.

**PR2 handles the same need at the TLS layer.** mTLS authenticates the WFM Client to the WFM directly — including in topologies where a TLS-offloading reverse proxy terminates the mTLS connection and forwards the validated client identity to the backend, with the proxy-to-backend boundary trusted by the operator (network segmentation or proxy-backend mTLS). The MIAF [Scope: traffic-inspecting proxies](./margo-identity-and-authorization-framework.md#scope-traffic-inspecting-proxies) statement covers this topology. Adding RFC 9421 on top of mTLS would require WFM Clients and WFMs to sign and verify every request without authenticating anything mTLS isn't already authenticating.

**The case PR2 leaves open: traffic-inspecting proxies.** Where a proxy MITMs TLS for content inspection, end-to-end mTLS is architecturally impossible. PR2 requires operators to exempt Margo mTLS endpoints from inspection. Deployments where that exemption is operationally infeasible are not abandoned: PR3 **will** specify a complementary authentication mechanism for them. Two candidates are on the table — an HTTP message-signature profile keyed to the X.509-SVID (effectively a re-introduction of RFC 9421, now keyed to the SVID rather than a WFM-local certificate) and a JWT-SVID bearer credential. The choice between them is deferred; the commitment to address the case is not.

### Web PKI for the WFM TLS certificate

A variant where the WFM's TLS cert is anchored in a trust root other than the Trust Domain's Trust Bundle — most commonly publicly-trusted web-PKI — was considered. This is the conventional pattern for service APIs in industry, and it would let a SaaS WFM serve multiple unrelated Trust Domains from a single TLS endpoint without per-Trust-Domain dispatch.

Rejected on the following grounds.

**It would forfeit a cryptographic property symmetric SPIFFE provides.** Under MIAF as specified, only the Trust Domain's issuing authority (its MIS) can mint a valid X.509-SVID with a `wfm-id` matching the operator's namespace. The wrong-WFM impersonation defense — formalized in [§8](#8-security-considerations)'s Wrong-WFM impersonation threat — is therefore rooted in a single, operator-controlled CA. Under web-PKI, that defense distributes across the public CA ecosystem; any publicly-trusted root CA can issue a domain-validated cert for a hostname, and CT logs catch abuse after the fact rather than preventing it. The operator loses cryptographic control over which entities are authorized to vouch for the WFM endpoints their clients connect to. This trade-off is acceptable for the public web — hostnames are inherently claims about network location, not about membership in an operator-defined trust domain — but it is not acceptable for a control-plane endpoint whose identity is meant to be governed by the operator's Trust Domain.

**It would introduce per-WFM trust-profile metadata as a new attack surface.** An asymmetric model requires each WFM Client to know, per WFM endpoint, which anchor to validate the server cert against. This metadata has to be delivered out of band — typically as part of per-client provisioning — and subverting that channel lets an attacker redirect a client to a different WFM endpoint that has a valid public cert; the client's SVID will authenticate upstream to the wrong WFM. Symmetric MIAF removes this weak link by construction; the cryptographic binding above is enforced regardless of any per-client metadata.

**It would force a spec bifurcation.** Conformant WFM Client implementations would have to handle two server-cert validation paths — Trust-Bundle SVID validation and standard TLS server-cert validation with per-endpoint trust profiles — and reviewers and operators would have to reason about two trust regimes interacting per endpoint. Symmetric MIAF has one.

This decision is deliberately revisitable. Concrete evidence that would prompt reconsideration:

1. **Operational infeasibility at scale.** Multiple independent WFM vendors demonstrate that the per-Trust-Domain SVID model cannot scale to their deployment shape, even with standard SaaS dispatch patterns, and the spec-bifurcation cost is judged smaller than the alternative.
2. **Threat-model invalidation.** PR2-based deployments accumulate evidence that the cryptographic-binding defense is not materially mitigating threats anyone faces in practice.
3. **A cleaner generalization.** A future MIAF revision subsumes the asymmetric case under a single model that preserves auditability of the operator's deliberate choice without bifurcating the spec.

## Rejection reason

Not applicable.
