# Margo Identity and Authorization Framework (MIAF)

- [Margo Identity and Authorization Framework (MIAF)](#margo-identity-and-authorization-framework-miaf)
  - [Owner](#owner)
  - [Summary](#summary)
  - [Reason for proposal](#reason-for-proposal)
  - [Requirements alignment acknowledgement](#requirements-alignment-acknowledgement)
  - [Technical proposal](#technical-proposal)
    - [1. Scope and Structure](#1-scope-and-structure)
    - [2. Terminology](#2-terminology)
    - [3. The Margo Identity and Authorization Framework (MIAF)](#3-the-margo-identity-and-authorization-framework-miaf)
      - [Framework overview](#framework-overview)
      - [Identity model](#identity-model)
      - [X.509 SVID profile](#x509-svid-profile)
      - [Trust Bundles and distribution](#trust-bundles-and-distribution)
      - [Cryptographic requirements](#cryptographic-requirements)
      - [The MIS role](#the-mis-role)
    - [4. Discovery and Trust Bundle Endpoints](#4-discovery-and-trust-bundle-endpoints)
      - [Discovery document endpoint](#discovery-document-endpoint)
      - [Trust Bundle retrieval endpoint](#trust-bundle-retrieval-endpoint)
    - [5. Transport Layer Security (TLS) Requirements](#5-transport-layer-security-tls-requirements)
      - [Initial trust bootstrap](#initial-trust-bootstrap)
      - [Minimum TLS baseline](#minimum-tls-baseline)
      - [Certificate validation](#certificate-validation)
      - [Session lifetime and re-validation](#session-lifetime-and-re-validation)
      - [Scope: traffic-inspecting proxies](#scope-traffic-inspecting-proxies)
    - [6. Lifecycle Vocabulary and Operator Playbooks](#6-lifecycle-vocabulary-and-operator-playbooks)
      - [Lifecycle Vocabulary](#lifecycle-vocabulary)
      - [Operator provisioning playbook (PR2)](#operator-provisioning-playbook-pr2)
      - [Operator revocation playbook (PR2)](#operator-revocation-playbook-pr2)
      - [SVID lifetime guidance (PR2)](#svid-lifetime-guidance-pr2)
    - [7. Security Considerations](#7-security-considerations)
    - [8. Roadmap and Forward Extensibility (Informative)](#8-roadmap-and-forward-extensibility-informative)
  - [Alternatives considered](#alternatives-considered)
  - [Rejection reason](#rejection-reason)

## Owner

[@matlec](https://github.com/matlec)

## Summary

This SUP introduces the **Margo Identity and Authorization Framework (MIAF)** — Margo's common foundation for identity, authentication, and authorization, built on cryptographically verifiable credentials aligned with open cloud-native identity standards (notably [SPIFFE](https://spiffe.io/)).

MIAF specifies:

- A **Trust Domain** model and the **SPIFFE ID** namespace for identifying Margo components.
- An **X.509-SVID** profile (an X.509 certificate carrying a SPIFFE ID in its URI SAN) as the credential principals present.
- The **SPIFFE Bundle Map** as the canonical format for distributing trust anchors, retrieved via an optional **discovery document**.
- The **Margo Identity Service (MIS)** as the identity-authority role within a Trust Domain.
- A cryptographic and TLS baseline shared by all Margo components, with authentication by **mTLS** using X.509-SVIDs validated against the Trust Bundle.

Together with its [active sibling SUP](#active-sibling-sup), this SUP replaces PR1's per-WFM trust anchor distribution, per-WFM client onboarding flow, and ad-hoc cryptographic baseline with a single Trust-Domain-level identity model that any Margo component can adopt.

PR2 ships the foundation only: identity formats, trust material distribution, and mTLS authentication. Automated enrollment, renewal, and revocation are deliberately deferred to PR3 — see [§1](#1-scope-and-structure) for what PR2 implementations look like in the meantime, and the [Roadmap](#8-roadmap-and-forward-extensibility-informative) for what comes later.

## Reason for proposal

Preview Release 1 (PR1) defines a secure Management Interface between a **WFM Client** on an Edge Compute Device and its **Workload Fleet Manager (WFM)**: each WFM assigns the client a `clientId` and binds it to a certificate the device presents at onboarding. That certificate signs HTTP payloads via [RFC 9421](https://datatracker.ietf.org/doc/html/rfc9421); TLS to the WFM is server-authenticated only, using the WFM's separate server certificate.

PR1's identity model has structural limits. Identities are local to one WFM and not portable across the Trust Domain. Trust anchors are distributed per-WFM rather than at the Trust Domain level. There's no shared lifecycle vocabulary, and no foundation other Margo components (DFM, observability, registries) can re-use. These limits become especially painful in multi-vendor deployments: a single Trust Domain may host several WFMs — and, later, DFMs — from different vendors, each shipping its own trust anchors and identity scheme, with no cross-vendor identity recognition.

MIAF fills those gaps by lifting identity to the Trust Domain level using SPIFFE primitives, with the PR1 WFM Client surface updated to consume it in the [active sibling SUP](#active-sibling-sup).

## Requirements alignment acknowledgement

This SUP partially addresses [margo/specification#127 — *Define Margo strategy for ecosystem identity and authorization*](https://github.com/margo/specification/issues/127). Its acceptance criteria are addressed as follows:

- **Inter-vendor authentication** — Trust Domain model with SPIFFE-based identity recognition across vendors ([§3](#3-the-margo-identity-and-authorization-framework-miaf)).
- **Shared trust-anchor distribution** — SPIFFE Trust Bundle retrieved via the discovery document ([§4](#4-discovery-and-trust-bundle-endpoints)).
- **Defined device identity strategy** — *deferred*. PR2 establishes the SPIFFE foundation on which a device identity profile can be defined; the profile itself is deferred (see [Roadmap](#8-roadmap-and-forward-extensibility-informative)).
- **Standard device identity schema** — *deferred*, alongside the device identity profile.

The non-functional requirement to integrate with the existing PR1 `clientId` model is addressed jointly with the [active sibling SUP](#active-sibling-sup).

## Technical proposal

### 1. Scope and Structure

This SUP defines the **MIAF framework**: the Trust Domain model, identity formats, trust material distribution, cryptographic baseline, the MIS role, and the authentication model. The framework is intentionally generic — it does not define an enrollment or renewal protocol, nor a specific identity profile. Those are layered on top:

- The active sibling SUP, **[Margo WFM Identity Profile and Management Interface Update](https://github.com/margo/specification-enhancements/blob/feat/miaf-wfm-identity-sup/proposals/wfm-identity-profile.md)**, defines the SPIFFE path conventions for WFMs and WFM Clients and updates the Management Interface to consume MIAF identities.
- Automated **enrollment, renewal, and revocation** are deferred to PR3 (see [Roadmap](#8-roadmap-and-forward-extensibility-informative)).
- A normative **device identity profile** is deferred (see [Roadmap](#8-roadmap-and-forward-extensibility-informative)).

Authentication is mTLS with an X.509-SVID; authorization is done locally by each verifier based on the peer's verified SPIFFE ID. There is no central authorization server.

In PR2, principals obtain their SVIDs through **operator-pre-provisioning**: the operator mints SVIDs with the right SPIFFE path and installs them through its existing provisioning channel. That's enough to run the trust architecture, and it's the long-term-correct foundation regardless of which automated lifecycle protocol PR3 picks — PR3 will add automation on top without forcing PR2 implementations to change their identity model or authentication mechanism.

#### Relationship to SPIFFE <!-- omit from toc -->

MIAF reuses SPIFFE identity primitives rather than inventing Margo-specific credential formats or trust semantics. This SUP:

- **adopts by reference** the SPIFFE concepts of **Trust Domain**, **SPIFFE ID**, **X.509-SVID**, and **Trust Bundle / Bundle Map**;
- **profiles or constrains** those standards where Margo needs additional rules; and
- **defines Margo-specific behavior** for discovery, the MIS role, and Margo path conventions under `/margo/`.

| Topic | Source | Notes |
| :---- | :----- | :---- |
| SPIFFE ID syntax and validation rules | SPIFFE, adopted by reference | This SUP defines only Margo path conventions where needed. |
| X.509-SVID baseline semantics | SPIFFE, adopted by reference + constrained | This SUP adds the MIAF profile constraints in [X.509 SVID profile](#x509-svid-profile). |
| Trust Bundle / Bundle Map | SPIFFE, adopted by reference | This SUP defines discovery and retrieval conventions around it. |
| Discovery document | Margo | Not part of SPIFFE. |

### 2. Terminology

The following terms define the common vocabulary for Margo's non-human identity and authorization model. Some are adopted directly from open standards such as [**SPIFFE**](https://spiffe.io/); others are Margo-specific concepts introduced by this SUP.

This SUP concerns identities used by *non-human* **Margo components** — logical units of the Margo system such as the Device Fleet Manager (DFM), Workload Fleet Manager (WFM), their clients, and infrastructure services such as registries or observability collectors, as defined in the [Envisioned System Design](https://specification.margo.org/overview/envisioned-system-design/).

#### Terms adopted from SPIFFE <!-- omit from toc -->

The following terms are used as defined by SPIFFE. This SUP does not redefine them; it applies them within the Margo context.

##### Trust Domain <!-- omit from toc -->

The governed security boundary within which identities are issued and mutually recognized. This SUP uses **Trust Domain** in the SPIFFE sense: a trust-root-backed identity namespace and policy boundary. A Trust Domain defines:

- authoritative **trust anchors** (for example, the X.509 authority certificates published for the Trust Domain);
- the namespace for **SPIFFE IDs**; and
- policies for identity lifecycle and authorization.

##### SPIFFE ID <!-- omit from toc -->

A URI of the form `spiffe://<trust-domain>/<path>` that names an identity within a Trust Domain. This SUP adopts SPIFFE ID syntax and validation rules by reference; Margo path conventions are defined where needed (see [Identity model](#identity-model)).

##### SPIFFE Verifiable Identity Document (SVID) <!-- omit from toc -->

The verifiable credential representing an identity within a Trust Domain. An SVID binds a SPIFFE ID to a key pair.

##### Trust Bundle <!-- omit from toc -->

The cryptographic material (X.509 trust anchors) used to validate SVIDs issued within a Trust Domain. Distributed via the SPIFFE Bundle Map; see [Trust Bundles and distribution](#trust-bundles-and-distribution) for the framework rules.

#### Terms introduced by this SUP <!-- omit from toc -->

##### Principal <!-- omit from toc -->

A non-human Margo component that holds — or is being provisioned with — a SPIFFE identity in a Trust Domain. Edge Compute Devices, WFMs, and WFM Clients are all principals under MIAF.

##### Margo Identity Service (MIS) <!-- omit from toc -->

The identity-authority **role** within a Trust Domain. The MIS issues SVIDs, publishes the discovery document and Trust Bundle, and enforces this SUP's cryptographic and SVID-profile rules. The MIS is defined by responsibilities, not by a specific API; see [The MIS role](#the-mis-role).

##### Policy-Based Authorization <!-- omit from toc -->

Each verifier makes authorization decisions locally, based on the peer's verified **SPIFFE ID**. MIAF does not use OAuth-style token scopes or a central authorization server.

### 3. The Margo Identity and Authorization Framework (MIAF)

#### Framework overview

MIAF has four moving parts (full definitions in [§2](#2-terminology)):

1. **Trust Domain** — the security boundary within which MIAF identities are issued and validated. Each SPIFFE ID belongs to exactly one Trust Domain; verifiers **MAY** also accept identities from other Trust Domains via configuration or federation.
2. **Margo Identity Service (MIS)** — the identity-authority role within a Trust Domain. The MIS issues SVIDs, publishes the discovery document and Trust Bundle, and enforces this SUP's cryptographic and SVID-profile rules. The role is not pinned to a specific service API — see [The MIS Role](#the-mis-role).
3. **Margo components** — DFMs, WFMs, their clients, and infrastructure services. They act as **SVID holders** when they authenticate, and as **verifiers** when they validate peer SVIDs.
4. **Trust Bundles** — each Trust Domain publishes a Trust Bundle (a set of X.509 trust anchors) for verifiers to validate SVIDs against. Bundles are distributed via the SPIFFE [Bundle Map](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Trust_Domain_and_Bundle.md).

How it works once a component holds an SVID:

1. **Discover trust material.** The component locates its Trust Domain's Trust Bundle via the discovery document and retrieves it over HTTPS.
2. **Authenticate to peers.** The component opens an mTLS connection, presenting its X.509-SVID.
3. **Authorize the call.** The peer validates the SVID chain against the Trust Bundle and applies its local policy to the verified SPIFFE ID.

How the component *gets* an SVID in PR2 is operator-driven (see [Operator provisioning playbook](#operator-provisioning-playbook-pr2)); automated enrollment, renewal, and revocation are deferred to PR3.

> **Conceptual Trust and Identity Architecture (Informative)**
> The diagram below illustrates MIAF in its most general form: a Margo component holds an X.509-SVID within a governed Trust Domain, then authenticates to peers via mTLS. The Trust Domain publishes the Trust Bundle that participants use to validate identities.
>
> ```mermaid
> flowchart LR
>  %% Figure 1: Conceptual Trust and Identity Architecture (Framework Level, Informative)
>
>  Client["**Margo Client Component**<br/>(e.g., DFM Client, WFM Client, Telemetry Agent)"]
>  Server["**Margo Server Component**<br/>(e.g., DFM, WFM, Observability Platform, Component Registry)"]
>  MIS["**Margo Identity Service (MIS)**<br/>Issues SVIDs, publishes Trust Bundle & discovery"]
>  TD["**Trust Domain**<br/>Defines trust anchors, policies, and namespace"]
>  X509["**X.509 SVID**<br/>Certificate binding SPIFFE ID to key pair"]
>  TB["**Trust Bundle**<br/>X.509 trust anchors"]
>
>  Client -->|"holds X.509 SVID"| X509
>  MIS -->|"issues X.509 SVID"| X509
>  Client -->|"authenticates using X.509 SVID (mTLS)"| Server
>  Server -->|"verifies SVID using Trust Bundle of"| TD
>  TD -->|"publishes"| TB
>
>  classDef comp fill:#e8f1ff,stroke:#5b8def,stroke-width:1px,rx:8px,ry:8px,color:#0b3b8c;
>  classDef ident fill:#e8f7ee,stroke:#2ca36b,stroke-width:1px,rx:8px,ry:8px,color:#0f5132;
>  classDef trust fill:#f7f7f7,stroke:#bdbdbd,stroke-width:1px,rx:8px,ry:8px,color:#333;
>
>  class Client,Server,MIS comp;
>  class X509 ident;
>  class TD,TB trust;
> ```

#### Identity model

- **Identity representation.** An identity is named by a **SPIFFE ID** and represented by an **SVID** issued under the Trust Domain's MIS.
- **Path namespace.** SPIFFE IDs issued under a MIAF identity profile **MUST** have a path beginning with `/margo/`. Each identity profile claims a non-conflicting sub-prefix and defines its structure (path conventions for WFMs and WFM Clients are in the [Margo WFM Identity Profile](https://github.com/margo/specification-enhancements/blob/feat/miaf-wfm-identity-sup/proposals/wfm-identity-profile.md)). To keep `/margo/` a reliable signal of MIAF provenance, non-MIAF SVIDs in the same Trust Domain **MUST NOT** use it.
- **Uniqueness.** Each SPIFFE ID uniquely identifies one component within its Trust Domain.
- **Lifecycle.** All identities follow the [Lifecycle Vocabulary](#lifecycle-vocabulary). In PR2 only the **Active** phase has a fully normative protocol surface.
- **Extensibility.** The MIS, Trust Domain, SVID, and Trust Bundle concepts are intentionally generic; future SUPs may add profiles for new principal classes without redefining the framework.

#### X.509 SVID profile

This SUP adopts the [SPIFFE X.509-SVID specification](https://github.com/spiffe/spiffe/blob/main/standards/X509-SVID.md) by reference. X.509-SVID is the only SVID representation in PR2; JWT-SVID is deferred (see [Roadmap](#8-roadmap-and-forward-extensibility-informative) — whether MIAF adopts JWT-SVIDs at all is itself a PR3 decision).

Identity profiles defined by other SUPs **MAY** further constrain validity periods, key-protection rules, or path conventions for their own principal classes.

Validation and chain delivery (per SPIFFE X.509-SVID):

- When presenting an X.509-SVID, the presenter **MUST** include the leaf SVID plus all intermediate CA certificates needed for path validation. The root **MAY** be omitted. This applies wherever an X.509-SVID is conveyed in this SUP, including the TLS `Certificate` message during mTLS.
- Verifiers **MUST** validate the presented chain against the Trust Domain's Trust Bundle. Verifiers **MUST NOT** rely on AIA fetching or other out-of-band intermediate retrieval.
- Each SPIFFE ID **MUST** be unique within its Trust Domain.

#### Trust Bundles and distribution

This SUP adopts the SPIFFE **Trust Domain and Bundle / Bundle Map** model by reference. Each Trust Domain publishes a **Trust Bundle** — the set of X.509 trust anchors used to validate SVIDs in that domain. A Trust Bundle is authoritative only after retrieval over an HTTPS connection authenticated per [Initial Trust Bootstrap](#initial-trust-bootstrap).

Intermediate CA certificates needed for path validation travel with the SVID chain (see [X.509 SVID Profile](#x509-svid-profile)) — not in the Trust Bundle.

The SPIFFE Bundle Map format also reserves a slot for JWKS material. PR2 does not populate it, and PR2 implementations **MUST** ignore any JWKS material on retrieval. Reserving the slot keeps PR2 forward-compatible should MIAF adopt JWT-SVIDs in a later release.

Bundles:

- **SHOULD** be published and discovered via the SPIFFE [Trust Domain and Bundle Map](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Trust_Domain_and_Bundle.md);
- **MAY** be distributed through deployment tooling or provisioning flows;
- **MUST** be refreshed before expiry or rotation; and
- **SHOULD** be cached locally to support offline validation.

To validate a peer's SVID, a verifier:

1. reads the peer's Trust Domain from its SPIFFE ID;
2. retrieves the SPIFFE Bundle Map from the [discovery document](#discovery-document-endpoint)'s `trustBundleUri` (or from cache) and picks the bundle for that Trust Domain;
3. validates the SVID chain against that bundle; and
4. on success, applies its local authorization policy keyed on the verified SPIFFE ID.

For cross-domain trust, the verifier needs the other Trust Domain's bundle configured locally. A Bundle Map can carry bundles for multiple Trust Domains, which supports this. Full SPIFFE Federation lifecycle semantics are out of scope here.

#### Cryptographic requirements

All cryptographic operations under MIAF — including key generation, SVID signing, and SVID verification — **MUST** conform to [RFC 9325](https://datatracker.ietf.org/doc/rfc9325/) and [NIST SP 800-131A Rev 2](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-131Ar2.pdf).

| Algorithm | Requirements |
| :-------- | :----------- |
| **ECDSA (P-256 + SHA-256)** | Keys **MUST** use curve P-256 (`prime256v1`) and signatures **MUST** use SHA-256 (`ES256`). |
| **RSA (PSS 3072 + SHA-256)** | Modulus **MUST** be >= 3072 bits; signatures **MUST** use RSASSA-PSS with SHA-256 (`PS256`). `RS256` (PKCS#1 v1.5) **MUST NOT** be used for MIAF-generated artifacts. |

Additional rules:

- Components that **present** SVIDs **MUST** implement at least one of the algorithms above and use it for their keys.
- Components that **validate** SVIDs **MUST** implement both algorithms, because they cannot know in advance which algorithm a peer will present.

These requirements apply to MIAF-generated artifacts and to keys used in SVIDs and CSRs. They do not constrain external bootstrap ecosystems that PR3 may integrate (for example, a manufacturer PKI used as a future bootstrap-method input), which **MAY** use algorithms permitted by their governing standards subject to Trust Domain policy.

#### The MIS role

The **Margo Identity Service (MIS)** is a **role**, not a specific service. Within a Trust Domain, the MIS is responsible for:

- issuing X.509-SVIDs to principals (operator-driven in PR2; automated in PR3);
- serving the Trust Bundle retrieval endpoint and, when used, the discovery document endpoint over HTTPS; and
- enforcing this SUP's cryptographic and SVID-profile requirements.

Anything that meets these responsibilities can fill the role: [SPIRE](https://spiffe.io/docs/latest/spire-about/), a CA configured per a future MIAF profile, an operator's pre-provisioning workflow, or something else. This SUP does not define a Margo-specific service API for the MIS — conformance is judged by behavior, not by API surface.

##### Deployment patterns (informative) <!-- omit from toc -->

Three common ways to fulfil the MIS role; the framework requirements above apply equally to all of them.

| Pattern | Description | Typical use case |
| :--- | :---------- | :--------------- |
| **Self-signed Root CA** | A CA operating as a self-signed root, directly issuing SVIDs. | Self-contained or air-gapped environments. |
| **Intermediate CA under enterprise PKI** | A CA operating as an intermediate, chaining SVIDs to an enterprise or offline root. | Enterprise environments aligned with corporate PKI. |
| **SPIFFE-conformant identity service** | A SPIFFE-conformant service such as SPIRE, configured with the Margo path conventions and Trust Bundle distribution. | Cloud-native or service-mesh environments. |

### 4. Discovery and Trust Bundle Endpoints

PR2 specifies two read-only HTTPS endpoints served by the MIS role: an optional **discovery document** that points clients to the Trust Bundle, and the **Trust Bundle retrieval** endpoint itself. Because the MIS is a role rather than a fixed service (see [The MIS role](#the-mis-role)), the origin(s) hosting these endpoints are chosen by the MIS implementation; this SUP constrains only the path convention (when discovery is used) and the response payloads. Both endpoints **MUST** be served over HTTPS authenticated per [Initial Trust Bootstrap](#initial-trust-bootstrap), and clients **MUST** tolerate unknown response fields so future SUPs can add fields without breaking PR2 implementations.

#### Discovery document endpoint

The discovery document is an optional entry point to a Trust Domain that points clients to the Trust Bundle URI. Each document describes exactly one Trust Domain.

When discovery is used:

- An origin serving exactly one Trust Domain **SHOULD** expose the document at `GET /.well-known/margo` per [RFC 8615](https://datatracker.ietf.org/doc/html/rfc8615). An origin serving several Trust Domains **MAY** use other absolute HTTPS URLs.
- Clients **SHOULD** revalidate cached copies using `ETag` and `Last-Modified`.

When discovery is not used, the Trust Domain identifier and Trust Bundle URI are supplied by operator-provided configuration.

| Item | Value |
| :--- | :---- |
| **Endpoint** | `GET <discovery-url>` (default convention: `GET /.well-known/margo`) |
| **Authentication** | None (application-layer); transport per [Initial Trust Bootstrap](#initial-trust-bootstrap) |
| **Headers** | `Accept: application/json` |
| **Body schema (response)** | See below |
| **Responses** | `200 OK` — discovery document<br>`404 Not Found` — not available |

**Response body schema (`200 OK`, `application/json`):**

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `trustDomain` | string | Y  | Identifier of the Trust Domain (e.g., `factory.example`). All SPIFFE IDs issued by the MIS **MUST** belong to this Trust Domain. |
| `trustBundleUri` | string | Y | Absolute HTTPS URL to the **SPIFFE Bundle Map** resource for this Trust Domain. The resource **MUST** conform to the [SPIFFE Trust Domain and Bundle Map specification](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Trust_Domain_and_Bundle.md#5-spiffe-bundle-map) and **MUST** contain an entry for the domain identified by `trustDomain`; that entry is the authoritative local Trust Bundle for this specification. The resource **SHOULD** expose caching headers (`ETag`, `Last-Modified`). |

The discovery document is intentionally extensible: PR2 clients **MUST** ignore unknown fields. Whether — and what — PR3 adds depends on its enrollment-protocol choice. A Margo-specific JSON enrollment API would naturally advertise its own endpoint URL and the set of supported bootstrap methods here; standards-based protocols like Lightweight CMP, EST, or ACME bring their own discovery conventions and may need few or no additional fields in this document.

##### Example: Discovery Document <!-- omit from toc -->

```http
GET /.well-known/margo
Accept: application/json
```

**Response (`200 OK`):**

```jsonc
{
  "trustDomain": "northstar-ida.com",
  "trustBundleUri": "https://mis.northstar-ida.com/.well-known/spiffe/bundle.json"
}
```

#### Trust Bundle retrieval endpoint

The resource identified by `trustBundleUri` returns a SPIFFE Bundle Map. The entry keyed by the local `trustDomain` contains the authoritative set of public trust anchors for that Trust Domain.

| Item | Value |
| :--- | :---- |
| **Endpoint** | `<trustBundleUri>` (for example: `https://mis.example.com/.well-known/spiffe/bundle.json`) |
| **Authentication** | None (application-layer); transport per [Initial Trust Bootstrap](#initial-trust-bootstrap) |
| **Media type** | `application/json` |
| **Body schema (response)** | The response **MUST** conform to the [SPIFFE Bundle Map format](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Trust_Domain_and_Bundle.md#5-spiffe-bundle-map). |
| **Responses** | `200 OK` — Bundle Map retrieved<br>`304 Not Modified` — cached copy still valid<br>`404 Not Found` — bundle unavailable |
| **Caching** | The endpoint **SHOULD** support HTTP caching headers (`ETag`, `Last-Modified`). |

Clients select the Trust Bundle for `trustDomain` from the retrieved Bundle Map and use it as the authoritative source when validating SVIDs issued within the Trust Domain.

### 5. Transport Layer Security (TLS) Requirements

All communication between Margo components — principals presenting SVIDs to peers, clients retrieving the discovery document or Trust Bundle — **MUST** occur over HTTPS. When mTLS is used, the client certificate **MUST** be a valid X.509-SVID issued under the applicable Trust Domain.

#### Initial trust bootstrap

There's a chicken-and-egg problem: clients can't validate MIAF-issued SVIDs against the Trust Bundle until they've retrieved the Bundle Map from `trustBundleUri` — but that retrieval is itself an HTTPS call. So the HTTPS connections used to fetch the discovery document and the Bundle Map need an **initial trust mechanism** established outside MIAF.

Clients **MUST** authenticate both HTTPS connections using at least one of:

1. **PKI-anchored validation:** validate the MIS server certificate chain to a configured set of trust anchors (web PKI, enterprise PKI, or an operator-configured private CA), with DNS name validation per [RFC 6125](https://datatracker.ietf.org/doc/html/rfc6125).
2. **Pinned trust:** validate the MIS server certificate chain or public key against operator-provisioned pins (for example, a pinned CA certificate).

Operators **MAY** deliver the trust material for either option — the configured anchors for (1) or the pins for (2) — via the same channel used to provision the principal's SVID.

After authenticated retrieval, the discovery document (if used) and the Bundle Map are MIAF's authoritative sources; the bundle they select then validates SVIDs within the Trust Domain.

If neither HTTPS connection can be authenticated by one of these mechanisms, the client **MUST** abort. Clients **MUST NOT** treat the first retrieval as unauthenticated or "trust on first use".

#### Minimum TLS baseline

| Requirement | Normative Directive | Reference |
| :---------- | :------------------ | :-------- |
| **Protocol Version** | Implementations **MUST** support **TLS 1.3**. Support for TLS 1.2 **MAY** be provided for compatibility but **MUST** follow modern security guidance. | [RFC 8446 — TLS 1.3](https://datatracker.ietf.org/doc/html/rfc8446), [RFC 9325 §4](https://datatracker.ietf.org/doc/html/rfc9325#section-4) |
| **Deprecated Versions** | SSL v2, SSL v3, TLS 1.0, and TLS 1.1 **MUST NOT** be used. | [RFC 8996](https://datatracker.ietf.org/doc/html/rfc8996) |

#### Certificate validation

Each endpoint **MUST** validate the peer's TLS certificate chain and identity in accordance with [RFC 5280 — X.509 Certificate Profile](https://datatracker.ietf.org/doc/html/rfc5280).

Validation **MUST** include the following, depending on context:

- The validating party **MUST** verify that the presented certificate chain is within its validity period and **MUST** reject expired certificates.

- **Server identity for MIAF HTTPS endpoints (discovery and Trust Bundle retrieval):**

  - The client **MUST** validate the server certificate chain to its configured initial trust anchors (see [Initial Trust Bootstrap](#initial-trust-bootstrap)).
  - The client **MUST** validate the expected DNS name per [RFC 6125 — Service Identity in TLS](https://datatracker.ietf.org/doc/html/rfc6125).
  - The Trust Bundle selected from `trustBundleUri` **MUST NOT** replace these TLS server-validation checks for MIAF HTTPS endpoints; it is used to validate SVIDs within the Trust Domain.

- **SVID identity for MIAF mTLS (X.509-SVIDs):**

  - The verifier **MUST** validate any presented X.509-SVID chain against the Trust Domain's Trust Bundle.
  - The verifier **MUST** extract the SPIFFE ID from the **URI SAN** and treat it as the **authoritative identity claim**.
  - DNS hostname matching is not applicable to SVID identity validation and **MUST NOT** override SPIFFE ID validation.
  - The structural SVID constraints from the [X.509 SVID profile](#x509-svid-profile) and the referenced SPIFFE X.509-SVID specification (URI SAN cardinality, path constraints, etc.) also apply during validation.

  PR2 does not standardize a revocation status endpoint for X.509-SVIDs; deployments use the [Operator Revocation Playbook](#operator-revocation-playbook-pr2). A standardized status mechanism is on the [Roadmap](#8-roadmap-and-forward-extensibility-informative).

#### Session lifetime and re-validation

mTLS authenticates a peer at handshake; on a long-lived or pooled connection the authenticated identity would otherwise persist unchecked for the connection's life. Because PR2 relies on short SVID lifetimes and on allowlist / Trust-Bundle changes for revocation rather than an online status mechanism, a connection that outlives its peer's SVID — or that survives a Trust-Bundle rotation — extends the revocation lag for as long as it stays open.

A verifier therefore **SHOULD** bound the period for which an authenticated connection remains in service after the SVID that established it has expired, or after that SVID would no longer validate against the current Trust Bundle. That bound **SHOULD** be short relative to the SVID lifetimes in use (see [SVID Lifetime Guidance](#svid-lifetime-guidance-pr2)).

Bounding the **maximum lifetime of a connection** — a maximum connection age, after which the connection is closed and re-established — satisfies this recommendation: the re-establishing handshake re-validates the peer's current SVID against the current Trust Bundle. A verifier **MAY** instead, or additionally, re-validate the peer SVID on the open connection and close it when it is no longer valid, and **MAY** tighten the expiry bound to the SVID's own `notAfter`. Separately, a verifier **SHOULD** re-evaluate its local authorization policy for the peer's SPIFFE ID on each request, so allowlist removals take effect without waiting for the connection to close.

This SUP does not rely on TLS renegotiation or post-handshake authentication. Clients **SHOULD** proactively re-establish affected connections after renewing their own SVID.

#### Scope: traffic-inspecting proxies

PR2 supports end-to-end mTLS between principals and relying parties. **TLS-offloading proxies** (e.g., AWS ALB in verify-with-trust-store mode, NGINX, Envoy) that terminate mTLS at the proxy and forward the validated client certificate identity to the backend — for example, via the [RFC 9440](https://datatracker.ietf.org/doc/html/rfc9440) `Client-Cert` header — are supported. The proxy validates the client certificate against the Trust Bundle, the backend extracts the authenticated SPIFFE ID from the forwarded header, and the proxy-to-backend boundary is trusted by network segmentation or by mTLS between proxy and backend.

PR2 does **not** support **traffic-inspecting proxies** (NGFW, SWG, SASE products) in the inline path between principals and relying parties. Traffic inspection requires the proxy to MITM TLS for content inspection, which is architecturally incompatible with mTLS. Operators **MUST** exempt Margo mTLS endpoints from inspection — the same pattern used by Open Banking, payment processors, IoT device platforms, and other B2B mTLS APIs, and supported as an explicit configuration option by all major inspection-proxy products.

PR3 will address deployments where exemption is operationally infeasible. Two candidate mechanisms are on the table — an HTTP message-signature profile keyed to the X.509-SVID, and a JWT-SVID exchange endpoint issuing short-lived bearer credentials — but the choice between them is deliberately deferred.

### 6. Lifecycle Vocabulary and Operator Playbooks

#### Lifecycle Vocabulary

Margo identities go through five lifecycle phases. In PR2, only the **Active** phase has a fully normative protocol surface; the rest are operator-driven (see [Operator Provisioning Playbook](#operator-provisioning-playbook-pr2) and [Operator Revocation Playbook](#operator-revocation-playbook-pr2)).

- **Enrollment** — initial issuance of an SVID for a principal.
- **Active** — the principal holds a valid SVID and uses it to authenticate. Fully specified in PR2: mTLS authentication, SVID validation against the Trust Bundle, recognition by relying parties.
- **Renewal** — refresh of an SVID before expiry.
- **Revocation** — declaration that an issued SVID is no longer valid before its natural expiry.
- **Re-issuance** — issuance of an SVID with the same SPIFFE ID to a replacement principal (for example, after device replacement).

Automated mechanisms for the non-active phases are deferred to PR3.

#### Operator provisioning playbook (PR2)

The Trust Bundle and discovery document are the only runtime artifacts MIAF defines in PR2; everything else flows through the operator's existing provisioning channel.

For **Enrollment**, the operator:

1. accepts a CSR from the principal — the preferred path, since it keeps the private key on the principal and supports hardware-bound keys (TPM, secure element, HSM). Where the principal cannot generate its own key pair, the operator generates one centrally and accepts the resulting key-custody concentration risk;
2. mints an X.509-SVID for the chosen SPIFFE ID under the Trust Domain's issuing authority;
3. installs the SVID on the principal (along with the private key, only if generated centrally, through a confidentiality-protected channel); and
4. ensures every relying party the principal will authenticate to has the Trust Bundle and any local-policy entries needed to recognize the new SPIFFE ID.

For **Renewal**, the operator repeats steps 1–3 before the current SVID expires, replacing the prior SVID in place.

For **Re-issuance** after principal replacement, the operator follows the same workflow on the replacement principal. Whether to reuse the original SPIFFE ID or assign a fresh one is an operator-policy choice.

The provisioning channel itself is deployment-specific and out of scope — typical options include device-management tooling, configuration management, HSM workflows, or out-of-band installer media.

#### Operator revocation playbook (PR2)

Without an automated revocation protocol, PR2 deployments revoke SVIDs through one of:

1. **Relying-party allowlist removal** — the operator removes the SPIFFE ID from each relying party's accepted list. Most precise: invalidates one principal without affecting any other. Recommended for routine revocation.
2. **Trust Bundle rotation** — the operator removes the compromised issuing CA from the Trust Bundle, invalidating all SVIDs issued under it. Heavy-handed, but effective when an entire issuance authority is compromised.
3. **Expiry** — wait for the SVID to expire. Viable only with shorter SVID lifetimes.

PR3 will introduce a relying-party-side revocation status mechanism (see [Roadmap](#8-roadmap-and-forward-extensibility-informative)).

#### SVID lifetime guidance (PR2)

MIAF is designed to favour **short SVID lifetimes** — short lifetimes keep blast radius low and let expiry double as revocation. The right number depends on the principal: a workload identity in a connected service can renew hourly; a device with intermittent or air-gapped connectivity needs enough margin to renew before a realistic offline window ends. As a rough orientation, automated SPIRE-style workload SVIDs are commonly 1–24 hours; long-lived device SVIDs are often weeks to a few months.

PR2 implementations **MAY** use longer lifetimes than they would under automated renewal, because manual provisioning makes short lifetimes operationally expensive. Treat any PR2-driven extension as a bridge: when automated renewal lands, operators **SHOULD** reduce lifetimes to the shortest value compatible with their principals' connectivity. A normative lifetime ceiling for devices comes with the device identity profile, whenever that lands.

### 7. Security Considerations

MIAF assumes an adversarial network and the possibility of compromised individual devices or services. Its goals are to protect private keys, preserve identity integrity, and minimize blast radius. The residual risks below are summarized; concrete mitigations live in the linked sections.

| Threat | Description | Mitigation  |
| :----- | :---------- | :---------- |
| **Replay or Theft of SVIDs** | An intercepted SVID is reused outside its intended context. | SVIDs **SHOULD** be short-lived per [SVID Lifetime Guidance](#svid-lifetime-guidance-pr2); verifiers **MUST** validate the SVID chain against the Trust Bundle and verify the SPIFFE ID before authorization; private keys **MUST** be protected by the principal (mechanism deployment-specific in PR2). |
| **Private Key Compromise** | An attacker exfiltrates a principal's private key. | Operators provision principals such that private keys are protected per deployment policy. Normative key-protection rules for devices come with the device identity profile, whenever that lands. |
| **Initial Trust Anchor Confusion** | An attacker causes a principal to trust the wrong HTTPS authority and serves a malicious discovery document or SPIFFE Bundle Map. | Principals **MUST** authenticate the first HTTPS retrieval of the discovery document and `trustBundleUri` per [Initial Trust Bootstrap](#initial-trust-bootstrap). |
| **Certificate Revocation Lag** | Revocation events are not propagated promptly. | PR2 deployments rely on the [Operator Revocation Playbook](#operator-revocation-playbook-pr2) and short SVID lifetimes. A standardized status mechanism is on the [Roadmap](#8-roadmap-and-forward-extensibility-informative); and verifiers **SHOULD** bound connection lifetime so a long-lived mTLS session does not extend that lag indefinitely (see [Session lifetime and re-validation](#session-lifetime-and-re-validation)). |
| **Service Impersonation / MITM** | An adversary attempts to impersonate the MIS or another service. | All endpoints **MUST** use HTTPS with strict certificate validation per [§5](#5-transport-layer-security-tls-requirements); peers **MUST** verify presented SVIDs against the configured Trust Domain and Trust Bundle. |
| **Cross-Domain Trust Confusion** | Components accept identities from unintended Trust Domains. | Verifiers **MUST** determine the Trust Domain from the SPIFFE ID and **MUST NOT** trust SVIDs unless the domain is explicitly configured or federated. |
| **Inadvertent inspection-proxy MITM** | An operator-deployed traffic-inspecting proxy intercepts and re-signs Margo mTLS traffic, presenting a substitute certificate to either endpoint. | PR2 deployments **MUST** exempt Margo mTLS endpoints from inspection per [Scope: traffic-inspecting proxies](#scope-traffic-inspecting-proxies). Both endpoints **MUST** validate peer SVIDs against the Trust Bundle and reject substituted certificates. |

### 8. Roadmap and Forward Extensibility (Informative)

#### Active sibling SUP <!-- omit from toc -->

One active sibling SUP ships alongside this one for PR2: the **[Margo WFM Identity Profile and Management Interface Update](https://github.com/margo/specification-enhancements/blob/feat/miaf-wfm-identity-sup/proposals/wfm-identity-profile.md)**. It defines the SPIFFE path conventions for WFMs and WFM Clients, the rules each side uses to recognize the other, and the application of MIAF authentication to the existing PR1 Management Interface — dropping `{clientId}` from URL paths, replacing RFC 9421 HTTP Message Signatures with mTLS, and removing the PR1 onboarding and certificate-distribution endpoints. It depends on this SUP for the identity foundation, X.509-SVID profile, Trust Bundle distribution, and mTLS baseline.

#### Work deferred beyond PR2 <!-- omit from toc -->

PR2 is written to stay forward-compatible with however a later release resolves each of the following. Most items below are scoped to PR3; a couple may land later. The wording of each bullet calls out which.

- **Automated enrollment, renewal, and revocation protocols.** PR3 will specify the protocol or protocols used by the MIS role to issue and rotate SVIDs and to publish revocation status. The selection may differ by principal class — cloud-hosted components (such as WFMs) and edge-deployed components (such as devices and WFM Clients) have distinct connectivity and attestation characteristics, and PR3 may profile separate protocol families accordingly. Candidates under consideration include Lightweight CMP ([RFC 9483](https://datatracker.ietf.org/doc/html/rfc9483)), EST ([RFC 7030](https://datatracker.ietf.org/doc/html/rfc7030)), ACME ([RFC 8555](https://datatracker.ietf.org/doc/html/rfc8555); with Device Attestation for attested edge principals), and a Margo-specific JSON API.
- **JWT-SVID support.** PR3 will decide whether MIAF supports JWT-SVIDs and, if so, by what mechanism (direct issuance, or an exchange endpoint that converts X.509-SVIDs into JWT-SVIDs). The SPIFFE Bundle Map already reserves a slot for JWKS material; PR2 does not populate it.
- **Device identity profile.** A normative device identity profile is on the table for PR3 but is not yet committed. The Margo Device Fleet Manager (DFM) work that naturally pairs with device identity is scheduled to ramp up only after the first GA release, so device identity may instead land alongside DFM in a later release. If and when a device identity profile is specified, the framing decision — is device identity a *foundation* for WFM Client identity, or a *peer profile* (one of several bootstrap mechanisms alongside operator-pre-provisioning, enrollment-token, FDO, etc.) — is decided then. PR2 stays silent on this relationship in either case.
- **Bootstrap mechanisms.** PR3 will specify how principals authenticate the first SVID-issuance request. Candidate mechanisms include factory certificate, device-SVID-attested (the candidate authenticates to the MIS with an existing device SVID), operator enrollment token, and FDO. How "a bootstrap mechanism" is expressed on the wire — a polymorphic credential envelope in a Margo-specific JSON API, vs. a CMP message protection profile under LWCMP — depends on the enrollment-protocol choice.
- **WFM Client enrollment protocol.** PR3 will specify the automated path by which WFM Clients receive their SVIDs. Operator-pre-provisioning (the PR2 baseline) remains a valid path in PR3 for deployments that prefer it.
- **Traffic-inspecting-proxy authentication for inline deployments.** Where exemption from traffic inspection is operationally infeasible, PR3 will specify a complementary authentication mechanism (candidates: an HTTP message-signature profile keyed to the X.509-SVID, or a JWT-SVID exchange endpoint).
- **Revocation status mechanism** for relying-party-side checking. The exact mechanism depends on the enrollment-protocol choice (OCSP/CRLs for standards-based protocols; a Margo-specific revocation endpoint under a Margo-JSON enrollment API).

#### Catalog of PR3 inputs <!-- omit from toc -->

Exploratory sketches that feed into PR3 deliberations — early design fragments from prior MIAF drafts, candidate bootstrap methods, and related design notes — are collected under [`proposals/miaf-pr3-inputs/`](./miaf-pr3-inputs/). They are not committed direction; PR3 may engage with, replace, or set aside each as it makes its decisions. See the [miaf-pr3-inputs index](./miaf-pr3-inputs/README.md) for the topic-organized catalog.

## Alternatives considered

Because PR2 defers the automated enrollment, renewal, and revocation protocol decision to PR3, this section is intentionally brief. PR3 will engage with the alternatives in depth — including Lightweight CMP ([RFC 9483](https://datatracker.ietf.org/doc/html/rfc9483)), EST ([RFC 7030](https://datatracker.ietf.org/doc/html/rfc7030)), ACME with Device Attestation, and a Margo-specific JSON API — and will document the rationale for the chosen approach.

The PR2 scope of SPIFFE-based identity formats, Trust Bundle distribution, and mTLS authentication is stable across all candidate PR3 enrollment protocols. The alternatives below are evaluated against that PR2 scope only:

- **Vendor-specific PKI hierarchies.** Rejected because per-vendor roots would fragment the ecosystem and prevent cross-vendor interoperability. MIAF's SPIFFE-based Trust Domain model provides a unified namespace and validation semantics across all vendors and components.
- **Identity as static bearer tokens.** Rejected because static tokens cannot offer hardware-bound, verifiable authenticity or controlled lifecycle management. MIAF uses cryptographically verifiable SVIDs presented over mTLS, ensuring continuous proof of possession.
- **Proprietary API-key or non-standard authorization schemes.** Rejected because they lack verifiable identity semantics and auditable lifecycle management. MIAF adopts policy-based authorization directly over verified SPIFFE IDs.

## Rejection reason

Not applicable.
