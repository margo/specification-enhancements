# Specification Update Proposal <!-- omit from toc -->

- [Owner](#owner)
- [Summary](#summary)
- [Reason for proposal](#reason-for-proposal)
- [Requirements alignment acknowledgement](#requirements-alignment-acknowledgement)
- [Technical proposal](#technical-proposal)
  - [1. Scope and Structure](#1-scope-and-structure)
  - [2. Terminology](#2-terminology)
  - [3. Margo Identity and Authorization Framework (MIAF)](#3-margo-identity-and-authorization-framework-miaf)
    - [Framework overview](#framework-overview)
    - [Identity model](#identity-model)
    - [SVID profiles and negotiation](#svid-profiles-and-negotiation)
      - [X.509 SVID Profile](#x509-svid-profile)
      - [JWT SVID Profile](#jwt-svid-profile)
    - [Trust Bundles and Distribution](#trust-bundles-and-distribution)
    - [Cryptographic Requirements](#cryptographic-requirements)
    - [MIS Deployment Modes (Informative)](#mis-deployment-modes-informative)
  - [4. Edge Compute Device Identity Profile](#4-edge-compute-device-identity-profile)
    - [Profile Scope](#profile-scope)
    - [Logical Device Identity](#logical-device-identity)
    - [Logical Device Identity Lifecycle](#logical-device-identity-lifecycle)
    - [Profile-specific Constraints on the X.509 SVID Profile](#profile-specific-constraints-on-the-x509-svid-profile)
    - [Profile-specific Enrollment and Identity Issuance](#profile-specific-enrollment-and-identity-issuance)
    - [Device Key Protection](#device-key-protection)
    - [JWT SVID Exchange (Optional)](#jwt-svid-exchange-optional)
  - [5. APIs](#5-apis)
    - [Common URI and Encoding Rules](#common-uri-and-encoding-rules)
    - [Discovery Document Endpoint](#discovery-document-endpoint)
    - [Trust Bundle Retrieval Endpoint](#trust-bundle-retrieval-endpoint)
    - [Enrollment and Identity Issuance Endpoint](#enrollment-and-identity-issuance-endpoint)
    - [SVID Renewal Endpoint](#svid-renewal-endpoint)
    - [JWT SVID Exchange Endpoint](#jwt-svid-exchange-endpoint)
    - [Revocation List Endpoint](#revocation-list-endpoint)
  - [6. Typical Workflows (informative)](#6-typical-workflows-informative)
    - [End-to-End Device Lifecycle Flow](#end-to-end-device-lifecycle-flow)
    - [Device SVID Renewal Flow](#device-svid-renewal-flow)
    - [JWT SVID Usage in Proxy Scenarios](#jwt-svid-usage-in-proxy-scenarios)
    - [Bootstrap Method Flows](#bootstrap-method-flows)
      - [Example: Factory Certificate Method (mTLS)](#example-factory-certificate-method-mtls)
      - [Example: Factory Certificate Method (JWT Assertion)](#example-factory-certificate-method-jwt-assertion)
      - [Example: FIDO Device Onboard (MIS-hosted OOS)](#example-fido-device-onboard-mis-hosted-oos)
  - [7. Transport Layer Security (TLS) Requirements](#7-transport-layer-security-tls-requirements)
    - [Relationship to MIAF and Profiles](#relationship-to-miaf-and-profiles)
    - [Initial Trust Bootstrap](#initial-trust-bootstrap)
    - [Minimum TLS Baseline](#minimum-tls-baseline)
    - [Certificate Validation](#certificate-validation)
  - [8. Security Considerations](#8-security-considerations)
    - [Threat Model Overview](#threat-model-overview)
  - [9. Future Work: WFM Client Identity Profile (Informative)](#9-future-work-wfm-client-identity-profile-informative)
- [Alternatives considered (optional)](#alternatives-considered-optional)
  - [Certificate-Based Device Enrollment Protocols](#certificate-based-device-enrollment-protocols)
    - [Rationale for standardized `/api/v1/identities` endpoint](#rationale-for-standardized-apiv1identities-endpoint)
  - [OAuth 2.0 / Authorization Server Integration](#oauth-20--authorization-server-integration)
  - [Alternative Trust Frameworks](#alternative-trust-frameworks)
- [Appendix A: Bootstrap Methods (Normative)](#appendix-a-bootstrap-methods-normative)
  - [Common Validation Rules](#common-validation-rules)
  - [FIDO Device Onboard (FDO) Method](#fido-device-onboard-fdo-method)
  - [Factory Certificate Method (mTLS)](#factory-certificate-method-mtls)
  - [Factory Certificate Method (JWT Assertion)](#factory-certificate-method-jwt-assertion)
  - [IEEE 802.1AR Method](#ieee-8021ar-method)
- [Appendix B: Error Responses (Normative)](#appendix-b-error-responses-normative)
  - [Error Representation Format](#error-representation-format)
  - [Problem Details Object Schema](#problem-details-object-schema)
  - [Error Type Conventions](#error-type-conventions)
  - [Error Handling for Specific APIs](#error-handling-for-specific-apis)
  - [Example - Unsupported Bootstrap Method](#example---unsupported-bootstrap-method)
  - [Client Behavior Recommendations](#client-behavior-recommendations)
- [Appendix C: OAuth2 and API Gateway Interoperability (Informative)](#appendix-c-oauth2-and-api-gateway-interoperability-informative)
  - [Purpose and Context](#purpose-and-context)
  - [Integration Models](#integration-models)
    - [Model 1 - Token Exchange Bridge](#model-1---token-exchange-bridge)
      - [Token Exchange Request](#token-exchange-request)
      - [Token Exchange Response](#token-exchange-response)
      - [Validation and Security Considerations](#validation-and-security-considerations)
    - [Model 2 - Federated AS Validation](#model-2---federated-as-validation)
      - [Validation and Security Considerations](#validation-and-security-considerations-1)
    - [Model 3 - Gateway Policy Mapping](#model-3---gateway-policy-mapping)
      - [Validation and Security Considerations](#validation-and-security-considerations-2)

## Owner

[@matlec](https://github.com/matlec)

## Summary

This Specification Update Proposal introduces the **Margo Identity and Authorization Framework (MIAF)** and defines its first normative **Edge Compute Device Identity Profile**.

**MIAF** provides a common foundation for identity, authentication, and authorization across all **Margo components** - including the **Device Fleet Manager (DFM)**, **Workload Fleet Manager (WFM)**, and their respective clients.
With this SUP, that vision becomes tangible through its first concrete application: a unified identity model for Edge Compute Devices.

MIAF defines:

- a **Margo Identity Service (MIS)** that issues, renews, and revokes identities within a **Trust Domain**, and
- a shared identity and trust model based on cryptographically verifiable credentials, aligned with open cloud-native identity standards, with its first normative application defining device identity.

A **Trust Domain** is the security boundary of a Margo deployment - for example, an end-user's factory network or a multi-tenant environment operated by a fleet management vendor. Within a Trust Domain, identities, policies, and trust anchors are governed consistently and understood by all participating Margo components.

This SUP defines roles, profiles, and APIs for Margo deployments. It does **not** define or provide a centrally operated identity service.

Preview Release 1 (PR1) focuses on **Workload Fleet Management** and defines the **Margo Management Interface for Workload Fleet Management**, where a **WFM Client**, running on an Edge Compute Device (standalone or as a single-node Kubernetes cluster), connects securely to its **WFM** using client-provided certificates and WFM-specific *client identifiers*.
This proves that a Margo Management Interface can be secured, but it does **not** define a portable, verifiable identity for the **Edge Compute Device** itself, nor a shared trust model that other Margo components can reuse.

This SUP therefore does two things:

1. **Defines MIAF** - a framework for non-human identity and authorization in Margo, based on cryptographically verifiable identities following open, widely adopted cloud-native standards. It aligns Margo with modern IT practices while remaining extensible to enterprise PKI and API gateway infrastructures where needed.
2. **Applies MIAF to Edge Compute Devices** through the **Edge Compute Device Identity Profile**, which introduces a persistent, verifiable **device identity**, a defined lifecycle, and an **extensible bootstrap mechanism**. This model allows Margo to leverage standard onboarding methods (for example, **FIDO Device Onboard**, **IEEE 802.1AR DevID**, **factory certificates**) and to support **late binding** to a Trust Domain.

In short:

> **PR1 gave us WFM Client identities. This SUP gives us Device identities - under a framework that every Margo component can build on.** MIAF directly replaces PR1's device onboarding, trust anchor distribution, and cryptographic requirements with a unified, standards-based approach, and lays the groundwork for replacing PR1's client authentication model. A future **WFM Client Identity Profile** will build on this device identity foundation to address the distinct authentication needs of WFM Clients across standalone, cluster, and gateway topologies.

This proposal is:

- a key enabler for **Device Fleet Management**,
- a foundation for **multi-vendor deployments**, where Edge Compute Devices, DFMs, and WFMs can share a consistent and verifiable device identity model, and
- a **common basis** for Margo components to:

  - **express and verify identities** (who is calling), and
  - **enforce authorization policies** (what they are allowed to do)

  within a shared **Trust Domain**.

Finally, this identity foundation **enables** - though does not yet integrate with - related work such as the **[Gateway SUP](https://github.com/margo/specification-enhancements/blob/28f04d64e8cedad8b82dad09840d0918bf6c699a/proposals/single-client-for-multiple-devices.md)** by providing the Logical Device Identity model that gateway-capable Edge Compute Devices could build upon in the future.

## Reason for proposal

### Current state in PR1 <!-- omit from toc -->

Preview Release 1 (PR1) focuses on **Workload Fleet Management (WFM)** and defines a secure Management Interface between a **WFM Client**, running on an Edge Compute Device, and its corresponding **Workload Fleet Manager**.

- Each WFM assigns a `client_id` to the WFM Client and associates it with a **client-provided certificate**.
- This certificate is used to:

  - establish a secure HTTPS connection (server-side TLS authentication), and
  - sign HTTP payloads using an [RFC 9421](https://datatracker.ietf.org/doc/html/rfc9421)-based request-signing mechanism.

This design achieves its goal for PR1: it enables the WFM Client to authenticate to its WFM and securely exchange **desired and actual workload deployment state** and **device capability information**.

However, PR1's identity model ends at the WFM boundary. It does **not** define:

- how to represent the **Edge Compute Device** itself as a verifiable identity within Margo,
- how other Margo components (for example, the **Device Fleet Manager (DFM)**, **Observability services**, or **Container Registries**) can rely on a shared trust model, or
- how device or client identities are **enrolled, renewed, revoked, or audited** throughout their operational lifetime.

As a result, the current system cannot provide consistent trust or authorization across multiple Margo components or vendors.

### Limitations of the PR1 Identity Model <!-- omit from toc -->

While PR1 successfully secures the Management Interface for WFM, its identity model is narrowly scoped to that single interface.
It does not yet provide a framework for consistent, lifecycle-managed identities across the broader Margo ecosystem.

- **No standardized device identity:** PR1 identifies WFM Clients, but not the underlying hardware platform. Other components (such as DFMs, registries, or observability platforms) cannot correlate activity to a specific physical device across the fleet.
- **Siloed trust anchors:** Each WFM manages its own trust configuration. There is no shared mechanism to distribute trust anchors across a deployment, requiring operators to manually configure trust for every new component or vendor.
- **Undefined identity lifecycle:** PR1 provides no guidance on how device credentials are renewed, rotated, or revoked over time. This creates operational risk for long-lived devices and large fleets, where credentials must be managed automatically and at scale.
- **No shared authorization model:** PR1 defines only binary trust decisions (trusted or untrusted) within the WFM boundary. There is no consistent way for other Margo components to evaluate what a verified entity is *allowed* to do based on its identity.
- **Unutilized supply-chain credentials:** Many devices already include hardware-based credentials such as TPM keys, FIDO Device Onboard (FDO) vouchers, or IEEE 802.1AR DevIDs. PR1 does not define how these can be reused for onboarding, leading to proprietary, non-interoperable solutions.

This proposal addresses these gaps by clearly separating two concepts:

- a **Device Identity**, representing the physical or virtual platform itself, and
- a **Client or Workload Identity**, representing the software components that operate on that platform (such as the WFM Client).

The new device-level identity defined in this SUP establishes a **trusted foundation** for the platform - a verifiable, hardware-bound identity that proves the authenticity of the device within a Trust Domain.
Once this trusted base exists, additional software components running on the device (for example, WFM Clients or managed workloads) can securely obtain their **own, distinct identities** in future extensions of the framework.
This layered model ensures that hardware trust and software trust are managed independently, enabling secure, auditable, and interoperable relationships across the Margo ecosystem.

### What this SUP introduces <!-- omit from toc -->

To address these limitations, this SUP **replaces PR1's device identity and onboarding model, and lays the foundation for replacing its client authentication model,** by introducing two closely related elements:

1. **The Margo Identity and Authorization Framework (MIAF):** A shared framework for all Margo components, based on cryptographically verifiable identities and a unified trust-domain model.
   It enables authentication and authorization decisions to be made directly using verifiable identities.

2. **The Edge Compute Device Identity Profile:** The first concrete application of MIAF, providing a consistent, hardware-bound identity model for Edge Compute Devices.
   This profile defines:

   - a stable **device identity** for each edge node,
   - how that identity is represented in X.509 certificates, and
   - lifecycle operations and **bootstrap methods** that map existing hardware credentials (for example, FDO, TPM, or IEEE 802.1AR) into the Margo identity model.

Together, these establish the foundation for interoperable identity and trust across Margo components and vendors, replacing PR1's device onboarding and trust model with a unified, lifecycle-managed approach and providing the basis for future WFM Client identity changes.

### Relationship to PR1 <!-- omit from toc -->

MIAF replaces core elements of the device identity, trust, and onboarding model introduced in Preview Release 1, and defines the framework that will enable replacement of WFM client authentication in a future profile.

**Directly replaced by this SUP:**

- **Device identity foundation:** PR1's device-facing identity and per-WFM trust model are replaced by Trust Domain-scoped device identities issued by the Margo Identity Service (MIS). The device's identity is no longer WFM-specific but portable across all Margo components within the Trust Domain.
- **Onboarding:** PR1's WFM-centric onboarding flow (`POST /api/v1/onboarding`) is replaced by MIAF's bootstrap and enrollment mechanism (`POST /api/v1/identities`), which binds a device's Physical Device Identity to a Logical Device Identity within the Trust Domain.
- **Trust anchor distribution:** PR1's per-WFM root CA endpoint (`GET /api/v1/onboarding/certificate`) is replaced by the SPIFFE Trust Bundle, retrieved from a standardized Trust Bundle endpoint and distributed in the SPIFFE Bundle Map format.
- **Cryptographic requirements:** PR1's permitted signature algorithms are superseded by MIAF's cryptographic requirements.
- **Device security requirements:** PR1's informational references to hardware key protection (TPM, secure boot, attestation) become normative requirements under MIAF's [Device Key Protection](#device-key-protection) section.

**Enabled but not yet defined by this SUP:**

A dedicated **WFM Client Identity Profile** (see [Future Work](#future-work-wfm-client-identity-profile-informative)) is expected to:

- replace PR1's WFM-specific `client_id` model with a MIAF-defined WFM Client identity;
- define the SPIFFE ID path format for WFM Client identities within the Trust Domain;
- define how device identities are used to bind WFM Client credentials for each supported deployment topology;
- address the lifecycle requirements of each supported topology (standalone, cluster, gateway); and
- inform the corresponding updates to the WFM API specification (for example, replacing `{clientId}` path parameters and PR1's [RFC 9421](https://datatracker.ietf.org/doc/html/rfc9421) HTTP Message Signatures security scheme with MIAF-based authentication).

Until that profile is defined, this SUP does **not** specify how WFM Clients authenticate to WFM APIs. A device SVID is not itself a WFM Client credential. The device identity defined here is the prerequisite for, but not a substitute for, WFM Client identity.

The WFM API's **API behavior** - desired state retrieval, deployment content distribution, and status reporting - is not changed by this SUP.

## Requirements alignment acknowledgement

This SUP supports Margo's core goals of **security**, **scalability**, **interoperability**, and **flexibility**, and aligns directly with the Product Management Group's roadmap epics.

### Security <!-- omit from toc -->

- Establishes a **cryptographically verifiable identity model** for Edge Compute Devices within a Trust Domain, using open, cloud-native non-human identity standards.
- Introduces stronger guidance and extensions around protecting private keys associated with device identities, refining the Margo Device Requirements specification.
- Authentication - and the basis for authorization decisions - is provided directly by verified device identities, rather than opaque, ad-hoc credentials.
- The MIS and associated trust-bundle mechanisms provide standard, auditable points for identity issuance, renewal, and revocation.

### Scalability <!-- omit from toc -->

- Separates **identity issuance and lifecycle management** (MIS) from individual consuming components, allowing MIS to scale independently while devices and services validate identities locally.
- Margo components such as the DFM can validate identities using trust bundles and profile rules, avoiding per-session coupling with MIS and minimizing centralized state.
- Standard lifecycle APIs (enrollment, renewal, revocation, replacement, termination) and a consistent Logical Device Identity model simplify long-term fleet management.

### Interoperability <!-- omit from toc -->

- Builds on open, widely adopted cloud-native identity standards (for example, SPIFFE/SVID) for non-human identity.
- Integrates cleanly with existing enterprise PKI and identity infrastructures that can issue or validate the underlying certificates used for device identities.
- Defines a **bootstrap-independent Logical Device Identity format**, enabling devices from multiple vendors to interoperate within a shared Trust Domain and be managed consistently by DFM and other components.
- Provides informative mappings to OAuth2 or API-gateway models for deployments that wish to reuse existing authorization infrastructure.

### Flexibility and resilience <!-- omit from toc -->

- The **pluggable bootstrap mechanism** supports multiple Physical Device Identity proofs (FIDO Device Onboard, IEEE 802.1AR DevID, factory certificates), ensuring wide hardware and supply-chain coverage.
- All supported bootstrap methods converge to the same Logical Device Identity, allowing operators to:

  - start with existing factory credentials, and
  - raise assurance levels over time (for example, mandate FDO or 802.1AR for production) without changing DFM or other consumers of the identity model.
- Support for both mTLS-based and JWT-style identity representations (for example, JWT-SVID in proxy-rich environments) allows deployments to operate behind TLS-terminating infrastructure while keeping a single, consistent identity model.

### Alignment with Product Management Epics <!-- omit from toc -->

- **[Parent Epic 5: Build a Margo compatible edge device (#41)](https://github.com/margo/product_management/issues/41):**
  Satisfies **[#56](https://github.com/margo/product_management/issues/56)** by defining the minimal identity and security capabilities every Margo-compatible device must implement and by extending the device requirements around key protection and identity representation.
- **[Parent Epic 6: Enroll an edge device with a workload fleet manager (#42)](https://github.com/margo/product_management/issues/42):**
  Replaces PR1's WFM-centric onboarding model with a Trust Domain-scoped enrollment mechanism based on standardized **device-level identity** and extensible bootstrap methods, aligning with **[#57](https://github.com/margo/product_management/issues/57)** and supporting late binding and pre-provisioned credentials (**[#62](https://github.com/margo/product_management/issues/62)**, **[#63](https://github.com/margo/product_management/issues/63)**).
- **[Parent Epic 12: Manage enrolled edge devices (#48)](https://github.com/margo/product_management/issues/48):**
  Provides the lifecycle primitives (enrollment, renewal, revocation, replacement, termination) required for large-scale device management and auditability.
- **[Parent Epic 7: Enroll an edge device with a device fleet manager (#43)](https://github.com/margo/product_management/issues/43):**
  Supplies the trust model for DFM onboarding and verification (**[#58](https://github.com/margo/product_management/issues/58)**, **[#64](https://github.com/margo/product_management/issues/64)**), enabling consistent authenticity verification across devices.

Together, these ensure that this SUP is not a standalone addition but the **identity foundation** for Margo's evolution - including **Device Fleet Management**, **gateway use cases**, and future profiles extending MIAF to additional Margo components such as WFM clients or telemetry agents.

## Technical proposal

### 1. Scope and Structure

This Specification Update Proposal defines the normative technical foundation for **non-human identity and authorization** in the Margo ecosystem.

It introduces the **Margo Identity and Authorization Framework (MIAF)** and applies it to the first concrete use case: the **Edge Compute Device Identity Profile**.

This SUP is therefore two-layered:

- **Framework (MIAF):** shared trust and identity concepts for any Margo component:

  - how a **Margo Identity Service (MIS)** issues, renews, and revokes identities;
  - how identities are expressed as verifiable credentials (**SVIDs**); and
  - how policies and **Trust Bundles** define the security boundary of a **Trust Domain**.

- **Edge Compute Device Identity Profile:** a specialization for Edge Compute Devices:

  - the **Logical Device Identity (LDI)**, its lifecycle, and its **X.509** representation;
  - handling of hardware-bound keys and **Bootstrap Credentials**; and
  - normative APIs for enrollment, renewal, and revocation of device identities.

The **normative core** of this SUP is based on cryptographically verifiable identities.
Authentication and authorization decisions are performed directly using these identities (for example, mTLS with an **X.509 SVID**).
Where mTLS is not feasible, a short-lived **JWT SVID** may be issued as a bearer credential. Optional mappings to OAuth 2.0 or enterprise token infrastructures are provided in an [informative appendix](#appendix-c-oauth2-and-api-gateway-interoperability-informative) and are **not required for compliance**.

A conceptual overview of how the **Margo Identity Service**, **Trust Domains**, and Margo components interact appears as an informative architecture diagram at the start of [Section 3](#3-margo-identity-and-authorization-framework-miaf).

#### Relationship to SPIFFE <!-- omit from toc -->

MIAF is intended to **reuse SPIFFE identity primitives** rather than invent Margo-specific credential formats or trust semantics.
In particular, this SUP:

- **adopts by reference** the SPIFFE concepts of **Trust Domain**, **SPIFFE ID**, **X.509-SVID**, **JWT-SVID**, and **Trust Bundle / Bundle Map**;
- **profiles or constrains** some of those standards where Margo needs device-specific rules; and
- **defines new Margo-specific behavior** for device bootstrap, lifecycle management, discovery, enrollment, renewal, revocation, and profile-specific authorization behavior.

This SUP does **not** adopt the **SPIFFE Workload API** or **SPIFFE Workload Endpoint** model. Those specifications define a local gRPC-based interface for workload identity delivery. The APIs defined here are **remote HTTPS lifecycle interfaces** designed for the device bootstrap and management problem.

Existing SPIFFE libraries and tooling can be used for SVID validation, Trust Bundle handling, and SPIFFE ID processing. A Margo-conformant deployment additionally requires the bootstrap methods, device identity lifecycle, discovery, and remote APIs defined in this SUP.

| Topic | Source | Notes |
| :---- | :----- | :---- |
| SPIFFE ID syntax and validation rules | SPIFFE, adopted by reference | This SUP defines only Margo path conventions where needed. |
| X.509-SVID baseline semantics | SPIFFE, adopted by reference + constrained | This SUP adds device-profile constraints. |
| JWT-SVID baseline semantics | SPIFFE, adopted by reference + constrained | This SUP defines device-profile usage and exchange behavior. |
| Trust Bundle / Bundle Map | SPIFFE, adopted by reference | This SUP defines discovery and retrieval conventions around it. |
| Discovery document | Margo | Not part of SPIFFE. |
| Enrollment / renewal / revocation / JWT exchange APIs | Margo | Remote HTTPS lifecycle interfaces, not the SPIFFE Workload API. |
| LDI / PDI / ESI model | Margo | Device-specific concepts introduced by this SUP. |
| Bootstrap methods | Margo + external standards | FDO, IEEE 802.1AR, and factory-certificate methods are integrated here. |

### 2. Terminology

The following terms define the common vocabulary for Margo's non-human identity and authorization model.
Some are adopted directly from open standards such as [**SPIFFE**](https://spiffe.io/); others are Margo-specific concepts introduced by this SUP.

This SUP concerns identities used by *non-human* **Margo components** - logical units of the Margo system such as the Device Fleet Manager (DFM), Workload Fleet Manager (WFM), their clients, and infrastructure services such as registries or observability collectors, as defined in the [Envisioned System Design](https://specification.margo.org/overview/envisioned-system-design/).

The **WFM Client** is called out specifically because this SUP draws a sharp distinction between *device identity* and *client identity*. A WFM Client runs on an Edge Compute Device, but its identity represents the deployed **client instance**, not the device itself. The **Logical Device Identity** defined here provides the stable, hardware-bound identity of the device; a planned **WFM Client Identity Profile** will define how WFM Clients obtain their own distinct identities, building on the device identity as their authentication foundation. This separation is necessary because device identity and WFM Client identity have different lifecycles, authorization scopes, and cardinalities across topologies (standalone devices, Kubernetes clusters, device gateways).

#### Terms adopted from SPIFFE <!-- omit from toc -->

The following terms are used as defined by SPIFFE. This SUP does not redefine them; it applies them within the Margo context.

##### Trust Domain <!-- omit from toc -->

The governed security boundary within which identities are issued and mutually recognized. This SUP uses **Trust Domain** in the SPIFFE sense: a trust-root-backed identity namespace and policy boundary. A Trust Domain defines:

- authoritative **trust anchors** (root and intermediate CAs);
- the namespace for **SPIFFE IDs**; and
- policies for identity lifecycle and authorization.

##### SPIFFE Verifiable Identity Document (SVID) <!-- omit from toc -->

The verifiable credential representing an identity within a Trust Domain. An SVID binds a SPIFFE ID to a key pair.
This SUP adopts SPIFFE **X.509-SVID** and **JWT-SVID** by reference and defines how Margo components use them. For the Edge Compute Device Identity Profile, LDIs are represented by **X.509 SVIDs**, while **JWT SVIDs** are optional derived credentials for short-lived bearer use in non-mTLS scenarios.

##### Policy-Based Authorization <!-- omit from toc -->

Authorization based on verified **SPIFFE IDs** and associated attributes, evaluated locally within the Trust Domain - not on external token scopes.

#### Terms introduced by this SUP <!-- omit from toc -->

The following terms are defined by this SUP. They represent Margo-specific concepts that build on the SPIFFE primitives above.

##### Margo Identity Service (MIS) <!-- omit from toc -->

A role that each Margo deployment must fill: the identity service within a Margo deployment that issues, renews, and revokes identities for components in a Trust Domain.
The MIS validates **Bootstrap Credentials**, enforces Trust Domain policy, and binds a component's physical or cryptographic root of trust to a stable identity within the Trust Domain. For this SUP, MIS issues **device identities** under the Edge Compute Device Identity Profile. Future SUPs may extend MIS to issue identities for other components such as WFM Clients or workloads.
The MIS is **not** a centrally provided Margo implementation; vendors, operators, or deployment tooling provide the actual service.

##### Logical Device Identity (LDI) <!-- omit from toc -->

The persistent, verifiable identity assigned to an **Edge Compute Device** within a Trust Domain. It is expressed as a SPIFFE URI, for example:

```text
spiffe://<trust-domain>/margo/device/<uuid-v4>
```

and represented by an **X.509 SVID**.
The LDI remains stable across hardware replacement or firmware updates when policy permits rebinding and serves as the anchor for device-level authentication and authorization.

##### Physical Device Identity (PDI) <!-- omit from toc -->

A hardware-rooted credential used during bootstrap, such as a factory X.509 certificate, a TPM-bound key, a **FIDO Device Onboard (FDO)** voucher, or an **IEEE 802.1AR DevID**. The MIS verifies the PDI and binds it to a **Logical Device Identity** during enrollment.

##### Bootstrap Credential <!-- omit from toc -->

A cryptographic credential presented by a Margo component to prove authenticity during initial enrollment with the MIS.
For devices, it conveys evidence of the **Physical Device Identity**. Each supported **Bootstrap Method** defines how this credential is formatted and verified.

##### Bootstrap Method <!-- omit from toc -->

A pluggable, normative method by which a Margo component presents its **Bootstrap Credential** to the MIS for enrollment.
This SUP defines methods for Edge Compute Devices, including **FDO**, **factory certificate** (via mTLS or JWT assertion), and **IEEE 802.1AR DevID**. Future SUPs may introduce methods for other Margo components.

##### Enrollment Subject Identifier (ESI) <!-- omit from toc -->

A deterministic, globally unique identifier derived by the MIS from the presented **Bootstrap Credential** during enrollment.
It is used to decide whether the presented bootstrap proof corresponds to an existing identity within the Trust Domain or a new one.

The derivation is **method-specific** and defined by each **Bootstrap Method**.
*Example (device profile):* from a verified PDI, the ESI may be the certificate fingerprint, or a hash derived from a device certificate contained in an FDO Ownership Voucher.
ESIs **MUST** be stable and unique within the Trust Domain and **MUST NOT** be reversible to the original credential material.

##### JWT SVID Exchange <!-- omit from toc -->

An API by which a holder of a valid **X.509 SVID** requests a short-lived **JWT SVID** for use behind TLS-terminating infrastructure. The request uses a **Client Authentication Assertion** signed with the SVID's private key.

### 3. Margo Identity and Authorization Framework (MIAF)

The **Margo Identity and Authorization Framework (MIAF)** defines how Margo components establish trust, authenticate, and are authorized within a **Trust Domain**.

MIAF prevents per-component identity silos by standardizing:

- how identities are **issued, renewed, and revoked** by the **Margo Identity Service (MIS)**;
- how identities are represented as **[SVIDs](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE.md)**; and
- how **policy-based authorization** is applied to verified **SPIFFE IDs**.

> **Architectural principle:** *Identity* ("who you are") and *authorization* ("what you can do") are distinct. MIAF defines the shared, verifiable identity layer; components apply local policy for authorization.

#### Framework overview

Conceptually, the **Margo Identity and Authorization Framework (MIAF)** consists of four main elements that together define how trust is established and maintained across the Margo ecosystem:

1. **Trust Domain**
   
   The logical and administrative boundary within which identities are issued and mutually recognized - for example, an end-user's factory network or a managed service operated by a fleet-management vendor.
   A Trust Domain defines:

   - the authoritative **trust anchors** (root and intermediate CAs);
   - the namespace for **SPIFFE IDs** (e.g., `spiffe://factory.example/...`);
   - and the **policies** governing identity lifecycle and authorization within that boundary.

   Each issued SPIFFE ID is scoped to exactly one Trust Domain; verifiers **MAY** validate identities from multiple Trust Domains via configuration and/or federation.

2. **Margo Identity Service (MIS)**
   
   The identity authority of a Trust Domain.
   MIS is responsible for:

   - validating **Bootstrap Credentials** presented by components during enrollment;
   - issuing, renewing, and revoking **SVIDs** for identities in the Trust Domain; and
   - maintaining the binding between **Physical Device Identities (PDIs)** and **Logical Device Identities (LDIs)** for devices covered by this SUP.

   MIS exposes a consistent set of APIs - **discovery**, **enrollment**, **renewal**, **revocation**, and **JWT SVID exchange** - that all identity profiles build upon.
   This API design allows other components (e.g., WFM Clients or telemetry agents) to reuse the same trust foundation in future SUPs.

3. **Margo components using MIAF**
   
   Components such as DFMs, WFMs, their clients, and telemetry agents act as:

   - **SVID holders**, presenting their SVIDs during mTLS authentication or when requesting a short-lived **JWT SVID**; and
   - **verifiers**, validating peer SVIDs using the Trust Domain's **Trust Bundle**, then applying **policy-based authorization** based on verified **SPIFFE IDs** and attributes.

4. **Trust Bundles**
   
   Each Trust Domain publishes a **Trust Bundle** containing:

   - root and intermediate certificates used to validate X.509 SVID chains; and
   - public keys used to verify JWT SVIDs (if used).

   Trust Bundles are identified by their Trust Domain name and distributed using the SPIFFE **Bundle Map mechanism**, as defined in the [SPIFFE Trust Domain and Bundle specification](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Trust_Domain_and_Bundle.md).
   Margo clients and servers use these bundles when validating SVIDs during mutual authentication.

A typical interaction sequence in MIAF (applicable to all Margo components, not just devices) is as follows. This sequence is **informative** and illustrates a typical flow; normative API details are in [Section 5](#5-apis).

1. **Discovery:** A Margo component locates MIS endpoints and Trust Bundle locations via the `.well-known/margo` discovery document defined in this SUP.
2. **Enrollment:** The component presents a **Bootstrap Credential** (per its Bootstrap Method) to the MIS and receives an **SVID** for its identity.
3. **Renewal:** Before expiry, the component renews its SVID via an authenticated request (for example, mTLS using the current SVID).
4. **Authentication to peers:** The component authenticates to other Margo components using mTLS with its **X.509 SVID**, or, where mTLS is not possible, using a short-lived **JWT SVID** obtained from the MIS.
5. **Authorization:** The receiving component validates the SVID (or JWT SVID) against the **Trust Bundle** and applies local **policy-based authorization** using the verified **SPIFFE ID** and associated attributes.

Specific details of these flows for **Edge Compute Devices** are defined later in the [Edge Compute Device Identity Profile](#4-edge-compute-device-identity-profile).

> **Conceptual Trust and Identity Architecture (Informative)**
> The diagram below illustrates MIAF in its most general form.
> Any Margo component - for example, a DFM Client, WFM Client, or OTEL Agent - enrolls with the **Margo Identity Service (MIS)** to obtain a verifiable identity, represented as either an **X.509 SVID** or a **JWT SVID** within a governed **Trust Domain**.
> Components authenticate to one another using these SVIDs:
>
> - X.509 SVIDs are typically used for mTLS-based authentication.
> - JWT SVIDs may be used for bearer-style authentication in environments with TLS-terminating proxies.
>
> The **Trust Domain** defines and distributes the **Trust Bundle** that all participants use to validate identities and enforce policy.
>
> ```mermaid
> flowchart LR
>  %% Figure 1: Conceptual Trust and Identity Architecture (Framework Level, Informative)
>
>  Client["**Margo Client Component**<br/>(e.g., DFM Client, WFM Client, Telemetry Agent)"]
>  Server["**Margo Server Component**<br/>(e.g., DFM, WFM, Observability Platform, Component Registry)"]
>  MIS["**Margo Identity Service (MIS)**<br/>Issues, renews, and revokes SVIDs"]
>  TD["**Trust Domain**<br/>Defines trust anchors, policies, and namespace"]
>  X509["**X.509 SVID**<br/>Certificate binding SPIFFE ID to key pair"]
>  JWT["**JWT SVID**<br/>Token binding SPIFFE ID to signed JWT claim set"]
>  TB["**Trust Bundle**<br/>Root and intermediate certificates,<br/>JWT verification keys"]
>
>  Client -->|"requests identity (X.509 SVID)"| MIS
>  MIS -->|"issues X.509 SVID"| X509
>  X509 -->|"certificate bound to locally generated private key"| Client
>  Client -->|"requests identity (JWT SVID)"| MIS
>  MIS -->|"issues JWT SVID"| JWT
>  Client -->|"authenticates using JWT SVID (bearer authentication)"| Server
>  Client -->|"authenticates using X.509 SVID (mTLS)"| Server
>  Server -->|"verifies SVID using Trust Bundle of"| TD
>  TD -->|"publishes"| TB
>
>  classDef comp fill:#e8f1ff,stroke:#5b8def,stroke-width:1px,rx:8px,ry:8px,color:#0b3b8c;
>  classDef ident fill:#e8f7ee,stroke:#2ca36b,stroke-width:1px,rx:8px,ry:8px,color:#0f5132;
>  classDef trust fill:#f7f7f7,stroke:#bdbdbd,stroke-width:1px,rx:8px,ry:8px,color:#333;
>
>  class Client,Server,MIS comp;
>  class X509,JWT ident;
>  class TD,TB trust;
> ```

#### Identity model

MIAF defines a unified identity model for all Margo components:

- **Identity representation:** An identity in a Trust Domain is named by a **SPIFFE ID** and represented by an **SVID** issued by the MIS.
  For devices, this SUP defines the **Logical Device Identity (LDI)** as the device-specific realization of that model.

- **Uniqueness and stability:** Each SPIFFE ID uniquely identifies one logical component within its Trust Domain.
  For devices, the **LDI** remains stable across hardware or firmware changes if policy permits rebinding.

- **Lifecycle:** All identities follow a consistent lifecycle:

  1. **Enrollment:** A new identity is created and an initial SVID is issued.
  2. **Active:** The SVID is valid and used for authentication.
  3. **Renewal:** The SVID is renewed before expiry.
  4. **Replacement:** The identity is rebound to new hardware or credentials (profile-specific).
  5. **Revocation:** The identity is invalidated due to compromise or policy.
  6. **Termination:** The identity is permanently retired.

  The **device-specific** realization of this lifecycle is defined later in [Logical Device Identity Lifecycle](#logical-device-identity-lifecycle).

- **Extensibility:**
  Although this SUP focuses on Edge Compute Devices, the same MIS, Trust Domain, SVID, and Trust Bundle concepts are intentionally generic.
  Future SUPs can therefore define profiles for other Margo components (such as WFM Clients or workloads) **without redefining** the underlying identity framework.

#### SVID profiles and negotiation

Within a Trust Domain, MIAF uses **[SPIFFE Verifiable Identity Documents (SVIDs)](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE.md)** to represent verifiable identities.
Each **SVID profile** defines how an identity is encoded, validated, and used for authentication (for example, as an **X.509 certificate** or a **JWT**).
Profiles are referenced by **profile URIs** in API exchanges.

This SUP uses two SPIFFE-defined SVID types:

- **X.509-SVID**, used for mTLS authentication; and
- **JWT-SVID**, used for bearer-style authentication.

> **Informative summary:**
> At the framework level, both are SVID-based representations of an identity. They differ in transport and validation semantics: X.509-SVIDs validate through certificate chains, while JWT-SVIDs validate through public keys distributed in the **Trust Bundle**.

For the **Edge Compute Device Identity Profile**, this SUP mandates:

- **X.509-SVIDs** as the only permitted representation for device enrollment and issuance of Logical Device Identities; and
- optional **JWT-SVIDs**, obtained only after enrollment via the **JWT SVID Exchange Endpoint**, for environments where end-to-end mTLS is not feasible.

**Profile identifiers:**

| Type        | Profile URI                                      | Status |
| :---------- | :----------------------------------------------- | :----- |
| `x509-svid` | `https://margo.org/profiles/spiffe/x509-svid/v1` | **Normative** ([adopts SPIFFE X.509-SVID by reference; constrained by this SUP](#x509-svid-profile)) |
| `jwt-svid`  | `https://margo.org/profiles/spiffe/jwt-svid/v1`  | **Normative** ([adopts SPIFFE JWT-SVID by reference; constrained by this SUP](#jwt-svid-profile)) |

A Trust Domain **MAY** support both types through the lifecycle APIs. Individual identity profiles define which types are permitted; for the Edge Compute Device Identity Profile, X.509-SVID is mandatory and JWT-SVID is optional and derived.
Future SUPs may introduce additional SVID profiles, but all **MUST** remain interoperable with the SPIFFE-based identity model used here.

##### X.509 SVID Profile

This SUP adopts the
[SPIFFE X.509-SVID specification](https://github.com/spiffe/spiffe/blob/main/standards/X509-SVID.md) by reference.
The following requirements are the **MIAF-specific constraints** that apply in addition to the base specification.

> **Informative summary:**
> An X.509-SVID carries exactly one SPIFFE ID in a URI SAN and is validated using the Trust Bundle for the SPIFFE ID's Trust Domain.

| Field | Requirement | Source | Notes |
| :---- | :---------- | :----- | :---- |
| **Subject Alternative Name (URI)** | Exactly one URI SAN containing the SPIFFE ID (e.g., `spiffe://<trust-domain>/margo/<component>/<uuid>`). | SPIFFE X.509-SVID | The SAN URI is the **only authoritative identity claim**; the Subject DN MUST be ignored. |
| **BasicConstraints** | `CA=false` for leaf SVIDs; `CA=true` permitted for intermediate issuers. | SPIFFE X.509-SVID | Issuer SVIDs SHOULD use `nameConstraints` to restrict SPIFFE ID namespaces. |
| **KeyUsage** | `digitalSignature` (**MUST**). `keyCertSign` and `cRLSign` **MUST NOT** be set. | SPIFFE X.509-SVID | Other usages such as `keyEncipherment` or `keyAgreement` **MAY** be set, per SPIFFE X.509-SVID. |
| **ExtendedKeyUsage** | If present, **MUST** follow the SPIFFE X.509-SVID specification. | SPIFFE X.509-SVID | MIAF does not introduce a different EKU rule at the framework level. |
| **Validity Period** | **RECOMMENDED:** <= 1 year | **MIAF** | SPIFFE does not define validity period constraints. Actual lifetime may be further constrained by the identity profile. |
| **NameConstraints** | **OPTIONAL** | SPIFFE X.509-SVID | May be used by MIS to limit valid SPIFFE ID namespaces. |

Validation (per SPIFFE X.509-SVID):

- Certificate chains **MUST** validate against the **Trust Bundle** of the Trust Domain.
- Each SPIFFE ID **MUST** be unique within its Trust Domain.

##### JWT SVID Profile

This SUP adopts the
[SPIFFE JWT-SVID specification](https://github.com/spiffe/spiffe/blob/main/standards/JWT-SVID.md) by reference.
The following requirements are the **MIAF-specific constraints** that apply in addition to the base specification.

> **Informative summary:**
> A JWT-SVID carries the SPIFFE ID in its `sub` claim and is validated using JWT signing keys from the Trust Bundle for the subject's Trust Domain.

| Field | Requirement | Source | Notes |
| :---- | :--------- | :----- | :----- |
| **`sub` (Subject Claim)** | MUST contain the SPIFFE ID. | SPIFFE JWT-SVID | Authoritative identity binding. |
| **`aud` (Audience Claim)** | MUST be present. | SPIFFE JWT-SVID | Specifies the intended verifier(s). |
| **Signature Algorithm** | MUST follow [Cryptographic Requirements](#cryptographic-requirements). | **MIAF** | SPIFFE allows a broader set of algorithms. |

Validation (per SPIFFE JWT-SVID):

- JWT-SVIDs are verified using **public keys** distributed via the **Trust Bundle**.
- Audience, expiry, and signature validation are mandatory.

#### Trust Bundles and Distribution

This SUP adopts the SPIFFE **Trust Domain and Bundle / Bundle Map** model by reference.
The following requirements describe how MIAF deployments publish, retrieve, and use Trust Bundles.

> **Informative summary:**
> A Trust Bundle contains the cryptographic material needed to validate SVIDs for exactly one Trust Domain. Trust Bundles from different Trust Domains must remain distinct and bound to the Trust Domain they represent.

Each Trust Domain maintains a **Trust Bundle**, the authoritative set of cryptographic material used to validate SVIDs within that domain.

A Trust Bundle **MUST** include:

- root and intermediate X.509 certificates for SVID chain validation; and
- public keys used to verify JWT SVID signatures.

Bundles:

- **SHOULD** be published and discovered via the SPIFFE [Trust Domain and Bundle Map](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Trust_Domain_and_Bundle.md);
- **MAY** be distributed through deployment tooling or provisioning flows;
- **MUST** be refreshed before expiry or rotation; and
- **SHOULD** be cached locally to support offline validation.

Validation process:

1. Determine the peer's Trust Domain from its SPIFFE ID.
2. Retrieve the corresponding Trust Bundle (via the [discovery document](#discovery-document-endpoint) or from cache).
3. Validate the SVID chain or JWT signature using that Trust Bundle.
4. If validation succeeds and local policy allows, apply **policy-based authorization**.

Cross-domain trust is configured explicitly by associating a Trust Domain with the correct Trust Bundle. The SPIFFE Bundle Map format supports inclusion of bundles for multiple Trust Domains, which can serve as a basis for cross-domain trust. Full federation lifecycle semantics, as defined by SPIFFE Federation, are out of scope for this SUP and may be addressed in future work.

#### Cryptographic Requirements

All cryptographic operations under MIAF - including key generation, CSR signing, SVID issuance, and verification - **MUST** conform to [RFC 9325](https://datatracker.ietf.org/doc/rfc9325/) and [NIST SP 800-131A Rev 2](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-131Ar2.pdf).

| Algorithm | Requirements |
| :-------- | :----------- |
| **ECDSA (P-256 + SHA-256)** | Keys **MUST** use curve P-256 (`prime256v1`) and signatures **MUST** use SHA-256 (`ES256`). |
| **RSA (PSS 3072 + SHA-256)** | Modulus **MUST** >= 3072 bits; signatures **MUST** use RSASSA-PSS with SHA-256 (`PS256`). `RS256` (PKCS#1 v1.5) **MUST NOT** be used. |

Additional rules:

- All components **MUST** implement at least one algorithm; the MIS and all verifiers **MUST** implement both.
- Hashes **MUST** be SHA-256 or stronger (SHA-384/512 permitted); MD5 and SHA-1 **MUST NOT** be used.
- These requirements apply to all **SVID** operations (X.509 and JWT).

#### MIS Deployment Modes (Informative)

The **Margo Identity Service (MIS)** may be deployed under various PKI topologies depending on policy or existing infrastructure.
These modes differ only in internal PKI hierarchy; external APIs and semantics remain identical.

| Mode | Description | Typical Use Case |
| :--- | :---------- | :--------------- |
| **Root CA Mode** | MIS operates as a self-signed **Root CA**, directly issuing all SVIDs. | Self-contained or air-gapped environments. |
| **Intermediate CA Mode** | MIS is an **Intermediate CA** signed by an enterprise or offline Root CA, chaining issued SVIDs to an enterprise trust root. | Enterprise environments aligned with corporate PKI. |
| **Registration Authority (RA) Mode** | MIS acts as a **Registration Authority**, validating enrollment and delegating issuance to a backend CA (e.g., EST, CMPv2, ACME). | Shared or external issuance infrastructures. |

> **Informative:**
> These modes are operationally interchangeable.
> Regardless of internal topology, every conformant MIS **MUST** expose the same APIs, lifecycle behaviors, and trust semantics defined in this specification.

### 4. Edge Compute Device Identity Profile

The **Edge Compute Device Identity Profile** applies the **Margo Identity and Authorization Framework (MIAF)** to **Edge Compute Devices**, defining how each device receives and maintains a verifiable **Logical Device Identity (LDI)** within a **Trust Domain**.

This profile specifies the normative representation of device identities (X.509 SVIDs), their lifecycle, SPIFFE ID structure, cryptographic and key-protection requirements, and the process for obtaining short-lived JWT SVIDs when mTLS cannot be used.

#### Profile Scope

This profile applies to all Edge Compute Devices that participate in a Margo deployment. It defines:

- the relationship between a device's **Physical Device Identity (PDI)** and its **Logical Device Identity (LDI)**;
- the lifecycle of that identity (enrollment, renewal, revocation, replacement, termination);
- the **X.509 SVID profile** constraints for representing LDIs;
- requirements for **hardware-bound key protection**; and
- the optional **JWT SVID exchange** mechanism for already-enrolled devices operating behind TLS-terminating infrastructure.

#### Logical Device Identity

A **Logical Device Identity** is a persistent, verifiable identity assigned to an Edge Compute Device within a Trust Domain. It provides a stable handle that can persist across hardware replacement or re-provisioning when permitted by operator policy.

Each LDI is identified by a SPIFFE-formatted URI and represented by an X.509 SVID called the **Device SVID**:

```text
spiffe://<trust-domain>/margo/device/<uuid-v4>
```

The `<uuid-v4>` component:

- **MUST** be a random RFC 4122 version 4 UUID generated by the MIS;
- **MUST** use lowercase hex with hyphens;
- **MUST NOT** be predictable or sequential; and
- **MUST** remain unchanged when the same LDI is legitimately rebound to new hardware under operator policy.

This identifier is unique within a Trust Domain and is the canonical reference for authenticating and authorizing the device across all Margo components.

#### Logical Device Identity Lifecycle

The LDI follows the **standard identity lifecycle** defined in [Identity model](#identity-model). The MIS applies device-specific policy and ensures consistent creation, renewal, replacement, and retirement within the Trust Domain.

| Lifecycle Phase | Description |
| :---- | :---------- |
| **Enrollment** | The device (or its operator) presents a **Bootstrap Credential** proving **PDI**. MIS validates it, derives an **Enrollment Subject Identifier (ESI)** per the method in use, and issues an initial **X.509 SVID** representing a new (or matched) LDI. |
| **Active** | The device uses its valid SVID to authenticate to Margo components within the Trust Domain. |
| **Renewal** | Before expiry, the device renews its SVID via an authenticated request (e.g., mTLS with the current SVID). Renewal semantics, including rate-limiting and backoff, are defined in [SVID Renewal Endpoint](#svid-renewal-endpoint). |
| **Replacement** | When hardware changes but the logical identity must persist, MIS binds the **new** PDI-derived ESI to the existing LDI and retires the previously active ESI, per operator policy. |
| **Revocation / Termination** | MIS invalidates the LDI when keys are compromised, the device is decommissioned, or policy mandates retirement. Once revoked/terminated, an LDI **MUST NOT** be re-issued. |

The MIS **MUST** maintain an authoritative mapping of ESI to LDI within the Trust Domain and **MUST NOT** allow duplicate or conflicting bindings.

> **Logical Device Identity Lifecycle (Informative)**
>
> ```mermaid
> flowchart TD
>   subgraph MIS["**Managed by MIS**"]
>     ENR["**Enrollment**<br/>Bind PDI to LDI (via ESI)"]
>     ACT["**Active**<br/>Valid X.509 SVID represents LDI"]
>     REN["**Renewal**<br/>Refresh SVID before expiry"]
>     REP["**Replacement**<br/>Rebind LDI to new hardware (PDI)"]
>     REV["**Revocation / Termination**<br/>Invalidate and retire LDI"]
>   end
>   ENR -->|SVID issued| ACT
>   ACT -->|Before expiry| REN
>   REN -->|SVID renewed| ACT
>   ACT -->|Hardware change| REP
>   REP -->|Rebinding complete| ACT
>   ACT -->|Compromise / Decommission / Retirement| REV
>   REV -->|Identity retired| END([End of Lifecycle])
>   classDef phase fill:#e8f7ee,stroke:#2ca36b,stroke-width:1px,rx:8px,ry:8px,color:#0f5132;
>   classDef terminal fill:#f7f7f7,stroke:#bdbdbd,stroke-width:1px,rx:8px,ry:8px,color:#333;
>   class ENR,ACT,REN,REP,REV phase;
>   class END terminal;
> ```

##### Device replacement: binding rules <!-- omit from toc -->

Device replacement has two parts: what the MIS is allowed to do (this section) and how that action is authorized (next section).

For devices, the MIS **MUST** enforce the following binding model between Enrollment Subject Identifiers (ESIs) and Logical Device Identities (LDIs):

1. **ESI immutability:** Once an ESI is bound to an LDI, the MIS **MUST NOT** bind that ESI to a different LDI.
2. **LDI replacement support:** An LDI **MAY** have multiple ESIs associated with it over time, but at most one may be **active** at any point.
3. **Single-active bootstrap subject:** An LDI **MUST NOT** have more than one **active** ESI at a time.
4. **Replacement semantics:** Replacement binds a **new** ESI to an **existing** LDI and retires the previously active ESI for that LDI.
5. **Retired ESI handling:** A retired ESI **MUST NOT** be sufficient to obtain new SVIDs for the LDI unless explicitly re-authorized by Trust Domain policy.

An ESI is **active** for an LDI if the MIS will accept enrollment requests based on that bootstrap subject for the purpose of issuing SVIDs for the LDI. An ESI is **retired** if it is no longer accepted for enrollment for that LDI.

If an ESI is not currently bound to an LDI, the MIS **MUST NOT** bind it to an existing LDI unless explicitly authorized by Trust Domain policy.

Replacement authorization is **policy-controlled** and **SHOULD** be auditable.

##### Device replacement: how it is authorized (informative) <!-- omit from toc -->

Replacement is a **policy-controlled** operation: a replacement device will typically present a **different** Physical Device Identity (and therefore a different ESI) than the prior device, but the operator may still require that the Logical Device Identity (LDI) remains stable.
To prevent unauthorized rebinding, MIS implementations will usually require explicit authorization before binding a **new** ESI to an **existing** LDI.

Common approaches include:

1. **Planned replacement: handover signed by the existing LDI**

   - While the existing device is still operational, it produces a one-time replacement authorization signed by its current LDI private key.
   - The authorization identifies the target LDI and binds the incoming device's bootstrap subject (for example, the incoming ESI or a stable, non-reversible identifier derived from its PDI).
   - MIS validates the signature and freshness (e.g., `exp`, one-time `jti`) and, if allowed by policy, performs the replacement binding.

2. **Operator-issued replacement ticket (fleet tooling / MIS admin)**

   - An operator initiates replacement for an existing LDI through deployment tooling.
   - The tooling issues a one-time ticket that authorizes binding the next eligible enrollment (identified by ESI or other enrollment metadata) to the specified LDI.

3. **Human-in-the-loop approval using enrollment metadata**

   - If the original device is unavailable, MIS (or fleet tooling) can present pending enrollments with non-secret metadata extracted from the bootstrap method (for example, manufacturer chain identity, model, voucher metadata, time, and the derived ESI).
   - An operator approves the binding of that pending enrollment to a selected existing LDI.

The exact approval workflow, token/ticket formats, and UI/automation are deployment-specific and may be standardized in a future revision.

This SUP does not yet standardize how such authorization evidence is conveyed to the MIS on the wire. Deployments may implement this out of band (for example, through MIS administration APIs) or through vendor extensions until a future revision profiles a common workflow.

#### Profile-specific Constraints on the X.509 SVID Profile

This profile **refines** the generic [X.509 SVID Profile](#x509-svid-profile) with additional certificate-level requirements for **device** identities. The MIS **MUST** issue device SVIDs as follows:

| Field | Requirement | Source | Notes |
| :---- | :---------- | :----- | :---- |
| **Subject Alternative Name (URI)**| Exactly one URI SAN containing `spiffe://<trust-domain>/margo/device/<uuid-v4>`. | **MIAF** | The Margo device path convention. The SAN is the authoritative device identity. |
| **Validity** | **MUST NOT** exceed **5 years**. **RECOMMENDED:** <= **90 days** for regularly online devices. | **MIAF** | SPIFFE does not constrain validity. Shorter lifetimes reduce risk; operators may choose longer for intermittently connected fleets. |

All other fields **MUST** comply with the base [X.509 SVID Profile](#x509-svid-profile).

The same LDI **MUST NOT** be active for multiple PDIs concurrently.

#### Profile-specific Enrollment and Identity Issuance

Device enrollment uses the generic API defined in [Section 5](#enrollment-and-identity-issuance-endpoint) with the following constraints:

- The only permitted `svid_profile_uri` for devices is `https://margo.org/profiles/spiffe/x509-svid/v1`. Attempts to enroll a device with `jwt-svid` **MUST** be rejected with `422` (`unsupported-svid-profile`).
- Device enrollment **MUST** use one of the device bootstrap methods defined in [Appendix A](#appendix-a-bootstrap-methods-normative).
- To ensure baseline interoperability, both the device and the MIS **MUST** implement the [Factory Certificate Method (mTLS)](#factory-certificate-method-mtls). Support for additional bootstrap methods is **OPTIONAL**.
- MIS **MUST** verify the presented bootstrap credential against Trust Domain policy and derive the **ESI** per the selected method before issuance.
- The enrollment request/response structure **MUST** conform to [Section 5](#enrollment-and-identity-issuance-endpoint).
- MIS **MUST** return `201 Created` when a new LDI is provisioned and `200 OK` for re-enrollments that match an existing LDI via the ESI.

#### Device Key Protection

> All device-identity cryptographic operations - key generation, CSR signing, SVID issuance - **MUST** comply with [Cryptographic Requirements](#cryptographic-requirements).

Private keys associated with device identities are critical assets and **MUST** be protected:

- Keys **MUST** be generated and stored in secure hardware (TPM, Secure Element, or TEE) where available and **MUST NOT** be exportable.
- Where only software storage is possible, implementations **MUST** provide at-rest encryption, integrity protection, and OS/process isolation (e.g., dedicated key service with strict ACLs).
- Keys **SHOULD** be regenerated upon re-enrollment or hardware replacement.
- Implementations **MAY** support attestation evidence of key provenance (e.g., TPM quotes or TEE reports) where platform capabilities exist. Attestation formats and verification semantics are out of scope for this SUP and **MAY** be defined in future specifications / SUPs.

This profile **refines and extends** the [Margo Device Requirements](https://specification.margo.org/specification/margo-devices/device-requirements/) by making hardware-backed key protection the expected norm for compliant devices.

#### JWT SVID Exchange (Optional)

Devices that already hold a valid X.509 SVID **MAY** obtain a short-lived **JWT SVID** for use behind TLS-terminating infrastructure via the **JWT SVID Exchange Endpoint** defined in [Section 5](#jwt-svid-exchange-endpoint). The following apply:

- The exchange **MUST** authenticate the device with **proof of possession** of the private key corresponding to the device's current **LDI**, using either:
  - **Mutual TLS** with the current X.509 SVID as the TLS client certificate, or
  - a **Client Authentication Assertion** JWT signed with the current LDI private key.
- JWT SVIDs **MUST** be short-lived and use algorithms permitted by [Cryptographic Requirements](#cryptographic-requirements).
- This mechanism **MUST NOT** be used for initial enrollment (see Appendix A's [**Bootstrap Assertion**](#factory-certificate-method-jwt-assertion) for the factory-key JWT method).

### 5. APIs

The Margo Identity and Authorization APIs define the network interfaces through which Margo components interact with the **Margo Identity Service (MIS)**.
This SUP normatively defines how these APIs apply to **Edge Compute Devices** via the Edge Compute Device Identity Profile, but the same endpoints and semantics are intended to be reusable by future profiles for other Margo components.

These APIs implement the operational behaviors described in previous sections - including identity issuance, renewal, revocation, and JWT SVID exchange - using RESTful patterns over HTTPS.
They are **Margo-specific lifecycle APIs**. They do **not** adopt the SPIFFE Workload API or SPIFFE Workload Endpoint specifications, which define a local gRPC-based interface for workload identity delivery.

All APIs in this section are **normative**.
They **MUST** use JSON for all request and response bodies unless otherwise specified, and **MUST** return errors in [RFC 9457](https://datatracker.ietf.org/doc/html/rfc9457) Problem Details format (see [Appendix B](#appendix-b-error-responses-normative) for details).

> **Note:**
> Integration with OAuth2 and API gateways, including mapping SVID-based authentication to OAuth access tokens and the use of an Authorization Server (AS), is defined **informatively** in [Appendix C](#appendix-c-oauth2-and-api-gateway-interoperability-informative) and is **not required for compliance** with this SUP.

#### Common URI and Encoding Rules

Some API endpoints (e.g., [SVID Renewal](#svid-renewal-endpoint) and [JWT SVID Exchange](#jwt-svid-exchange-endpoint)) include a `{spiffeIdEncoded}` placeholder in their URL paths.
This value **MUST** be computed as follows:

- Take the SPIFFE ID as a UTF-8 string (for example:
  `spiffe://northstar-ida.com/margo/device/123e4567-e89b-12d3-a456-426614174000`).
- Encode it using **Base64URL** as defined in [RFC 4648 § 5](https://datatracker.ietf.org/doc/html/rfc4648#section-5), omitting padding (`=`).
- Use this encoded value wherever `{spiffeIdEncoded}` appears in an endpoint path.

> **Example**
>
> ```text
> spiffe://northstar-ida.com/margo/device/123e4567-e89b-12d3-a456-426614174000
> becomes
> c3BpZmZlOi8vbm9ydGhzdGFyLWlkYS5jb20vbWFyZ28vZGV2aWNlLzEyM2U0NTY3LWU4OWItMTJkMy1hNDU2LTQyNjYxNDE3NDAwMA
> ```

All endpoints using `{spiffeIdEncoded}` **MUST** follow this same encoding rule.

> **Informative:**
> Base64URL encoding ensures the SPIFFE ID can safely appear within a URI path without further escaping and maintains consistent, deterministic mapping across all Margo APIs.

#### Discovery Document Endpoint

The discovery document serves as the **entry point** to a Trust Domain. Before any enrollment, renewal, or JWT SVID exchange operation, a client **MUST** retrieve this document to discover MIS location, supported bootstrap methods, and compatible SVID profiles. It provides the foundational metadata required to interact with all subsequent APIs defined in this specification.

This document is **Margo-specific metadata**. It advertises Margo endpoints and bootstrap capabilities and points clients to the standard SPIFFE Trust Bundle resource published for the Trust Domain.

| Item | Value |
| :--- | :---- |
| **Endpoint** | `GET /.well-known/margo` |
| **Authentication** | None |
| **Headers** | `Accept: application/json` |
| **Body schema (request)** | None |
| **Body schema (response)** | See below |
| **Responses** | `200 OK` - discovery document<br>`404 Not Found` - not available |
| **Errors** | RFC 9457 Problem Details as per [Appendix B](#appendix-b-error-responses-normative) |

> **Initial trust bootstrap (normative):**
> The discovery document is intentionally unauthenticated at the application layer, but clients **MUST** authenticate the HTTPS connection used to retrieve it.
> Specifically, clients **MUST** validate the MIS server identity for `GET /.well-known/margo` using an **initial trust mechanism** that exists prior to this protocol, as defined in [Initial Trust Bootstrap](#initial-trust-bootstrap).

**Response body schema (`200 OK`, `application/json`):**

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `trust_domain` | string | Y  | Identifier of the Trust Domain (e.g., `factory.example`). All SPIFFE IDs issued by the MIS **MUST** belong to this trust domain. |
| `trust_bundle_uri` | string | Y | Absolute HTTPS URL to the **SPIFFE Bundle Map** for this Trust Domain. The resource **MUST** conform to the [SPIFFE Trust Domain and Bundle Map specification](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Trust_Domain_and_Bundle.md#5-spiffe-bundle-map) and **MUST** contain the Trust Bundle for the domain identified by `trust_domain`. The resource **SHOULD** expose caching headers (`ETag`, `Last-Modified`). Clients **MUST** authenticate the HTTPS connection used to retrieve this resource per [Initial Trust Bootstrap](#initial-trust-bootstrap). |
| `margo_identity_service_base_uri` | string | Y | Absolute HTTPS base URL of the Margo Identity Service (MIS). All MIS endpoints defined in this section are derived from this base URI. Clients **MUST** authenticate the HTTPS connection to this host per [Initial Trust Bootstrap](#initial-trust-bootstrap). |
| `supported_bootstrap_methods` | array of string | Y | URNs of supported bootstrap methods. Each URN **MUST** reference a method defined in [Appendix A](#appendix-a-bootstrap-methods-normative) or a registered vendor extension (`urn:margo:bootstrap:<method>:<version>`). Custom methods **SHOULD** use an organization-scoped namespace (e.g., `urn:margo:bootstrap:acme-factory:v1`). Servers **MUST NOT** advertise a method without a corresponding verification configuration in MIS. |
| `svid_profiles_supported` | array of object | Y | List of supported SVID profile descriptors, used by clients to negotiate compatible identity formats. |
| `svid_profiles_supported.type` | string | Y | Profile type identifier (e.g., `x509-svid`, `jwt-svid`). |
| `svid_profiles_supported.versions` | array of string | Y | Absolute URIs of supported profile versions for the given type. Clients **MUST** select one from this list when enrolling. |
| `recommended_svid_profile_uri` | string | Y | URI of the SVID profile version recommended by the MIS. Clients **SHOULD** prefer this profile when compatible. |

> **Informative:**
> The discovery document is the authoritative entry point for all identity operations. Clients **MUST** query this endpoint before enrollment or renewal to determine the MIS base URI, supported profiles, and authentication methods. This design allows Margo deployments to evolve profiles and bootstrap mechanisms without breaking existing clients, while keeping endpoint locations predictable.
> Clients **SHOULD** honor `ETag`/`Last-Modified` when polling the discovery document to minimize load.

##### Example: Discovery Document <!-- omit from toc -->

**Example request:**

```http
GET /.well-known/margo
Accept: application/json
```

**Example response (`200 OK`):**

```jsonc
{
  "trust_domain": "northstar-ida.com",
  "trust_bundle_uri": "https://mis.northstar-ida.com/.well-known/spiffe/bundle.json",
  "margo_identity_service_base_uri": "https://mis.northstar-ida.com",
  "supported_bootstrap_methods": [
    "urn:margo:bootstrap:factory-cert-jwt:v1",
    "urn:margo:bootstrap:fdo:v1"
  ],
  "svid_profiles_supported": [
    {
      "type": "x509-svid",
      "versions": ["https://margo.org/profiles/spiffe/x509-svid/v1"]
    },
    {
      "type": "jwt-svid",
      "versions": ["https://margo.org/profiles/spiffe/jwt-svid/v1"]
    }
  ],
  "recommended_svid_profile_uri": "https://margo.org/profiles/spiffe/x509-svid/v1"
}
```

##### SVID Profile Negotiation <!-- omit from toc -->

Before enrollment, a client **MUST** inspect the `svid_profiles_supported` array in the discovery document to determine a compatible profile:

1. Iterate each entry in `svid_profiles_supported`.
2. If the entry's `type` (e.g., `x509-svid`, `jwt-svid`) is supported by the client, iterate its `versions` array and select one or more profile URIs the client recognizes.
3. If multiple compatible URIs exist, the client **SHOULD** prefer the `recommended_svid_profile_uri` when supported.

If no compatible profile URI is found, the client **MUST NOT** attempt enrollment.
During enrollment, the client **MUST** include the selected `svid_profile_uri` in the request body.

> **Rationale (informative):**
> This negotiation mechanism ensures forward compatibility between clients and servers. It allows both sides to adopt new SVID profile versions or types without protocol changes, preserving interoperability across Margo deployments.

#### Trust Bundle Retrieval Endpoint

The Trust Bundle endpoint provides the authoritative set of public trust anchors for a Trust Domain.
Its location is given by the `trust_bundle_uri` field in the [Discovery Document](#discovery-document-endpoint).

| Item | Value |
| :--- | :---- |
| **Endpoint** | `<trust_bundle_uri>` (for example: `https://mis.example.com/.well-known/spiffe/bundle.json`) |
| **Authentication** | None (public resource, HTTPS required) |
| **Media type** | `application/json` |
| **Body schema (response)** | The response **MUST** conform to the [SPIFFE Bundle Map format](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Trust_Domain_and_Bundle.md#5-spiffe-bundle-map). |
| **Responses** | `200 OK` - bundle retrieved<br>`304 Not Modified` - cached copy still valid<br>`404 Not Found` - bundle unavailable |
| **Caching** | The endpoint **SHOULD** support HTTP caching headers (`ETag`, `Last-Modified`). |

> **Informative:**
> Clients **MUST** retrieve and validate this bundle before validating any SVIDs issued within the Trust Domain.
> The HTTPS connection used to retrieve the Trust Bundle **MUST** be authenticated using an initial trust mechanism as defined in [Initial Trust Bootstrap](#initial-trust-bootstrap).
> The SPIFFE Bundle Map format supports inclusion of bundles for multiple Trust Domains, which can serve as a basis for cross-domain trust. Full federation lifecycle semantics, as defined by SPIFFE Federation, are out of scope for this SUP.

#### Enrollment and Identity Issuance Endpoint

This endpoint is used by a Margo component (for this SUP: an Edge Compute Device) or an authorized bootstrap intermediary acting on its behalf (for example, an FDO Owner Onboarding Service component of the MIS) to perform **initial enrollment** with the Margo Identity Service (MIS).

During enrollment, the component authenticates using its **Bootstrap Credential** and requests issuance of a new identity, represented by an SVID.
For Edge Compute Devices, this operation establishes the authoritative binding between the device's **Physical Device Identity** and **Logical Device Identity** within the Trust Domain.

| Item | Value |
| :--- | :---- |
| **Endpoint** | `POST /api/v1/identities` |
| **Authentication** | Defined by the selected [bootstrap method](#appendix-a-bootstrap-methods-normative) (e.g., mTLS or JWT assertion) |
| **Headers** | `Content-Type: application/json` |
| **Body schema (request)** | See below |
| **Body schema (response)** | See below |
| **Responses** | `201 Created` (initial enrollment)<br>`200 OK` (re-enrollment)<br>`401`, `422`, `429` - per RFC 9457 |
| **Errors** | RFC 9457 Problem Details as per [Appendix B](#appendix-b-error-responses-normative) |

**Request body schema (`application/json`):**

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `svid_profile_uri` | string | Y | Absolute URI identifying the SVID profile requested. **MUST** match one of the URIs listed in `svid_profiles_supported` from the [discovery document](#discovery-document-endpoint). |
| `svid_request` | object | Y | Profile-specific payload containing parameters required to issue an SVID. See [Profile-specific `svid_request` formats (request payload)](#profile-specific-svid_request-formats-request-payload) below. |
| `bootstrapCredential` | object | Y | Credential and associated proof used to authenticate the component during enrollment. See [Bootstrap Methods](#appendix-a-bootstrap-methods-normative) for normative method definitions. |
| `bootstrapCredential.method` | string | Y | URN uniquely identifying the bootstrap method (e.g., `urn:margo:bootstrap:factory-cert-jwt:v1`). |
| `bootstrapCredential.proof`  | object | N | Method-specific proof of possession (e.g., a signed JWT assertion or an mTLS client certificate chain). Present only if the bootstrap method requires explicit proof material. |

**Response body schema (`201 Created` or `200 OK`, `application/json`):**

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `svid_profile_uri` | string | Y | URI of the SVID profile used for issuance. Identifies the structure and semantics of the `svid` object returned. |
| `svid` | object | Y | Profile-specific payload containing the issued SVID. See [Profile-specific `svid` formats (response payload)](#profile-specific-svid-formats-response-payload) below. |

> **Informative:**
> The MIS returns `201 Created` when it creates a new identity record and `200 OK` when it issues a new SVID for an existing identity as part of a re-enrollment or recovery flow.
> Identity-profile-specific interpretations (for example, mapping Physical to Logical Device Identity for devices) are defined in the corresponding profile section.

##### Profile-specific `svid_request` formats (request payload) <!-- omit from toc -->

###### X.509 SVID profile (`https://margo.org/profiles/spiffe/x509-svid/v1`) <!-- omit from toc -->

For `svid_profile_uri = "https://margo.org/profiles/spiffe/x509-svid/v1"`, the `svid_request` object **MUST** conform to the following structure:

```json
{
  "csr": "<base64 DER PKCS#10>"
}
```

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `csr` | string | Y | Base64-encoded (standard alphabet, no newlines) representation of a DER-encoded PKCS#10 CSR. The CSR public key **MUST** comply with [Cryptographic Requirements](#cryptographic-requirements). |

**Validation (normative):**

- The MIS **MUST** ignore any Subject DN and SANs in the CSR and set the authoritative SPIFFE ID in the URI SAN of the issued certificate according to the identity profile in effect (for devices, the Logical Device Identity format). However, the MIS **MAY** enforce structural requirements (e.g., requiring a Common Name) if backed by a strict PKI.
- Inputs containing PEM armor or malformed Base64 **MUST** be rejected with `400 Bad Request`.
- CSRs using unsupported key types or signature algorithms **MUST** be rejected per [Cryptographic Requirements](#cryptographic-requirements) with `422 Unprocessable Entity` (`unsupported-svid-profile`) or another profile-appropriate error type (see [Appendix B](#appendix-b-error-responses-normative)).

> **Informative example**
> To generate `svid_request.csr` (be sure to remove the PEM armor and use DER + Base64 only):
>
> ```bash
> openssl req -new -key device_key.pem -outform DER -subj "/CN=my-margo-device" | base64 -w0
> ```

###### JWT SVID profile (`https://margo.org/profiles/spiffe/jwt-svid/v1`) <!-- omit from toc -->

For `svid_profile_uri = "https://margo.org/profiles/spiffe/jwt-svid/v1"`, the `svid_request` object **MUST** conform to the following minimal structure:

```json
{
  "aud": ["<audience-1>", "<audience-2>"],
  "ttl": 300
}
```

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `aud` | array of string | Y | One or more audience identifiers to be placed into the JWT SVID `aud` claim. Each item **MUST** be a non-empty string. |
| `ttl` | integer (seconds) | N | Requested lifetime in seconds for the JWT SVID. The MIS **MAY** cap or override this value according to policy. |

**Validation (normative):**

- If `ttl` is omitted, the MIS **MUST** use a default configured lifetime.
- JWT SVID lifetime limits are defined by the applicable endpoint (see [JWT SVID Exchange Endpoint](#jwt-svid-exchange-endpoint)).

> **Important:**
> The **Edge Compute Device Identity Profile** defined in this SUP **MUST NOT** use the JWT SVID profile for enrollment. Devices **MUST** request X.509 SVIDs only. The JWT SVID profile is defined here for framework completeness and for future identity profiles.

##### Profile-specific `svid` formats (response payload) <!-- omit from toc -->

###### X.509 SVID profile (`https://margo.org/profiles/spiffe/x509-svid/v1`) <!-- omit from toc -->

For `svid_profile_uri = "https://margo.org/profiles/spiffe/x509-svid/v1"`, the `svid` object in the response **MUST** conform to the following structure:

```json
{
  "certificate_chain_pem": ["<leaf>", "<intermediate-1>", "..."]
}
```

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `certificate_chain_pem` | array of string | Y | PEM-encoded X.509 certificate chain. The first element **MUST** be the SVID (leaf certificate representing the issued identity), followed by any required intermediates. The root **MAY** be omitted if distributed via the Trust Bundle. PEM strings **MUST** be base64 with line breaks; clients **MUST NOT** assume a specific wrap width. |

###### JWT SVID profile (`https://margo.org/profiles/spiffe/jwt-svid/v1`) <!-- omit from toc -->

For `svid_profile_uri = "https://margo.org/profiles/spiffe/jwt-svid/v1"`, the `svid` object **MUST** conform to:

```json
{
  "jwt": "<compact-jwt-svid>",
  "expires_at": "2025-10-25T14:12:31Z"
}
```

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `jwt` | string | Y | The compact JWT SVID string, as defined by the [JWT SVID Profile](#jwt-svid-profile). Its `sub` claim **MUST** contain the SPIFFE ID of the issued identity. |
| `expires_at` | string (ISO 8601) | N | UTC timestamp indicating when the JWT SVID expires. If omitted, clients **MUST** derive expiry from the token's `exp` claim. |

> **Usage of JWT SVIDs (normative):**
> When used with HTTP APIs defined by this SUP, a JWT SVID **MUST** be presented to the server using the `Authorization: Bearer <jwt-svid>` header.

##### Example: Enrollment and Identity Issuance <!-- omit from toc -->

**Example request (device with X.509 profile):**

```http
POST /api/v1/identities
Content-Type: application/json
```

```jsonc
{
  "svid_profile_uri": "https://margo.org/profiles/spiffe/x509-svid/v1",
  "svid_request": {
    "csr": "MIICVzCCAT8CAQAwEjEQMA4GA1UEAwwHbWFyZ28tZGUwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATKxRZ8YtMUVcgG9l7oY7OqDyy0kchPr0ET6lm3MKbkT2vSzr6X0Spbz4cPmgqK4pYpFV4lLhl9pKUx3Cdd5L0YoycwJQYJKoZIhvcNAQkOMRYwFDASBgNVHRETCzAJggdtYXJnby1kZTAKBggqhkjOPQQDAgNHADBEAiB5VsvzqBhw+L4i6V60oU5gN1jKMmGfdyR2PqQ8q5RdjQIgQdBBQLehRzCwH8ApVfP1PZAfV1qTLp1vR7m1LcwTnXs="
  },
  "bootstrapCredential": {
    "method": "urn:margo:bootstrap:factory-cert-jwt:v1",
    "proof": {
      "assertion": "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9..."
    }
  }
}
```

**Example response (`201 Created`):**

```jsonc
{
  "svid_profile_uri": "https://margo.org/profiles/spiffe/x509-svid/v1",
  "svid": {
    "certificate_chain_pem": [
      "-----BEGIN CERTIFICATE-----\nMIIC4TCCAcigAwIBAgIUFsO2...\n-----END CERTIFICATE-----",
      "-----BEGIN CERTIFICATE-----\nMIIDdTCCAl2gAwIBAgIURv7O...\n-----END CERTIFICATE-----"
    ]
  }
}
```

##### MIS Validation and Processing Logic <!-- omit from toc -->

Upon receiving an enrollment request, the Margo Identity Service (MIS) **MUST** perform the following sequence of validation and issuance steps.
This logic ensures consistent handling of first-time enrollments, retried network submissions, and re-enrollments or recoveries across all Margo components.

1. **Derive Enrollment Subject Identifier**

   The MIS **MUST** derive a deterministic **[Enrollment Subject Identifier](#enrollment-subject-identifier-esi)** from the presented `bootstrapCredential`.
   This identifier anchors the binding between the presented bootstrap material and the resulting identity (for devices: between the Physical Device Identity and the Logical Device Identity).

2. **Validate bootstrap proof**

   The MIS **MUST** verify the cryptographic proof included in the `bootstrapCredential` according to the verification rules defined by the selected bootstrap `method`.

   - If proof validation fails, the MIS **MUST** reject the request with `401 Unauthorized` using a Problem Details object.
   - Validation includes method-specific checks such as certificate chain verification (for mTLS), signature verification (for JWT-based methods), and temporal validity checks (`iat`, `exp`).

3. **Validate requested profile**

   The MIS **MUST** verify that the `svid_profile_uri` appears within its `svid_profiles_supported.versions` list as published in the discovery document.
   Validation semantics, including the structure and verification rules for `svid_request`, are defined by the selected SVID profile.

   - If unsupported, the MIS **MUST** return `422 Unprocessable Entity` with an `unsupported-svid-profile` error type (see [Appendix B](#appendix-b-error-responses-normative)).
   - If the provided `svid_request` fails profile-specific validation (for example, malformed CSR under the X.509 profile), the MIS **MUST** return `400 Bad Request`.

4. **Check for existing identity binding**

   - **Case A - No binding exists (initial enrollment)**

      - The MIS applies operator-defined Trust Domain policy to determine whether new identities may be created.
      - Upon approval, the MIS **MUST** create a new identity (for devices: a UUIDv4 Logical Device Identity) and persist a mapping between the enrollment subject identifier and that identity.
      - The MIS then issues an SVID according to the selected `svid_profile_uri` and returns `201 Created` with the profile-conformant response body.

   - **Case B - Binding exists (re-enrollment / recovery)**

      - The MIS **MUST** retrieve the existing identity bound to the enrollment subject identifier.
      - If the CSR contains a **new** public key, the MIS **MUST** apply operator policy to decide if **key rotation** (same identity, new key) is permitted. If not permitted, return `409 Conflict`. If permitted, issue a new SVID and invalidate the prior SVID.
      - The MIS then issues a new SVID for the same identity and returns `200 OK`.

   - **Case C - Replacement / rebinding to an existing identity (policy-controlled)**

      - If the presented enrollment subject identifier is not currently bound to an identity but Trust Domain policy explicitly authorizes binding it to an existing identity (for example, as part of a device replacement workflow), the MIS **MAY** bind the ESI to the authorized existing identity and issue an SVID for that identity.
      - Replacement authorization evidence and approval workflows are policy-controlled (see [Device replacement: how it is authorized](#device-replacement-how-it-is-authorized-informative)).

5. **Finalize and audit**

   The MIS **SHOULD** record enrollment metadata (bootstrap method, time, and trust anchor) for auditability and traceability.

> **Informative:**
> This deterministic workflow ensures idempotent enrollment behavior across retries and consistent lifecycle semantics between new and returning Margo components.
> The Edge Compute Device Identity Profile specializes this generic behavior by defining how the enrollment subject identifier is derived from PDIs and how it is bound to LDIs.

#### SVID Renewal Endpoint

This endpoint allows an already enrolled component to **renew its expiring SVID** while preserving its existing identity.

Renewal is authenticated **directly with an existing SVID**:
the client either presents its current X.509 SVID as a TLS client certificate (mTLS), or presents a JWT SVID as an HTTP Bearer token, and the MIS issues a new SVID for the same SPIFFE ID.

| Item | Value |
| :--- | :---- |
| **Endpoint** | `POST /api/v1/identities/{spiffeIdEncoded}/renewals` |
| **Authentication**         | **Either:**<br>- **Mutual TLS:** The client **MUST** present its current X.509 SVID as the TLS client certificate. The MIS **MUST** extract the SPIFFE ID from the URI SAN and verify that it matches `{spiffeIdEncoded}`.<br>- **JWT SVID (Bearer):** The client **MUST** present a valid JWT SVID using `Authorization: Bearer <jwt-svid>`. The MIS **MUST** validate the JWT SVID according to the [JWT SVID Profile](#jwt-svid-profile), and verify that the `sub` claim's SPIFFE ID matches `{spiffeIdEncoded}`.<br>`{spiffeIdEncoded}` **MUST** be computed as defined in the [Common URI and Encoding Rules](#common-uri-and-encoding-rules). |
| **Headers** | `Content-Type: application/json` |
| **Body schema (request)**  | See below |
| **Body schema (response)** | Same as [Enrollment response](#enrollment-and-identity-issuance-endpoint) |
| **Responses**              | `200 OK` on success<br>`400`, `401`, `422`, `429` - RFC 9457 errors |
| **Errors**                 | RFC 9457 Problem Details as per [Appendix B](#appendix-b-error-responses-normative) |

**Request body schema (`application/json`):**

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `svid_profile_uri` | string | Y | Absolute URI of the SVID profile used for renewal. **MUST** match a profile supported by MIS and **SHOULD** match the currently active profile unless explicitly allowed by policy. |
| `svid_request` | object | Y | Profile-specific renewal payload. For X.509 SVID, this object contains a Base64-encoded CSR as defined in [X.509 SVID Profile](#x509-svid-profile). For JWT SVIDs, it contains an `aud`/`ttl` object as defined in [JWT SVID Profile](#jwt-svid-profile). |

> **Note:**
> JWT SVID renewal behavior depends on the applicable **identity profile under MIAF**:
>
> - For the **Edge Compute Device Identity Profile** defined in this SUP, JWT SVIDs are **derived credentials** obtained from an X.509 SVID. They are short-lived and **MUST NOT** be renewed via this endpoint. Devices requiring a fresh JWT SVID **MUST** use the [JWT SVID Exchange Endpoint](#jwt-svid-exchange-endpoint).
> - For other (future) identity profiles that directly issue JWT SVIDs through `/identities`, renewal semantics **MAY** be defined in those profiles.
>
> When renewing an X.509 SVID, clients **MAY** rotate keys by submitting a CSR for a new key pair; acceptance is **policy-controlled** (see [MIS Validation and Processing Logic](#mis-validation-and-processing-logic)).

##### Example: SVID Renewal <!-- omit from toc -->

**Example request (device renewing X.509 SVID over mTLS):**

```http
POST /api/v1/identities/c3BpZmZlOi8vbm9ydGhzdGFyLWlkYS5jb20vbWFyZ28vZGV2aWNlLzEyM2U0NTY3LWU4OWItMTJkMy1hNDU2LTQyNjYxNDE3NDAwMA/renewals
Content-Type: application/json
# TLS 1.3, client certificate = current device X.509 SVID
```

```jsonc
{
  "svid_profile_uri": "https://margo.org/profiles/spiffe/x509-svid/v1",
  "svid_request": {
    "csr": "MIICVjCCAT8CAQAwEjEQMA4GA1UEAwwHbWFyZ28tZGUwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATKxRZ8YtMUVcgG9l7oY7OqDyy0kchPr0ET6lm3MKbkT2vSzr6X0Spbz4cPmgqK4pYpFV4lLhl9pKUx3Cdd5L0YoycwJQYJKoZIhvcNAQkOMRYwFDASBgNVHRETCzAJggdtYXJnby1kZTAKBggqhkjOPQQDAgNHADBEAiB5VsvzqBhw+L4i6V60oU5gN1jKMmGfdyR2PqQ8q5RdjQIgQdBBQLehRzCwH8ApVfP1PZAfV1qTLp1vR7m1LcwTnXs="
  }
}
```

**Example response (`200 OK`):**

```jsonc
{
  "svid_profile_uri": "https://margo.org/profiles/spiffe/x509-svid/v1",
  "svid": {
    "certificate_chain_pem": [
      "-----BEGIN CERTIFICATE-----\nMIIC4TCCAcigAwIBAgIUFsO2...\n-----END CERTIFICATE-----",
      "-----BEGIN CERTIFICATE-----\nMIIDdTCCAl2gAwIBAgIURv7O...\n-----END CERTIFICATE-----"
    ]
  }
}
```

> **Informative:**
> The MIS verifies that the SPIFFE ID encoded in the path matches either the URI SAN of the client certificate (mTLS) or the `sub` claim of the presented JWT SVID (Bearer). If they match and the `svid_request` is valid, a new SVID is issued for the same identity. The client **MUST** replace its previous SVID and **SHOULD** preserve or rotate its key according to policy.

> **Note:**
> JWT SVID (Bearer) authentication is supported for the **renewal** endpoint. The JWT SVID **exchange** endpoint explicitly does **not** accept JWT Bearer authentication.

##### MIS Renewal Rate-Limiting and Backoff Policy <!-- omit from toc -->

To prevent resource exhaustion, credential churn, or abuse through automated replay, the MIS **MUST** apply rate-limiting controls to all renewal operations.

1. **Renewal frequency control**

   - The MIS **MUST** track renewal frequency per SPIFFE ID.
   - A **RECOMMENDED** baseline policy is no more than 5 successful renewals per 24-hour period per identity, **configurable by deployment**.

2. **Error handling**

    - When limits are exceeded, the MIS **MUST** return `429 Too Many Requests` using the RFC 9457 Problem Details format.
    - The MIS **MUST** include a `Retry-After` response header (delta-seconds) indicating when the client may retry.

3. **Client behavior**

    - Clients **MUST NOT** automatically retry failed renewals before the `Retry-After` duration has elapsed.
    - Clients **SHOULD** apply exponential backoff to avoid synchronized retry storms.

#### JWT SVID Exchange Endpoint

This endpoint allows a component that already holds a valid X.509 SVID to request a **short-lived JWT SVID** representing the same identity.

It is intended for environments where end-to-end mTLS is not feasible (for example, in the presence of TLS-terminating proxies), while still using the MIS and Trust Domain as the source of truth for identities.

| Item | Value |
| :--- | :---- |
| **Endpoint** | `POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid` |
| **Authentication** | The client **MUST** authenticate using one of the following mechanisms:<br>- **Mutual TLS** - Present its current X.509 SVID as the TLS client certificate. MIS **MUST** verify that the SPIFFE ID in the URI SAN matches `{spiffeIdEncoded}`.<br>- **Client Assertion JWT** - Include a `client_assertion` JWT in the request body (see below), signed with the private key corresponding to the current X.509 SVID. MIS **MUST** validate the signature and verify that the `sub` claim matches `{spiffeIdEncoded}`.<br>For this endpoint, MIS **MUST NOT** accept authentication using a JWT SVID in `Authorization: Bearer ...`.<br>`{spiffeIdEncoded}` **MUST** be computed as defined in the [Common URI and Encoding Rules](#common-uri-and-encoding-rules). |
| **Headers** | `Content-Type: application/json` |
| **Body schema (request)** | See below |
| **Body schema (response)** | See below |
| **Responses** | `201 Created` on success<br>`400`, `401`, `422`, `429` - RFC 9457 errors |
| **Errors** | RFC 9457 Problem Details as per [Appendix B](#appendix-b-error-responses-normative) |

**Client Assertion JWT (Normative Definition)**
A `client_assertion` JWT used for this endpoint **MUST** conform to the following claims and constraints:

| Claim | Requirement |
| :---- | :---------- |
| `iss`, `sub` | **MUST** be identical and equal to the client's SPIFFE ID. |
| `aud` | **MUST** equal the exact URL of the JWT SVID exchange endpoint. |
| `exp` | **MUST NOT** exceed five (5) minutes after issuance. |
| `jti` | **MUST** be unique for each assertion. |

The JWT **MUST** be digitally signed using the private key associated with the client's X.509 SVID, and the MIS **MUST** validate the signature chain against the Trust Bundle for the client's Trust Domain. The JWS `alg` **MUST** comply with [Cryptographic Requirements](#cryptographic-requirements) and **MUST** match the key type in the current X.509 SVID.

> **Warning:** Do not confuse this **Client Authentication Assertion** with the **Bootstrap Assertion** used in the `factory-cert-jwt` bootstrap method:
>
> - **Bootstrap Assertion:** Signed by the **factory private key** (PDI). Used only once during initial enrollment.
> - **Client Authentication Assertion:** Signed by the active **identity private key** (LDI). Used repeatedly to exchange an existing X.509 SVID for a fresh JWT SVID.

**Request body schema (`application/json`):**

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `aud` | array of string | Y | Audience identifiers to include in the JWT SVID `aud` claim. |
| `ttl` | integer (seconds) | N | Requested lifetime in seconds (capped by MIS policy). |
| `client_assertion` | string | N | Optional JWT for client authentication when mTLS is not used. If present, it **MUST** meet the requirements defined above. |

> **Note:**  
> The `client_assertion` JWT used in this endpoint is a **Client Authentication Assertion** signed with the private key corresponding to an existing X.509 SVID.  
> It is distinct from the **Bootstrap Assertion** defined in the `factory-cert-jwt` bootstrap method (see [Appendix A](#factory-certificate-method-jwt-assertion)), which is signed with the device's factory key during initial enrollment.

**Response body schema (`201 Created`, `application/json`):**

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `jwt` | string | Y | The compact JWT SVID string, as defined by the [JWT SVID Profile](#jwt-svid-profile). Its `sub` claim **MUST** equal the SPIFFE ID identified by `{spiffeIdEncoded}`. |
| `expires_at` | string (ISO 8601) | N | UTC timestamp when the JWT SVID expires. If omitted, clients **MUST** derive expiry from the token's `exp` claim. |

**Validation (normative):**

- If `client_assertion` is used, its signature chain **MUST** validate back to the Trust Domain's **Trust Bundle**.
- The MIS **MUST** ensure the JWT SVID's `sub` claim equals the SPIFFE ID encoded in `{spiffeIdEncoded}`.
- The MIS **MUST** include the requested audiences (possibly filtered or restricted by policy) in the `aud` claim.
- JWT SVID lifetime **MUST** comply with the [JWT SVID Profile](#jwt-svid-profile) and **MUST** respect [Cryptographic Requirements](#cryptographic-requirements) for signature algorithms.
- The MIS **SHOULD** limit the issued JWT SVID's lifetime to **no more than one hour** by default, unless a shorter or longer duration is explicitly authorized by Trust-Domain policy.

> **Relationship to MIAF (informative):**
> This endpoint is a *profile-specific realization* of the JWT SVID Profile for identities that already hold an X.509 SVID. It allows a long-lived, hardware-bound X.509 SVID (for example, a device Logical Device Identity) to be *exchanged* for a short-lived JWT SVID suitable for bearer-style authentication in non-mTLS environments. Other identity profiles may use direct issuance of JWT SVIDs via the enrollment endpoint instead of this exchange pattern.

##### Example: JWT SVID Exchange <!-- omit from toc -->

**Example request (using client assertion JWT):**

```http
POST /api/v1/identities/c3BpZmZlOi8vbm9ydGhzdGFyLWlkYS5jb20vbWFyZ28vZGV2aWNlLzEyM2U0NTY3LWU4OWItMTJkMy1hNDU2LTQyNjYxNDE3NDAwMA/jwt-svid
Content-Type: application/json
```

```jsonc
{
  "aud": [
    "https://dfm.northstar-ida.com/",
    "https://observability.example.com/"
  ],
  "ttl": 300,
  "client_assertion": "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCIsInR5cGUiOiJzcGlmZmUranV3dCJ9.eyJpc3MiOiJzcGlmZmU6Ly9ub3J0aHN0YXItaWRhLmNvbS9tYXJnby9kZXZpY2UvMTIzZTQ1NjctZTg5Yi0xMmQzLWE0NTYtNDI2NjE0MTc0MDAwIiwic3ViIjoic3BpZmZlOi8vbm9ydGhzdGFyLWlkYS5jb20vbWFyZ28vZGV2aWNlLzEyM2U0NTY3LWU4OWItMTJkMy1hNDU2LTQyNjYxNDE3NDAwMCIsImF1ZCI6WyJodHRwczovL21pcy5ub3J0aHN0YXItaWRhLmNvbS9hcGkvdjEvaWRlbnRpdGllcy9jM0JwWm1abE9pOHZibTl0YzJWeS9qd3Qtc3ZpZCIsImh0dHBzOi8vbWlzLm5vcnRoc3Rhci1pZGEuY29tLyJdLCJleHAiOjE3MzAyMTQ3MDAsImlhdCI6MTczMDIxNDYwMCwianRpIjoiNjk4MWNkMWUtZGI2YS00MmE1LTk1NDgtNzQ3NWIxMGY2MWNkIn0.<signature-truncated>"
}
```

**Example response (`201 Created`):**

```jsonc
{
  "jwt": "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCIsInR5cGUiOiJzcGlmZmUranV3dCJ9.eyJzdWIiOiJzcGlmZmU6Ly9ub3J0aHN0YXItaWRhLmNvbS9tYXJnby9kZXZpY2UvMTIzZTQ1NjctZTg5Yi0xMmQzLWE0NTYtNDI2NjE0MTc0MDAwIiwiYXVkIjpbImh0dHBzOi8vZGZtLm5vcnRoc3Rhci1pZGEuY29tLyIsImh0dHBzOi8vb2JzZXJ2YWJpbGl0eS5leGFtcGxlLmNvbS8iXSwiZXhwIjoxNzMwMjE0NzAwLCJpYXQiOjE3MzAyMTQ2MDAsImlzcyI6Imh0dHBzOi8vbWlzLm5vcnRoc3Rhci1pZGEuY29tLyJ9.hM8Z...-truncated",
  "expires_at": "2025-10-25T14:12:31Z"
}
```

> **Informative:**
> In this example, the device cannot use mTLS towards the MIS but has access to its X.509 SVID private key. It signs a short-lived `client_assertion` JWT that identifies its SPIFFE ID and the JWT SVID exchange endpoint as audience. MIS validates the assertion and issues a short-lived JWT SVID whose `sub` is the device's SPIFFE ID and whose `aud` matches the requested audiences, subject to policy.

#### Revocation List Endpoint

This endpoint provides a **machine-readable list of revoked SVIDs** within the Trust Domain maintained by the Margo Identity Service (MIS).

Clients and services use it to check SVID status and enforce revocation, without relying solely on traditional PKI mechanisms such as CRLs or OCSP.

| Item | Value |
| :--- | :---- |
| **Endpoint** | `GET /api/v1/revocations` |
| **Authentication** | None (public resource, HTTPS required) |
| **Headers** | `Accept: application/json` |
| **Body schema (request)** | None |
| **Body schema (response)** | See below |
| **Responses** | `200 OK` - list of revoked SVIDs<br>`304 Not Modified` - list unchanged<br>`404 Not Found` - revocation list unavailable |
| **Errors** | RFC 9457 Problem Details as per [Appendix B](#appendix-b-error-responses-normative) |

**Response body schema (`200 OK`, `application/json`):**

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `last_updated` | string (ISO 8601) | Y | UTC timestamp when the list was last generated. Used by clients to detect updates. |
| `revoked_svids` | array of object | Y | Array of revoked SVID records. Clients **MUST** treat this list as authoritative for the trust domain. |
| `revoked_svids.cert_fingerprint_sha256` | string | Y | Lowercase hexadecimal representation of the SHA-256 digest of the DER-encoded **leaf X.509 SVID certificate** (no prefixes or delimiters). Consumers **MUST** use this value as the primary identifier when checking revocation status. |
| `revoked_svids.serial_number` | string | Y | Uppercase hexadecimal representation of the X.509 certificate serial number, without prefixes or delimiters. |
| `revoked_svids.revoked_at` | string (ISO 8601) | Y | UTC timestamp when the SVID was revoked. Consumers **SHOULD** ignore entries with timestamps in the future. |

This list covers **X.509 SVIDs** (identified by the leaf certificate SHA-256 fingerprint). **JWT SVIDs are expected to be short-lived and are not listed individually**; their primary protection is limited lifetime and audience scoping.

**Revocation matching rules:**

- Consumers **MUST** compute the SHA-256 digest of the DER-encoded **leaf** X.509 SVID certificate they are validating, encode it as lowercase hex (no delimiters), and compare it to `cert_fingerprint_sha256` entries.
- Consumers **MUST NOT** use `serial_number` as the sole revocation identifier, because serial numbers are not globally unique across issuers.

> **Informative:**
> The MIS **SHOULD** support standard caching headers (`ETag`, `Last-Modified`) to allow efficient synchronization. Clients **SHOULD** periodically poll this endpoint or subscribe to a trust-bundle update mechanism to maintain up-to-date revocation data.

##### Example: Revocation List Retrieval <!-- omit from toc -->

**Example request:**

```http
GET /api/v1/revocations
Accept: application/json
```

**Example response (`200 OK`):**

```jsonc
{
  "last_updated": "2025-10-25T14:12:31Z",
  "revoked_svids": [
    {
      "cert_fingerprint_sha256": "2f4c3d9a7b1e0c6d8f2a1b9c0d7e6f5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c0d9e",
      "serial_number": "8F12A4C9D23E41B1",
      "revoked_at": "2025-10-20T09:33:45Z"
    },
    {
      "cert_fingerprint_sha256": "9c1d0e2f3a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5",
      "serial_number": "A74E91F1B8CC4092",
      "revoked_at": "2025-10-21T17:58:11Z"
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
> Clients and services validating SVIDs should periodically refresh this list to ensure that revoked identities cannot be used for authentication. Deployments that implement OAuth2 integration as described in [Appendix C](#appendix-c-oauth2-and-api-gateway-interoperability-informative) **SHOULD** also consult this list when mapping SVIDs into other token forms.

##### Revocation Model <!-- omit from toc -->

The Margo Identity Service (MIS) **MUST** maintain a consistent revocation model to ensure that compromised or decommissioned identities cannot be used for authentication.

1. **Short-lived credentials (primary containment)**

   - X.509 SVID and JWT SVID lifetimes **MUST** be limited according to [Cryptographic Requirements](#cryptographic-requirements) and profile-specific guidance.
   - For JWT SVIDs, deployments **SHOULD** use lifetimes on the order of minutes and **MUST NOT** accept tokens past `exp`. Revocation of individual JWT SVIDs is out of scope.
   - Clients and servers **MUST** reject expired SVIDs and **MUST NOT** cache authorization decisions beyond the SVID lifetime.

2. **Margo-native revocation list (secondary defense)**

    - MIS **MUST** maintain a JSON-based revocation list of revoked SVIDs for the Trust Domain.
    - Consumers **MUST** match revoked X.509 SVIDs using the leaf certificate SHA-256 fingerprint (`cert_fingerprint_sha256`).
    - Clients **SHOULD** use HTTP caching semantics (`ETag`, `If-None-Match`, `Last-Modified`, `If-Modified-Since`) when supported by MIS to minimize bandwidth.

3. **Standard PKI revocation (optional)**

   - Deployments integrating with external CAs (see [MIS Deployment Modes](#mis-deployment-modes-informative)) **MAY** also rely on standard PKI revocation mechanisms (for example, OCSP, CRLs).
   - These are **optional** extensions for hybrid or enterprise environments.

> **Scalability Note (Informative):**
> For the scale of early adoption and GA (tens of thousands of devices), a cached JSON list provides the best balance of reliability, simplicity, and offline resilience. For hyper-scale deployments (millions of devices), future versions of this specification **MAY** introduce differential updates (e.g., CRLite) or OCSP support.

### 6. Typical Workflows (informative)

This section illustrates how the components and APIs defined in previous chapters operate together in practice.
The workflows show representative message flows for device enrollment, renewal, and JWT SVID use, providing a practical, end-to-end view of how a Margo deployment maintains identity and trust over time.

These sequences are **informative** only and do not introduce additional normative requirements.

> **Note:** These workflows are provided for illustrative purposes to demonstrate the interaction between components. In the event of any discrepancy between these diagrams and the normative API definitions and schemas in **Section 5**, the normative definitions in **Section 5** take precedence.

#### End-to-End Device Lifecycle Flow

This flow represents the complete "golden path" for a new **Edge Compute Device**, from bootstrap through its first authenticated request using **mutual TLS (mTLS)** with an **X.509 SVID**.

> **Note:**
> This flow illustrates the **default case** for Margo deployments, where devices authenticate directly to Margo components using mTLS and X.509 SVIDs.
> Scenarios where mTLS is not feasible (for example, due to TLS termination at a proxy) are covered separately in [**JWT SVID Usage in Proxy Scenarios**](#jwt-svid-usage-in-proxy-scenarios).

```mermaid
sequenceDiagram
    participant Device
    participant MIS
    participant RS as Resource Server (e.g., WFM)

    rect rgb(230,230,230)
        note over Device,MIS: Discovery & Trust Bootstrap
        Device->>MIS: GET /.well-known/margo
        MIS-->>Device: 200 OK (Discovery Document)
        Device->>MIS: GET trust_bundle_uri
        MIS-->>Device: 200 OK (SPIFFE Bundle Map / Trust Bundle)
    end

    rect rgb(235,235,235)
        note over Device,MIS: Enrollment
        Device->>Device: Generate SVID key pair + CSR
        Device->>MIS: POST /api/v1/identities<br/>(bootstrapCredential, svid_profile_uri, svid_request)
        activate MIS
        MIS->>MIS: Validate bootstrap credential & policy<br/>Derive Enrollment Subject Identifier (ESI)<br/>Bind Physical to Logical Device Identity (LDI)
        MIS-->>Device: 201 Created (X.509 SVID)
        deactivate MIS
    end

    rect rgb(245,245,245)
        note over Device,RS: Authenticated Request (mTLS)
        Device->>RS: Establish TLS 1.3 (mutual TLS)<br/>(present X.509 SVID as client certificate)
        activate RS
        RS->>RS: Validate SVID chain & SPIFFE ID with Trust Bundle
        RS-->>Device: 200 Success
        deactivate RS
    end
```

#### Device SVID Renewal Flow

This sequence shows how a device renews its SVID before expiry.

```mermaid
sequenceDiagram
    participant Device
    participant MIS

    note over Device: Current X.509 SVID nearing expiration

    rect rgb(235,235,235)
        note over Device,MIS: SVID Renewal
        Device->>MIS: POST /api/v1/identities/{spiffeIdEncoded}/renewals<br/>(svid_profile_uri, svid_request[CSR])
        activate MIS
        MIS->>MIS: Authenticate via current identity<br/>(mTLS with current X.509 SVID or JWT SVID (Bearer))
        MIS->>MIS: Validate CSR & policy (key rotation permitted?) 
        MIS-->>Device: 200 OK (new X.509 SVID)
        deactivate MIS
    end

    note over Device: Replace prior SVID with newly issued SVID
```

> **Notes:**
>
> - Key rotation during renewal is **policy-controlled**. If a new key pair is presented in the CSR
>   and rotation is disallowed, MIS returns `409 Conflict`.
> - JWT SVIDs derived from an X.509 SVID are **not renewed** using this endpoint.
>   Devices **MUST** obtain a fresh JWT SVID via the [JWT SVID Exchange Endpoint](#jwt-svid-exchange-endpoint).
>   Future profiles may allow direct JWT SVID renewal for natively issued JWT identities.

#### JWT SVID Usage in Proxy Scenarios

This flow shows how an Edge Compute Device that holds a valid **X.509 SVID** can obtain and use a **JWT SVID** when **end-to-end mTLS is not feasible** because a TLS-terminating proxy sits between the device and other Margo components.

> **Assumptions:**
>
> - A **TLS-terminating proxy** (transparent or explicit) is present on the network path between the device and both the **Margo Identity Service (MIS)** and the **Resource Server** (for example, a DFM).
> - The proxy terminates TLS from the device and opens a separate TLS connection to the MIS / Resource Server. As a result, **end-to-end mTLS** between the device and these services is not possible.
> - The device *does* have access to the private key corresponding to its X.509 SVID, allowing it to authenticate to MIS using a **client assertion JWT** at the `/jwt-svid` endpoint.

```mermaid
sequenceDiagram
    participant Device
    participant Proxy as TLS-Terminating Proxy
    participant MIS
    participant RS as Resource Server (e.g., DFM)

    note over Device: Device already enrolled<br/>and holds an X.509 SVID

    rect rgb(235,235,235)
        note over Device,MIS: JWT SVID Exchange via Proxy
        Device->>Device: Create client_assertion JWT<br/>(signed with X.509 SVID key)
        Device->>Proxy: HTTPS POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid<br/>(aud, ttl, client_assertion)
        activate Proxy
        Proxy->>MIS: HTTPS POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid<br/>(forwarded body)
        activate MIS
        MIS->>MIS: Validate client_assertion using Trust Bundle<br/>Verify sub == SPIFFE ID from {spiffeIdEncoded}
        MIS-->>Proxy: 201 Created (JWT SVID)
        deactivate MIS
        Proxy-->>Device: 201 Created (JWT SVID)
        deactivate Proxy
    end

    rect rgb(245,245,245)
        note over Device,RS: Accessing Resource via JWT SVID
        Device->>Proxy: HTTPS POST /api/v1/device-operation<br/>(Authorization: Bearer <jwt-svid>)
        activate Proxy
        Proxy->>RS: HTTPS POST /api/v1/device-operation<br/>(Authorization: Bearer <jwt-svid>)
        activate RS
        RS->>RS: Validate JWT SVID (signature, exp, aud)<br/>using Trust Bundle of Trust Domain
        RS-->>Proxy: 200/201 Success
        Proxy-->>Device: 200/201 Success
        deactivate RS
        deactivate Proxy
    end
```

The `client_assertion` used at the exchange endpoint **MUST** use an algorithm permitted by the [Cryptographic Requirements](#cryptographic-requirements) and the key associated with the active X.509 SVID. The Resource Server **MUST** validate the JWT SVID's `aud`, `exp`, and signature using the Trust Bundle for the Trust Domain.

> **Informative:**
> In this pattern, the proxy is *identity-transparent*: it terminates TLS but forwards the application-layer request and the `Authorization: Bearer <jwt-svid>` header unchanged. MIAF does not require the proxy to understand SPIFFE or SVIDs. It only requires that the **Resource Server** and **MIS** validate SVIDs using the Trust Bundle and the rules defined in this SUP.

#### Bootstrap Method Flows

The following flows expand on [Enrollment and Identity Issuance](#enrollment-and-identity-issuance-endpoint) and illustrate selected bootstrap methods defined in [Appendix A: Bootstrap Methods (Normative)](#appendix-a-bootstrap-methods-normative).
They are **informative only** and do not introduce additional normative requirements.

Each flow shows how a device presents its bootstrap credential, how the MIS validates it, and how the **enrollment subject identifier** (as defined in [Section 5](#mis-validation-and-processing-logic)) is derived from that credential to establish a deterministic binding between the physical credential and the resulting identity.

##### Example: Factory Certificate Method (mTLS)

```mermaid
sequenceDiagram
    participant Device
    participant MIS

    Device->>Device: Generate SVID key pair + CSR
    Device->>MIS: Establish TLS 1.3 (mutual TLS)<br/>(present factory leaf certificate)
    activate MIS
    MIS->>MIS: Validate client certificate chain<br/>(manufacturer CA, revocation status)
    Device->>MIS: POST /api/v1/identities<br/>(svid_profile_uri, CSR, bootstrapCredential.method)
    MIS->>MIS: Derive enrollment subject identifier = SHA-256 fingerprint of factory leaf certificate
    MIS->>MIS: Apply policy, bind Physical to Logical Identity
    MIS-->>Device: 201 Created<br/>(X.509 SVID)
    deactivate MIS
```

> **Alignment with [Appendix A](#appendix-a-bootstrap-methods-normative):**
>
> - `bootstrapCredential.method` = `urn:margo:bootstrap:factory-cert-mtls:v1`.
> - The credential is conveyed implicitly through the mTLS client certificate (no proof field).
> - The **Enrollment Subject Identifier (ESI)** is the **SHA-256 fingerprint** of the DER-encoded factory **leaf** certificate presented during the TLS handshake.

##### Example: Factory Certificate Method (JWT Assertion)

```mermaid
sequenceDiagram
    participant Device
    participant MIS

    Device->>Device: Generate SVID key pair + CSR
    Device->>Device: Create JWT assertion (signed with factory key)<br/>(include x5c header with full factory chain)
    Device->>MIS: POST /api/v1/identities<br/>(svid_profile_uri, CSR, bootstrapCredential.method, bootstrapCredential.proof.assertion)
    activate MIS
    MIS->>MIS: Verify JWT signature and certificate chain
    MIS->>MIS: Validate claims (aud, exp, jti)
    MIS->>MIS: Derive enrollment subject identifier = SHA-256 fingerprint of leaf certificate from x5c[0]
    MIS->>MIS: Apply policy, bind Physical to Logical Identity
    MIS-->>Device: 201 Created<br/>(X.509 SVID)
    deactivate MIS
```

> **Alignment with [Appendix A](#appendix-a-bootstrap-methods-normative):**
>
> - `bootstrapCredential.method` = `urn:margo:bootstrap:factory-cert-jwt:v1`.
> - `bootstrapCredential.proof.assertion` is a compact JWT signed with the factory private key.
> - The **Enrollment Subject Identifier (ESI)** is the **SHA-256 fingerprint** of the DER-encoded **leaf** certificate in `x5c[0]`.

##### Example: FIDO Device Onboard (MIS-hosted OOS)

```mermaid
sequenceDiagram
    participant Device
    participant MIS as Margo Identity Service (includes OOS)

    Device->>MIS: Execute FDO TO2 protocol<br/>(ServiceInfo includes CSR)
    activate MIS
    MIS->>MIS: Validate FDO proof<br/>(ownership voucher chain)
    MIS->>MIS: Invoke enrollment logic<br/>(svid_profile_uri, CSR, bootstrapCredential.method = FDO)
    MIS->>MIS: Derive enrollment subject identifier = SHA-256 fingerprint of DER-encoded voucher device leaf certificate
    MIS->>MIS: Apply policy, bind Physical to Logical Identity
    MIS-->>Device: Deliver SVID and Trust Bundle (via TO2 ServiceInfo)
    deactivate MIS
```

> **Alignment with [Appendix A](#appendix-a-bootstrap-methods-normative):**
>
> - `bootstrapCredential.method` = `urn:margo:bootstrap:fdo:v1`.
> - `bootstrapCredential.proof` includes the FDO Ownership Voucher.
> - The **Enrollment Subject Identifier (ESI)** is derived from the **device leaf certificate** contained in the Ownership Voucher.
> - The **Owner Onboarding Service (OOS)** is implemented as a component of the MIS and acts on behalf of the device.

### 7. Transport Layer Security (TLS) Requirements

All communication between **Margo components** - including **Edge Compute Devices**, the **Margo Identity Service (MIS)**, and **Resource Servers** - **MUST** occur over **HTTPS secured with TLS**.
TLS provides confidentiality, integrity, and peer authentication for all MIAF identity and authorization exchanges.

This section defines the **interoperability and security baseline** for TLS in Margo.
Implementations **SHOULD** rely on well-maintained TLS 1.3 libraries and follow current industry guidance in the cited RFCs.

#### Relationship to MIAF and Profiles

TLS forms the transport security layer beneath MIAF.

- When authenticating via **mutual TLS (mTLS)**, the client certificate **MUST** be a valid **X.509 SVID** issued by the **Margo Identity Service** of the applicable **Trust Domain**.
- When authenticating using a **JWT SVID**, the session **MUST** still be protected by HTTPS to preserve token confidentiality and integrity in transit.

These requirements ensure that all MIAF operations - discovery, enrollment, renewal, JWT SVID exchange, and revocation - occur over authenticated and encrypted channels.

#### Initial Trust Bootstrap

All MIAF trust semantics ultimately depend on an initial root of trust for reaching the MIS in the first place.
In particular, a new client cannot validate MIS-issued SVIDs using the Trust Bundle until it has retrieved that Trust Bundle.

Therefore, clients **MUST** authenticate the HTTPS connection used to retrieve the discovery document (`GET /.well-known/margo`) and the Trust Bundle (`trust_bundle_uri`) using an **initial trust mechanism** that exists prior to this protocol.
At least one of the following mechanisms **MUST** be used:

1. **Web PKI / enterprise PKI:** Validate the MIS server certificate chain to a configured set of trust anchors and validate the expected DNS name per [RFC 6125](https://datatracker.ietf.org/doc/html/rfc6125).
2. **Pinned trust:** Validate the MIS server certificate chain or public key against operator-provisioned pins (for example, a pinned CA certificate).
3. **Secure bootstrap delivery:** In bootstrap-channel-delivered scenarios (for example, FDO TO2), obtain the initial Trust Bundle and MIS endpoint metadata through the authenticated bootstrap channel and treat it as authoritative.

Clients **MUST NOT** treat the first retrieval of the discovery document or Trust Bundle as unauthenticated or "trust on first use".

#### Minimum TLS Baseline

| Requirement | Normative Directive | Reference |
| :---------- | :------------------ | :-------- |
| **Protocol Version** | Implementations **MUST** support **TLS 1.3**. Support for TLS 1.2 **MAY** be provided for compatibility but **MUST** follow modern security guidance. | [RFC 8446 - TLS 1.3](https://datatracker.ietf.org/doc/html/rfc8446), [RFC 9325 §4](https://datatracker.ietf.org/doc/html/rfc9325#section-4) |
| **Deprecated Versions** | SSL v2, SSL v3, TLS 1.0, and TLS 1.1 **MUST NOT** be used. | [RFC 8996](https://datatracker.ietf.org/doc/html/rfc8996) |
| **Cipher Suites** | Only **AEAD** cipher suites providing Perfect Forward Secrecy (PFS) **MUST** be supported. | [RFC 8446](https://datatracker.ietf.org/doc/html/rfc8446#section-9.1) |
| **Key Exchange** | Ephemeral key exchange (e.g., ECDHE) **MUST** be used for all sessions. | [RFC 8446](https://datatracker.ietf.org/doc/html/rfc8446) |
| **Hash Algorithms** | SHA-256 or stronger **MUST** be used; MD5 and SHA-1 **MUST NOT**. | [NIST SP 800-131A Rev 2](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-131Ar2.pdf) |

#### Certificate Validation

Each endpoint **MUST** validate the peer's TLS certificate chain and identity in accordance with [RFC 5280 - X.509 Certificate Profile](https://datatracker.ietf.org/doc/html/rfc5280).

Validation **MUST** include the following, depending on context:

- The validating party **MUST** verify that the presented certificate chain is within its validity period and **MUST** reject expired certificates.

- **Server identity for MIS HTTPS endpoints (discovery, Trust Bundle retrieval, and MIS APIs):**

  - The client **MUST** validate the server certificate chain to its configured initial trust anchors (see [Initial Trust Bootstrap](#initial-trust-bootstrap)).
  - The client **MUST** validate the expected DNS name per [RFC 6125 - Service Identity in TLS](https://datatracker.ietf.org/doc/html/rfc6125).

- **SVID identity for MIAF mTLS (X.509 SVIDs):**

  - The verifier **MUST** validate any presented X.509 SVID chain against the Trust Domain's Trust Bundle.
  - The verifier **MUST** extract the SPIFFE ID from the **URI SAN** and treat it as the **authoritative identity claim**.
  - DNS hostname matching is not applicable to SVID identity validation and **MUST NOT** override SPIFFE ID validation.

  Revocation checking for X.509 SVIDs is defined by the applicable profile and revocation model (see [Revocation List Endpoint](#revocation-list-endpoint)).

### 8. Security Considerations

This section defines the **normative security properties** and **threat mitigations** underpinning Margo's identity and authorization architecture.

#### Threat Model Overview

MIAF assumes an **adversarial network** and the possibility of **compromised individual devices or services**.
Primary objectives: protect private keys, preserve identity integrity, and minimize the blast radius of a compromise.

| Threat | Description | Mitigation  |
| :----- | :---------- | :---------- |
| **Unauthorized Enrollment** | An attacker attempts to enroll without valid bootstrap credentials. | MIS **MUST** cryptographically validate all bootstrap proofs as defined by the selected bootstrap method. Each method **MUST** use manufacturer-verified or hardware-rooted credentials. |
| **Replay or Theft of SVIDs / Tokens** | An intercepted X.509 SVID or JWT SVID is reused outside its intended context. | All SVIDs **MUST** be short-lived. Resource Servers **MUST** verify signature, expiry, and audience. JWT SVIDs **MUST** be bound to specific audiences and limited lifetimes (<= 5 minutes recommended). |
| **Private Key Compromise** | An attacker exfiltrates a device's private key. | Keys **MUST** be stored in secure hardware (TPM, TEE, SE) where available; where only software storage is possible, implementations **MUST** provide at-rest encryption, integrity protection, and OS/process isolation (see [Device Key Protection](#device-key-protection)). MIS renewal **MUST** require fresh proof of possession (via CSR or bootstrap credential) before issuing a replacement SVID. |
| **Certificate Revocation Lag** | Revocation events are not propagated promptly. | MIS **SHOULD** maintain and publish a [JSON-based revocation list](#revocation-list-endpoint) within its Trust Domain. Clients and servers **SHOULD** poll or cache this list periodically to maintain fresh state. |
| **Denial-of-Service (DoS)** | Attackers flood enrollment or renewal requests to exhaust MIS resources. | MIS **MUST** rate-limit enrollment, renewal, and JWT SVID exchange operations and return `429 Too Many Requests` with `Retry-After`. Clients **MUST NOT** retry before this interval. |
| **Service Impersonation / MITM** | An adversary attempts to impersonate MIS or another service. | All endpoints **MUST** use HTTPS with TLS 1.3 and strict certificate validation. Clients **MUST** verify that peer certificates are valid SVIDs issued under the expected Trust Domain and signed by anchors in the Trust Bundle. |
| **Replay of Bootstrap Assertions** | A factory JWT assertion or bootstrap credential is captured and re-submitted. | MIS **MUST** reject any assertion with duplicate `jti` values and **MUST** enforce tight time windows (`exp` <= 5 minutes). |
| **Cross-Domain Trust Confusion** | Components accept identities from unintended Trust Domains. | Verifiers **MUST** determine the Trust Domain from the SPIFFE ID and **MUST NOT** trust SVIDs unless the domain is explicitly configured or federated. |

### 9. Future Work: WFM Client Identity Profile (Informative)

The **Edge Compute Device Identity Profile** defined in this SUP establishes the authentication **foundation** for Edge Compute Devices within a Trust Domain. However, the device SVID is **not itself** the credential used to authenticate to the WFM API.

A device's Logical Device Identity proves that a specific physical or virtual platform is authentic and enrolled within the Trust Domain. A **WFM Client**, by contrast, is a software component that runs on one or more devices and interacts with a specific Workload Fleet Manager. These are fundamentally different principals:

- **Different lifecycles.** A device identity is established at bootstrap and persists across software updates. A WFM Client identity is established when a client registers with a WFM and ceases when the binding is removed - independently of the device's continued existence.
- **Different authorization scopes.** A device identity asserts *"this device is authentic."* A WFM Client identity asserts *"this client is authorized to retrieve deployments from and report status to WFM X."*
- **Different cardinalities across topologies.** Margo supports multiple device topologies, each with a distinct mapping between devices and WFM Clients:

  | Topology | Device : WFM Client |
  | :------- | :------------------ |
  | Standalone Device | 1 : 1 |
  | Kubernetes Cluster | N devices : 1 WFM Client |
  | Device Gateway | 1 gateway device (+N non-Margo sub-devices) : 1 WFM Client |

  In the cluster case, multiple devices share a single WFM Client relationship - the cluster's WFM Client identity must survive leader failover without being bound to any single device's SVID. In the gateway case, a single gateway device holds one WFM Client identity and mediates access to non-Margo sub-devices on their behalf; sub-device targeting is handled through payload-level routing fields (such as `deviceId`), not through separate identities.

A dedicated **WFM Client Identity Profile** is expected to be defined in a subsequent SUP. That profile is anticipated to:

- replace PR1's WFM-specific `client_id` model with a MIAF-defined WFM Client identity;
- define the SPIFFE ID path format for WFM Client identities within the Trust Domain;
- define how device identities are used to bind WFM Client credentials for each supported deployment topology;
- address the lifecycle requirements of each supported topology (standalone, cluster, gateway); and
- inform the corresponding updates to the WFM API specification (for example, replacing `{clientId}` path parameters and PR1's RFC 9421 HTTP Message Signatures security scheme with MIAF-based authentication).

Until that profile is defined, this SUP does **not** specify how WFM Clients authenticate to WFM APIs. A device SVID is not itself a WFM Client credential. The device identity defined here is the prerequisite for, but not a substitute for, WFM Client identity.

## Alternatives considered (optional)

This section documents alternative mechanisms that were evaluated during the design of the **Margo Identity and Authorization Framework (MIAF)** and the **Edge Compute Device Identity Profile**.
Each alternative was rejected for specific technical or operational reasons, although certain principles were retained where compatible with Margo's goals of interoperability, simplicity, and cryptographic assurance.

### Certificate-Based Device Enrollment Protocols

| Protocol | Reference | Evaluation Summary | Decision |
| :------- | :-------- | :----------------- | :------- |
| **EST (Enrollment over Secure Transport)** | [RFC 7030](https://datatracker.ietf.org/doc/html/rfc7030) | Provides standardized certificate enrollment over HTTPS, but assumes TLS-based client authentication only. Lacks extensibility for modern, pluggable bootstrap proofs (e.g., JWT or FDO) and cannot carry structured JSON credentials natively. | **Rejected** - insufficient flexibility for multi-method bootstrap and non-TLS proofs. |
| **SCEP (Simple Certificate Enrollment Protocol)** | [IETF Draft](https://datatracker.ietf.org/doc/html/draft-nourse-scep-00) | Relies on PKCS#7 payloads and a shared-secret "challenge password." Mechanically incompatible with SPIFFE identity semantics and unsuitable for modern cryptographic agility. | **Rejected** - legacy design and weak authentication model. |
| **ACME (Automated Certificate Management Environment)** | [RFC 8555](https://datatracker.ietf.org/doc/html/rfc8555) | Uses modern JSON/HTTPS exchanges but is designed for *proving control over existing identifiers* (e.g., DNS names), not for *assignment* of new device identities. Would require non-standard challenges and multi-step orchestration. | **Rejected** - excessive protocol overhead and semantic mismatch for device identity. |

#### Rationale for standardized `/api/v1/identities` endpoint

The **Margo Identity Service (MIS)** exposes a unified, **Margo-specific enrollment API built on SPIFFE primitives** that supports pluggable **bootstrap methods** and structured **SVID profile negotiation**.
Unlike EST, SCEP, or ACME, this approach allows Margo to:

- carry JSON-encoded credential proofs (for example, JWT assertions, FDO vouchers);
- deterministically derive an **enrollment subject identifier** from the bootstrap material;
- issue SVIDs bound to a stable **Logical Device Identity**; and
- remain stateless with respect to session management.

This design balances interoperability and flexibility while remaining consistent with SPIFFE's trust-domain model and avoiding dependencies on any specific PKI protocol or topology.

### OAuth 2.0 / Authorization Server Integration

Early designs proposed using OAuth 2.0 **Authorization Servers (AS)** as part of the normative enrollment and authorization workflow.
This was **rejected for the normative core**, because it would:

- require every MIAF-compliant deployment to maintain a full OAuth 2.0 infrastructure;
- introduce additional moving parts (token lifecycles, introspection endpoints) unrelated to MIAF's cryptographic identity model; and
- duplicate functionality already achieved through verified SPIFFE IDs and SVIDs.

Instead, OAuth 2.0 integration is retained **informatively** in [Appendix C](#appendix-c-oauth2-and-api-gateway-interoperability-informative).
Deployments that require interoperability with existing enterprise access control systems may map MIAF-issued SVIDs to OAuth tokens or API-gateway mechanisms at that layer.

### Alternative Trust Frameworks

- **Vendor-specific PKI hierarchies.**
  Rejected because per-vendor roots would fragment the ecosystem and prevent cross-vendor interoperability.
  MIAF's **SPIFFE-based Trust Domain** model provides a unified namespace and validation semantics across all vendors and components.

- **Device identity as static bearer tokens.**
  Rejected because static tokens cannot offer hardware-bound, verifiable authenticity or controlled lifecycle management.
  MIAF's use of **short-lived, cryptographically verifiable SVIDs** ensures continuous proof of possession and prevents long-term credential reuse.

- **Proprietary API-key or non-standard authorization schemes.**
  Rejected because they lack verifiable identity semantics and auditable lifecycle management.
  MIAF adopts **policy-based authorization** directly over verified SPIFFE IDs, with optional mapping into OAuth 2.0 tokens for environments that require delegated access.

## Appendix A: Bootstrap Methods (Normative)

This appendix defines the registered **bootstrap methods** supported by this specification, including their identifiers, object schemas, derivation of the **Enrollment Subject Identifier (ESI)**, and validation requirements. Each method describes how an Edge Compute Device proves its initial authenticity to the **Margo Identity Service (MIS)** before receiving an **X.509 SVID representing the device's Logical Device Identity (LDI)**.

All cryptographic algorithms, key sizes, and signatures referenced in this appendix **MUST** comply with [Cryptographic Requirements](#cryptographic-requirements).

### Common Validation Rules

Unless a method states stricter requirements, the MIS **MUST** enforce the following for all device bootstrap requests:

1. **Freshness and replay protection:**

   - Any signed assertion **MUST** include `iat`/`exp` with `exp - iat <= 300 seconds`.
   - Assertions **MUST** include a unique `jti`; the MIS **MUST** reject replays of a previously seen `jti`.

2. **Audience binding:**

   - Where a JWT/JWS is used, `aud` **MUST** equal the exact enrollment endpoint URL advertised by the MIS.

3. **Chain validation:**

   - Any certificate chain presented (mTLS or `x5c`) **MUST** validate to a manufacturer/OEM trust anchor authorized by Trust Domain policy.

4. **ESI derivation:**

   - The MIS **MUST** derive the **Enrollment Subject Identifier** exactly as specified by the selected method and use it to locate or create the LDI binding (see [MIS Validation and Processing Logic](#mis-validation-and-processing-logic)).

5. **Bootstrap trust anchor provisioning:**

   - The MIS **MUST** be configured with the trust anchors (e.g., manufacturer or OEM root and intermediate CA certificates) needed to validate Bootstrap Credentials for each supported Bootstrap Method.
   - The mechanism for provisioning these trust anchors is deployment-specific and outside the scope of this specification.

### FIDO Device Onboard (FDO) Method

> **TODO:** This method is a draft outline. The FDO integration details (voucher handling, OOS interaction model, ESI derivation) need to be validated against the FDO specification and refined.

This method enables **secure, hardware-rooted onboarding** using [FIDO Device Onboard (FDO)](https://fidoalliance.org/specs/FDO/).
It supports automated, authenticated transfer of device ownership from factory to operator, allowing devices to join a Trust Domain without prior configuration or manual provisioning.

**Bootstrap Method Identifier (URN):**
`urn:margo:bootstrap:fdo:v1`

**Purpose:**
Use a hardware-rooted onboarding mechanism compatible with FDO to enable factory-provisioned devices to securely transfer ownership into an operational Trust Domain.

**Enrollment Subject Identifier (ESI):**
Implementations **MUST** derive the ESI as the **SHA-256 fingerprint of the DER-encoded device leaf certificate** contained in the presented FDO Ownership Voucher.
The resulting SHA-256 digest **MUST** be encoded as lowercase hexadecimal.

**`bootstrapCredential` object schema (`application/json`):**

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `method` | string | Y | **MUST** be `urn:margo:bootstrap:fdo:v1`. |
| `proof` | object | Y | **MUST** contain `ownershipVoucher`. |
| `proof.ownershipVoucher` | string | Y | Base64url-encoded (no padding) CBOR bytes of the FDO **Ownership Voucher**. The MIS **MUST** validate the voucher chain per FDO before issuing an identity. |

**Submission and authentication:**

- The MIS **MUST** provide an FDO **Owner Onboarding Service (OOS)** endpoint that acts as the TO2 Owner Onboarding Service for this Trust Domain.
- Devices enrolling via this method **MUST** perform FDO TO2 directly with the MIS-provided OOS endpoint.
- After TO2 completes, the MIS OOS component **MUST** submit the CSR and the FDO proof to the MIS enrollment logic on behalf of the device (conceptually, `POST /api/v1/identities`).

> **Deployment note (informative):**
> The OOS is commonly deployed as a component of the MIS (consistent with the FDO definition of OOS as a component of the management service). In such deployments, `POST /api/v1/identities` may be an internal interface rather than a public API. If the OOS is separated from the main MIS enrollment service, the OOS **MUST** authenticate to the enrollment service using mutual TLS with an **X.509 SVID** whose SPIFFE ID is authorized by Trust Domain policy to perform FDO-backed enrollment.

**Voucher validation:**

- The MIS **MUST** validate the Ownership Voucher chain per the FDO specification and Trust Domain policy (including verifying that the voucher is rooted in an authorized manufacturer/OEM trust anchor).
- The MIS **MUST** treat the device leaf certificate contained in the voucher as non-secret and use it only for ESI derivation and authorization decisions.

**CSR handling:**

- The OOS **MUST** obtain the CSR from the device over the authenticated TO2 channel (for example, via ServiceInfo using the `fdo.csr` module) and **MUST** ensure the CSR corresponds to the device that completed TO2.
- The MIS **MUST** validate that the submitted CSR is well-formed and that its signature verifies (proof of possession).

> **Reference (informative):**
> The `fdo.csr` ServiceInfo Module (FSIM) is specified in the FIDO Alliance FDO SIM repository: <https://github.com/fido-alliance/fdo-sim/blob/FSIM_v1.0_20230209/fsim-repository/fdo.csr.md>

**Process Summary (informative):**

1. The device executes **FDO TO2** with the MIS-hosted **Owner Onboarding Service (OOS)** and submits a CSR via TO2 **ServiceInfo** (commonly using the `fdo.csr` ServiceInfo Module).
2. The MIS OOS validates the Ownership Voucher and device credentials per FDO.
3. The MIS validates the Ownership Voucher chain against authorized trust anchors and applies Trust Domain policy.
4. MIS derives **ESI = SHA-256 fingerprint of the DER-encoded voucher device leaf certificate (encoded as lowercase hex)**, enforces policy/rate limits, and issues an **X.509 SVID (LDI)**.
5. The MIS OOS securely delivers the SVID and Trust Bundle to the device via the authenticated TO2 channel.

### Factory Certificate Method (mTLS)

This method enables **certificate-based onboarding** using a **manufacturer-issued X.509 certificate** presented via **mutual TLS**.
It provides a **direct, low-latency** path for environments with end-to-end TLS.

**Bootstrap Method Identifier (URN):**
`urn:margo:bootstrap:factory-cert-mtls:v1`

**Purpose:**
Authenticate the device during enrollment by presenting a manufacturer-issued X.509 certificate via **mTLS**.

**Enrollment Subject Identifier (ESI):**
Implementations **MUST** derive the ESI as the **SHA-256 fingerprint of the DER-encoded leaf certificate** presented during the TLS handshake.

**`bootstrapCredential` object schema (`application/json`):**

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `method`| string | Y | **MUST** be `urn:margo:bootstrap:factory-cert-mtls:v1`. |
| `proof` | object or null | N | **MUST** be omitted (`null` or absent); the credential is conveyed by the mTLS client certificate. |

**Process Summary (informative):**

1. The device holds a manufacturer X.509 certificate and private key (ideally hardware-protected).
2. It establishes HTTPS to MIS using **mTLS**.
3. MIS validates the presented chain and revocation status against authorized manufacturer CAs.
4. Over the authenticated session, the device submits `POST /api/v1/identities` with CSR and `svid_profile_uri`.
5. MIS derives **ESI = SHA-256 fingerprint of leaf cert**, applies policy, and issues an **X.509 SVID (LDI)**.

### Factory Certificate Method (JWT Assertion)

This method enables **application-layer onboarding** using a **JWT assertion signed with the factory private key**, suitable when **end-to-end mTLS is not feasible** (for example, due to TLS-terminating proxies).

**Bootstrap Method Identifier (URN):**
`urn:margo:bootstrap:factory-cert-jwt:v1`

**Purpose:**
Support environments where direct mutual TLS is not possible due to intermediaries or TLS-terminating proxies.
This method uses a **Bootstrap Assertion** - a JWT signed with the device's **factory key** - to prove the **Physical Device Identity (PDI)** during initial enrollment with the MIS.

> For ongoing authentication by enrolled devices, see the **Client Authentication Assertion** used in the [JWT SVID Exchange Endpoint](#jwt-svid-exchange-endpoint).

**Enrollment Subject Identifier (ESI):**
Implementations **MUST** derive the ESI as the **SHA-256 fingerprint of the DER-encoded leaf certificate** contained in the JWT `x5c` header (`x5c[0]`).

**`bootstrapCredential` object schema (`application/json`):**

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `method` | string | Y | **MUST** be `urn:margo:bootstrap:factory-cert-jwt:v1`. |
| `proof` | object | Y | **MUST** contain `assertion`. |
| `proof.assertion` | string | Y | Compact **JWT** per [RFC 7519](https://datatracker.ietf.org/doc/html/rfc7519), signed with the factory private key. The signing algorithm **MUST** conform to [Cryptographic Requirements](#cryptographic-requirements). The JWS header **MUST** include `x5c` with the full certificate chain ([RFC 7517 §4.7](https://datatracker.ietf.org/doc/html/rfc7517#section-4.7)). |

#### JWT Assertion Structure <!-- omit from toc -->

- The assertion **MUST** be a JWT ([RFC 7519](https://datatracker.ietf.org/doc/html/rfc7519)) using **JWS Compact Serialization** (RFC 7515 §3.1).
- The signature **MUST** use `ES256` (ECDSA P-256) or `PS256` (RSA-PSS 3072), per [Cryptographic Requirements](#cryptographic-requirements).
- The JWS header **MUST** include `x5c` with the **complete** manufacturer chain; `x5c[0]` **MUST** be the device's factory leaf certificate.

**Header fields:**

| Header Parameter | Required | Description  |
| :--------------- | :------- | :----------- |
| `alg` | Y | **MUST** match the key type of the factory certificate (`ES256` for ECDSA P-256 or `PS256` for RSA-PSS 3072). Algorithms **MUST** conform to [Cryptographic Algorithm Requirements](#cryptographic-requirements). |
| `x5c` | Y | **MUST** contain the complete certificate chain, with the factory leaf certificate as the first entry, per [RFC 7517 §4.7](https://datatracker.ietf.org/doc/html/rfc7517#section-4.7). |

**Payload claims:**

| Claim | Required | Description |
| :---- | :------- | :---------- |
| `iss` | Y | **MUST** be `urn:margo:device:sha256:<lowercase-hex-fingerprint>` of `x5c[0]`. **Policy MUST NOT** be based on this value. |
| `sub` | Y | **MUST** equal `iss`. |
| `aud` | Y | **MUST** equal the full URL of the MIS enrollment endpoint. |
| `iat` | Y | Issued-at timestamp (seconds since UNIX epoch). |
| `nbf` | N | Optional "not before"; MIS **MUST** reject assertions not yet valid |
| `exp` | Y | Expiration; **MUST** be <= 5 minutes after `iat`. |
| `jti` | Y | Unique token ID; MIS **MUST** reject replays of the same `jti` |

**Process Summary (informative):**

1. Device generates SVID key pair and CSR.
2. Device creates a Bootstrap Assertion JWT signed with its factory key and embeds the full factory chain in `x5c`.
3. Device calls `POST /api/v1/identities` with CSR and `bootstrapCredential`.
4. MIS validates signature and manufacturer chain, enforces claims (`aud`, `exp`, `jti`), and verifies manufacturer authorization.
5. MIS derives **ESI = SHA-256 fingerprint of x5c[0]**, applies policy, and issues an **X.509 SVID (LDI)**.

> **Example (truncated):**
>
> ```text
> eyJhbGciOiJQUzI1NiIsIng1YyI6WyJNSUl...Il19.
> eyJpc3MiOiJ1cm46bWFyZ286ZGV2aWNlOnNoYTI1NjphYmNkZWY...IiwiYXVkIjoiaHR0cHM6Ly9taXMuZXhhbXBsZS5jb20vYXBpL3YxL2lkZW50aXRpZXMiLCJleHAiOjE2Nzc2MTE4MDEsImp0aSI6IjBhMDFiMzI1In0.
> SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c...
> ```

### IEEE 802.1AR Method

> **TODO:** This method is a placeholder. The required details have not yet been defined.

This method enables **hardware-anchored onboarding** using an [IEEE 802.1AR](https://standards.ieee.org/standard/802_1AR-2018.html) **Device Identity (DevID)** embedded in the device's secure element. It supports **standards-based authentication and enrollment** across vendors, ensuring interoperable and verifiable device identity without requiring manufacturer-specific extensions or external onboarding services.

All cryptographic operations and key usages defined by this method **MUST** comply with the [Cryptographic Requirements](#cryptographic-requirements).

## Appendix B: Error Responses (Normative)

This appendix defines the **mandatory error handling model** for all API endpoints introduced in this specification.
It ensures consistent error semantics, human readability, and machine parsability across all Margo Identity Service (MIS) and Trust Domain endpoints.

### Error Representation Format

All Margo-compliant services **MUST** return error details for any `4xx` or `5xx` HTTP status code as a **Problem Details JSON Object**, following [RFC 9457 - "Problem Details for HTTP APIs"](https://datatracker.ietf.org/doc/html/rfc9457).

For `429 Too Many Requests` responses, services **MUST** include a `Retry-After` response header (delta-seconds) indicating when the client may retry.

| Requirement | Normative Directive |
| :---------- | :------------------ |
| **Content-Type** | Error responses **MUST** set `Content-Type: application/problem+json`. |
| **Body Schema** | The response body **MUST** conform to the structure defined below. |
| **HTTP Status Synchronization** | The `status` field in the JSON body **MUST** match the actual HTTP status code. |

### Problem Details Object Schema

| Member | Type | Required | Description |
| :----- | :--- | :------- | :---------- |
| `type` | string (URI) | Y | Identifies the problem type. Used by clients to map to specific error categories. |
| `title` | string  | Y | Short, human-readable summary of the error. |
| `status` | integer | Y | The HTTP status code of the response. |
| `detail` | string | N | Developer-readable explanation of this specific problem instance. |
| `instance` | string (URI) | N | Unique URI reference identifying the specific error occurrence - e.g., correlation or audit ID. |

**Example (Generic Unauthorized Error):**

```json
{
  "type": "about:blank",
  "title": "Unauthorized",
  "status": 401,
  "detail": "The provided credential is invalid or expired."
}
```

### Error Type Conventions

Error `type` URIs fall into two categories.

1. **General HTTP Errors**

   - Use `type: "about:blank"`.
   - The `title` field **SHOULD** match the HTTP reason phrase.

     ```json
     {
       "type": "about:blank",
       "title": "Forbidden",
       "status": 403
     }
     ```

2. **Margo-Specific Protocol Errors**

   - Use absolute URIs under the Margo namespace (`https://margo.org/docs/errors/<error-code>`).
   - These identify standardized error classes across MIS implementations.

   | Condition | HTTP Status | `type` URI | `title` |
   | :-------- | :---------- | :--------- | :------ |
   | Unsupported bootstrap method | 422 | `https://margo.org/docs/errors/unsupported-method` | Unsupported Bootstrap Method  |
   | Unsupported SVID profile | 422 | `https://margo.org/docs/errors/unsupported-svid-profile` | Unsupported SVID Profile |
   | Enrollment or renewal rate limit exceeded | 429 | `https://margo.org/docs/errors/too-many-requests` | Too Many Requests |
   | Invalid revocation list format | 500 | `https://margo.org/docs/errors/revocation-format`| Revocation List Parsing Error |

### Error Handling for Specific APIs

Each API endpoint defined in this SUP **MUST** map validation and authorization failures to Problem Details responses.
The following table summarizes normative mappings.

| Endpoint | Error Condition | Status | Error Type | Required Action |
| :------- | :-------------- | :----- | :--------- | :-------------- |
| `POST /api/v1/identities` | Unknown `bootstrapCredential.method` | 422 | `unsupported-method` | Client **MUST** retry only with a supported method. |
| `POST /api/v1/identities` | Invalid or missing CSR | 400 | `about:blank` | Client **MAY** resubmit with a corrected CSR. |
| `POST /api/v1/identities` | Malformed JWT assertion or proof | 401 | `about:blank` | Client **MUST** regenerate a valid assertion. |
| `POST /api/v1/identities/{spiffeIdEncoded}/renewals` | Unsupported SVID profile | 422 | `unsupported-svid-profile` | Client **MUST** retry with a supported profile. |
| `POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid` | Audience or assertion invalid | 400 | `about:blank` | Client **MUST** correct request and retry. |
| Any endpoint | Authorization failed (credential invalid or expired) | 401 | `about:blank` | Client **MUST** re-authenticate and retry. |
| Any endpoint | Rate limit exceeded | 429 | `too-many-requests` | Client **SHOULD** apply backoff and alert operator. |
| `GET /api/v1/revocations` | JSON structure invalid | 500 | `revocation-format` | Server **SHOULD** log and repair malformed data. |

### Example - Unsupported Bootstrap Method

```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/problem+json

{
  "type": "https://margo.org/docs/errors/unsupported-method",
  "title": "Unsupported Bootstrap Method",
  "status": 422,
  "detail": "The bootstrap method 'urn:margo:bootstrap:custom-legacy:v1' is not supported by this server."
}
```

### Client Behavior Recommendations

Clients **MUST** implement the following behaviors to ensure consistent interoperability.

1. **Structured Error Mapping**

   - Clients **SHOULD** map known `type` URIs to internal enums for deterministic handling.
   - Unknown `type` URIs **MUST** be treated as generic errors using `status` and `title`.

2. **Retry Logic**

    - For recoverable errors (`429`), clients **MAY** retry after the indicated delay or per `Retry-After`.
    - For permanent errors (`422`, `400`), clients **MUST NOT** retry without correction.

3. **Logging and Auditability**

   - Clients **SHOULD** log the entire Problem Details object for diagnostics and auditing.
   - If `instance` is present, it **SHOULD** be included in all operator or support logs to enable cross-correlation with server records.

## Appendix C: OAuth2 and API Gateway Interoperability (Informative)

This appendix provides **informative guidance** for deployments that wish to integrate the **Margo Identity and Authorization Framework (MIAF)** with existing **OAuth 2.0-based** or **API gateway** infrastructures.

### Purpose and Context

While MIAF relies natively on **cryptographically verifiable identities** (SVIDs) for authentication and authorization, many enterprise environments already operate OAuth 2.0 Authorization Servers (AS) and API gateways for coarse-grained access control.
This appendix outlines how a deployment can:

1. **Map SVID-based identities** to OAuth 2.0 tokens for consumption by existing gateways or services.
2. **Delegate authorization decisions** to an OAuth 2.0 AS while maintaining MIAF as the underlying trust and identity authority.
3. **Provide interoperability** without requiring OAuth 2.0 to be implemented as part of the normative MIAF core.

### Integration Models

Three complementary integration models are recognized.
All models treat the **Margo Identity Service (MIS)** as the authoritative identity provider, with the OAuth 2.0 AS acting as a consumer or translator.

| Model | Description | Typical Use Case |
| :---- | :---------- | :--------------- |
| **1. Token Exchange Bridge** | The AS issues OAuth 2.0 access tokens based on a validated SVID or SPIFFE ID. MIS remains the root of trust. | Environments where API gateways or external services require OAuth 2.0 bearer tokens. |
| **2. Federated AS Validation** | The AS accepts X.509 or JWT SVIDs directly as client assertions per RFC 7523 / RFC 8705 and mints an access token with mapped claims. | Enterprise OAuth deployments using existing infrastructure but relying on MIAF identities for proof. |
| **3. Gateway Policy Mapping**  | An API gateway validates SVIDs locally using the Trust Bundle and maps the SPIFFE ID to roles, scopes, or policies. | Deployments that prefer local verification without introducing an external AS. |

All models rely on the same verification primitives defined in the normative specification: SVID validation against the Trust Bundle, and optional SPIFFE ID-to-policy mapping.

#### Model 1 - Token Exchange Bridge

In this model, the OAuth 2.0 AS exposes an [RFC 8693 **Token Exchange**](https://datatracker.ietf.org/doc/html/rfc8693) endpoint. An **Authorization Server (AS)** issues **OAuth 2.0 access tokens** based on a validated **SVID** presented by a client.
The **Margo Identity Service (MIS)** remains the trust root; the AS simply translates a verified SVID into a conventional OAuth 2.0 token for interoperability with existing gateways or services.

```mermaid
sequenceDiagram
    participant Device
    participant MIS
    participant AS
    participant RS as API Gateway / Resource Server

    Device->>MIS: Obtain X.509 or JWT SVID
    Device->>AS: POST /token (grant_type=urn:ietf:params:oauth:grant-type:token-exchange)<br/>(subject_token=<SVID>, subject_token_type=<type>)
    AS->>AS: Validate SVID using Trust Bundle
    AS-->>Device: 200 OK (Access Token)
    Device->>RS: HTTPS request Authorization: Bearer <token>
    RS->>AS: Introspect / verify token (per RFC 7662 / JWT validation)
```

> **Note:**
> Deployments that bridge SVID-based identities into OAuth 2.0 tokens may define internal claim mappings as needed for their authorization infrastructure. Such mappings are **deployment-specific** and **out of scope** for this specification.

##### Token Exchange Request

| Parameter | Required | Description |
| :-------- | :------- | :-----------|
| `grant_type` | Y | **MUST** be `urn:ietf:params:oauth:grant-type:token-exchange`. |
| `subject_token` | Y | The encoded SVID representing the requester. |
| `subject_token_type` | Y | **MUST** identify the format:<br>- `urn:margo:token-type:x509-svid`: base64-encoded PEM chain<br>- `urn:margo:token-type:jwt-svid`: compact JWT SVID<br>Additional values MAY be registered later. |
| `audience` | N | Target resource audience for the requested token. |
| `scope` | N | Optional scopes; AS policy determines allowed values. |

The AS **MUST** reject requests with unknown or unsupported `subject_token_type`.

##### Token Exchange Response

| Field | Description |
| :---- | :---------- |
| `access_token` | OAuth 2.0 access token (JWT or opaque).  |
| `issued_token_type` | Usually `urn:ietf:params:oauth:token-type:access_token`. |
| `token_type` | `"Bearer"`. |
| `expires_in` | Token lifetime in seconds; **SHOULD NOT** exceed the underlying SVID's validity. |

##### Validation and Security Considerations

- The AS **MUST** validate the SVID chain (for X.509) or signature (for JWT) against the **Trust Bundle**.
- The access token's `sub` claim **SHOULD** equal the SPIFFE ID of the validated SVID.
- Access-token lifetime **MUST NOT** exceed the remaining validity of the SVID.
- The AS **MUST** set `iss` to its own OAuth issuer identifier to avoid audience confusion.
- Token-exchange implementations **MUST NOT** bypass SVID validation or accept untrusted issuers.

#### Model 2 - Federated AS Validation

In this model, an **Authorization Server (AS)** directly accepts **SVIDs** for client authentication using standard OAuth 2.0 mechanisms such as **JWT Client Assertion** ([RFC 7523](https://datatracker.ietf.org/doc/html/rfc7523)) or **Mutual-TLS Client Authentication** ([RFC 8705](https://datatracker.ietf.org/doc/html/rfc8705)).
The AS validates the presented SVID against the **Trust Bundle** of the declared Trust Domain before issuing an access token.

```mermaid
sequenceDiagram
    participant Device
    participant AS
    participant RS as Resource Server / API Gateway

    Device->>AS: POST /token (client_assertion=<JWT SVID> or mTLS client auth)
    AS->>AS: Validate SVID chain or signature using Trust Bundle
    AS-->>Device: 200 OK (Access Token)
    Device->>RS: HTTPS request Authorization: Bearer <token>
    RS->>AS: Introspect / verify token (per RFC 7662 or local JWT validation)
```

##### Validation and Security Considerations

- The AS **MUST** validate the X.509 or JWT SVID using the corresponding Trust Bundle before issuing any OAuth token.
- The access token's `sub` claim **SHOULD** contain the verified SPIFFE ID.
- Access-token lifetime **MUST NOT** exceed the validity period of the SVID.
- The AS **MUST** set `iss` to its own issuer identifier and MUST NOT reuse the MIS identifier.
- Deployments **SHOULD** limit accepted issuers to known Trust Domains and log all SVID-based authentications.

#### Model 3 - Gateway Policy Mapping

In this model, an **API Gateway** or **Reverse Proxy** validates **X.509 or JWT SVIDs** directly using the **Trust Bundle**, without involving an Authorization Server.
After successful validation, the gateway maps the verified SPIFFE ID to local authorization constructs such as roles or scopes.

```mermaid
sequenceDiagram
    participant Device
    participant Gateway
    participant RS as Backend Service

    Device->>Gateway: HTTPS request (mTLS or JWT SVID)
    Gateway->>Gateway: Validate SVID against Trust Bundle
    Gateway->>RS: Forward request with mapped authorization context
    RS-->>Gateway: 200 OK
    Gateway-->>Device: 200 OK
```

##### Validation and Security Considerations

- Gateways **MUST** retrieve and cache the Trust Bundle from the MIS discovery document and refresh it regularly.
- Incoming SVIDs **MUST** be validated against the bundle for signature and expiry.
- Authorization mappings (e.g., SPIFFE ID → roles) **SHOULD** be deterministic and policy-driven.
- Gateways **MUST** reject expired or untrusted SVIDs and **SHOULD** log validation failures for audit purposes.
