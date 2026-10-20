# SUP-03: OCI Artifact Signing for Margo

| Field | Value |
|-------|-------|
| SUP ID | SUP-03 |
| Title | OCI Artifact Signing for Margo |
| Status | Draft |
| Category | Cat 2 — Enhancement |
| Created | 2025-03-15 |
| Revised | 2026-10-19 |
| Revision | v1.1 |
| Depends on | SUP-00 "The Helm Way" (PR #67, MERGED), SUP-01 "Compose OCI" (PR #68, MERGED) |
| Anchor feature | margo/specification issue #138 |

## Owner

@javatask — Andrii Melashchenko, Belden Inc.

## Changes in v1.1

This revision responds to the PR #100 review (phil-abb, samuel-greising). Reviewers checking only the delta can read the sections below.

| # | Section | Change |
|---|---|---|
| 1 | §2 Signing obligations | Signing obligation recast as an **artifact property** ("a Margo application artifact MUST be signed; its signature MUST verify"), actor-agnostic and conformance-testable; developer pre-publish self-verify lowered MUST → SHOULD. |
| 2 | §4 Verification, item 6 | Expired-certificate rule restated as three explicit cases (valid → accept; expired + RFC 3161 timestamp → accept; expired + no timestamp → **reject**), removing the prior double-negative. |
| 3 | §6 Conformance actors | Added rationale for the WFM vs WFM Client requirement asymmetry (placement + staged adoption); Known Limitation 2 cited as the bound on client-side enforcement, not the cause. |
| 4 | §5, §7 item 3, Requirements alignment | Trust-material distribution reframed from "out of scope" to a normative, payload-observable boundary: a conformant WFM MUST NOT include publisher signing material or trust policy in any Management Interface payload. Mechanism left to the device vendor. |
| 5 | Alternatives considered → Cosign | *(pending owner rewrite)* rejection rationale to rest on identity binding + scheme fragmentation rather than the inaccurate online-only claim. |
| 6 | §5 Publisher signing material | Clarified that the trust-policy template is an onboarding input; the operator composes one consolidated fleet trust policy that the verifier evaluates. |
| 7 | §7 Known limitations | Added item 4: no machine-readable "declared-unsigned" mechanism (deferred). Companion guide gains a "Third-party / unsigned artifacts" section and a fleet-scale provisioning note. |

## Summary

This SUP defines the digital signature requirements for Margo OCI artifacts. It adopts the CNCF Notary Project specifications (v1.1.0) by reference as the signing and verification framework, and specifies which Margo actors sign which artifacts, how verification is performed, and how publisher signing material is distributed to enable trust.

## Reason for proposal

- The Margo specification currently has no OCI-native signing model for application artifacts.
- No standardized verification workflow exists for WFM or WFM Client implementations.
- No trust policy framework defines how operators configure signature acceptance criteria.
- No mechanism exists for Application Developers to distribute signing material to downstream consumers.
- Regulatory frameworks including the EU Cyber Resilience Act (CRA) and IEC 62443 require software integrity verification in industrial environments.

## Requirements alignment acknowledgement

This SUP addresses the following features from the TWG Feature Backlog:

- [margo/specification issue #138](https://github.com/margo/specification/issues/138) — OCI artifact signing and verification
- [margo/specification issue #36](https://github.com/margo/specification/issues/36) — Application package integrity

| Acceptance criterion | SUP-03 coverage |
|---|---|
| Signing model defined for OCI artifacts | §2 Signing obligations |
| Verification workflow for WFM | §4 Verification |
| Trust policy configuration | §4 adopted by reference from Notary Project |
| Publisher signing material format | §5 Publisher signing material |
| Conformance obligations per actor | §6 Conformance actors and obligations |

**Scope boundary:** Publisher signing material and trust policy are provisioned over an operator / device-management channel distinct from the Margo Management Interface; the device-management protocol itself is left to the device vendor (see §5 and Known Limitation 3). Digest pinning across artifact tiers is deferred to a future SUP.

## Technical proposal

### 1. Normative references

Margo adopts the following CNCF Notary Project specifications (v1.1.0) in full. Where Margo narrows or extends, this SUP states the delta explicitly.

**Release tracking.** The Notary Project versions by release, so this SUP pins to the tag **v1.1.0** — the latest released tag at the time of writing. All Notary references in this SUP and its companion guide resolve against `v1.1.0`, not `main`. Unreleased `main` drift (for example the renaming of `trustpolicy.json` to `trustpolicy.oci.json`) is out of scope until it appears in a tagged release; a future revision of this SUP moves the pin deliberately after reviewing the release notes. Margo does not track Notary `main`.

| Specification | Reference |
|---|---|
| Signature Specification | [signature-specification.md](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/signature-specification.md) |
| COSE Signature Envelope | [signature-envelope-cose.md](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/signature-envelope-cose.md) |
| JWS Signature Envelope | [signature-envelope-jws.md](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/signature-envelope-jws.md) |
| Signing and Verification Workflow | [signing-and-verification-workflow.md](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/signing-and-verification-workflow.md) |
| Trust Store and Trust Policy | [trust-store-trust-policy.md](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/trust-store-trust-policy.md) |
| Signing Scheme | [signing-scheme.md](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/signing-scheme.md) |

Additional normative references:

| Specification | Reference |
|---|---|
| OCI Distribution Specification | v1.1.0 |
| OCI Image Specification | v1.1.0 |
| RFC 9052 | CBOR Object Signing and Encryption (COSE): Structures and Process |
| RFC 7515 | JSON Web Signature (JWS) |
| RFC 3161 | Internet X.509 PKI Time-Stamp Protocol |
| RFC 5280 | Internet X.509 PKI Certificate and CRL Profile |

### 2. Signing obligations

| Artifact | Signer | Requirement |
|---|---|---|
| Application Package manifest | Application Developer | **MUST** sign |
| Compose Archive manifest | Application Developer | **MUST** sign |
| Helm Chart OCI manifest | Application Developer | **MUST** sign |
| Container images published by the developer | Application Developer | **SHOULD** sign |

A Margo application artifact **MUST** be signed, and its signature **MUST** verify against the configured trust policy, subject to the adoption staging in §6 (SHOULD in v1-alpha1, MUST at GA; where no trust policy is configured for a registry scope, unsigned artifacts remain deployable during the transition, per §6). This obligation is a property of the artifact, verifiable by a conformance suite that takes the artifact as input — it is independent of who signed it or when. The signing obligation applies only to artifacts the Application Developer directly publishes to a registry. Upstream third-party container images are outside the scope of this SUP; see the companion guide, "Third-party / unsigned artifacts".

Application Developers **SHOULD** verify their own signatures before publishing a signed artifact to a registry. This is a pre-publish good practice, not a conformance point, because no Margo actor can observe it.

### 3. Cryptographic profile

This section defines a normative mandatory-to-implement (MTI) algorithm set so that any two conformant Margo implementations can verify each other's signatures. The set is a **deliberate narrowing** to match the MIAF cryptographic baseline ([MIAF §3 "Cryptographic requirements"](../../completed/margo-identity-and-authorization-framework.md)): a Margo device that already implements MIAF for identity should not carry a second, lower cryptographic floor for the supply chain. This is an alignment of algorithms only — it does **NOT** claim that MIAF governs artifact-signing keys, which are issued by the publisher's code-signing PKI, not by a MIAF Trust Domain.

Mandatory-to-implement algorithms:

| Algorithm | Requirement |
|---|---|
| **ES256** (ECDSA P-256 + SHA-256) | Verifiers **MUST** implement. |
| **PS256** (RSA-PSS, modulus ≥ 3072 bits, + SHA-256) | Verifiers **MUST** implement. |

- Verifiers **MUST** implement both ES256 and PS256, because a verifier cannot know in advance which algorithm a publisher will use.
- Signers **MUST** use at least one of ES256 or PS256.
- **RS256** (RSA PKCS#1 v1.5) **MUST NOT** be used for Margo artifact signatures.
- Verifiers **MUST** support both the COSE and JWS signature envelopes.

Broader parameters are RECOMMENDED (informative), not mandated:

| Parameter | Recommendation |
|---|---|
| ECDSA curve | P-256 (MTI) or P-384 |
| RSA key size | Larger than the 3072-bit floor where operator policy requires it |
| Signing scheme | notary.x509 |
| Timestamping | Include an RFC 3161 TSA response when the certificate validity period is shorter than the expected artifact lifetime |

### 4. Verification

The Workload Fleet Manager (WFM) **MUST** verify artifact signatures when the configured trust policy verification level is not `skip`.

The WFM Client **SHOULD** verify artifact signatures before deploying workloads to the device.

Verification requirements:

1. Verification **MUST** be performed against the target artifact's OCI manifest digest.
2. Verifiers **MUST** discover signatures via the OCI Referrers API, filtered on `artifactType=application/vnd.cncf.notary.signature`.
3. Certificate path validation **MUST** conform to RFC 5280.
4. The leaf certificate **MUST** carry the Key Usage `digitalSignature`. The Extended Key Usage `id-kp-codeSigning` is OPTIONAL: the leaf certificate **MAY** carry it, and where an EKU extension is present it **MUST NOT** contain `anyExtendedKeyUsage`, `id-kp-serverAuth`, `id-kp-clientAuth`, `id-kp-emailProtection`, or `id-kp-timeStamping`. This matches the Notary Project v1.1.0 [signature-specification.md](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/signature-specification.md) leaf-certificate requirements, which mandate `digitalSignature` but do not require `id-kp-codeSigning`. Margo does not add a delta here; operators who wish to require `id-kp-codeSigning` express that through trust-policy configuration, not through this normative baseline.
5. Signature validity binds to the manifest digest, not the registry path. A signature remains valid after the artifact is mirrored to a different registry.
6. Certificate expiry is evaluated as three explicit cases: (a) a signing certificate valid at verification time **MUST** be accepted; (b) an expired signing certificate **MUST** be accepted when an embedded RFC 3161 timestamp proves the signature was created before the certificate's `notAfter` time; (c) an expired signing certificate with no such timestamp **MUST** be rejected. This is the `strict`-level baseline; per-level modulation (for example `permissive` accept-with-log) is adopted by reference per item 7 and detailed in the companion guide §3.2.
7. Per-level verification semantics (`strict`, `permissive`, `audit`, `skip`) are adopted by reference from the Notary Project [trust-store-trust-policy.md](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/trust-store-trust-policy.md).

### 5. Publisher signing material

Publisher signing material is the set of artifacts an Application Developer provides to enable downstream signature verification.

Publisher signing material **MUST** include:

1. **Root CA certificate(s)** — PEM-encoded X.509 v3 certificates forming the trust anchor for the signing certificate chain.
2. **Trust policy template** — A trust policy document conforming to the Notary Project [trust-store-trust-policy.md](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/trust-store-trust-policy.md) schema, with `registryScopes` left as a placeholder for operator customization.
3. **README** — Human-readable installation instructions.

Publisher signing material and the operator trust policy are provisioned over an operator or device-management channel — firmware, device onboarding, and the device-owner PKI — and **MUST NOT** be delivered to a verifier as part of any Margo Management Interface (workload-plane) payload. A conformant WFM therefore **MUST NOT** include publisher signing material or trust policy in the Desired State or any other Management Interface message it sends to a device; a conformance suite can check this by inspecting Management Interface payloads. The rationale is separation of planes: the party that tells a device what to run should not also be the party that tells it what to trust, because collapsing the two would let a compromised or mistaken WFM make a bad artifact appear authentic. The device-management protocol that carries the trust material is left to the device vendor; this SUP states the boundary, not the mechanism. The initial root of trust is ideally established in firmware or onboarding (an immutable, out-of-band anchor the WFM cannot reach or overwrite), while the operational trust policy — which publishers an operator accepts — flows over that same device-management channel over the device's life and is therefore not frozen in firmware.

**Terminology note:** This SUP uses the term "publisher signing material" exclusively. The term "trust bundle" is reserved for MIAF SPIFFE Trust Bundles to avoid ambiguity.

The trust policy template is an **onboarding input to the operator**, not a per-application policy a verifier loads at verification time. For each publisher, the operator reviews the root CA and template once at onboarding and composes a **single consolidated trust policy for the fleet**. The verifier evaluates that one consolidated policy — not N per-application templates. This avoids an accumulation of independent active policies and keeps a single, uniform trust decision across the fleet.

### 6. Conformance actors and obligations

| Actor | Obligation | v1-alpha1 (transition) | GA specification release |
|---|---|---|---|
| Application Developer | Sign artifacts per §2 | **SHOULD** | **MUST** |
| WFM | Verify signatures when trust policy level ≠ `skip` | **SHOULD** | **MUST** |
| WFM Client | Verify signatures before deploying workloads | **MAY** | **SHOULD** |

When no trust policy is configured for a given registry scope, verification is not performed. Unsigned artifacts remain deployable. This default permits incremental adoption during the transition period.

The stronger verb on the WFM relative to the WFM Client is deliberate, for two reasons. First, **placement**: the WFM resolves and hands down the artifact set, making it the natural fleet-wide enforcement point, so it carries the stronger obligation. Second, **staged adoption**: resource-constrained devices receive the softer verb during the transition period and tighten at GA, which is the SHOULD→MUST staging already expressed in the table above. The WFM Client can and should verify what it pulls directly; Known Limitation 2 bounds only its ability to enforce verification on the container engine's own image pulls, and is the bound on client-side end-to-end enforcement — not the cause of the verb asymmetry.

### 7. Known limitations

1. **Mutable tag references between tiers.** Signing binds to a single manifest digest, but references between Application Package, Component, and Container Image tiers use mutable OCI tags. A consumer verifying an Application Package signature has no cryptographic assurance that referenced container images have not changed. Digest pinning is deferred to a future SUP.

2. **WFM Client cannot interpose on image pulls.** The container engine (e.g., kubelet, podman) pulls container images directly. The WFM Client cannot enforce signature verification on those pulls without a device-level admission controller, which is outside Margo's scope.

3. **Publisher signing material distribution is a device-management concern.** Publisher signing material and trust policy are provisioned over an operator / device-management channel distinct from the Margo Management Interface (see §5). This SUP defines the plane boundary but not the device-management protocol, which is left to the device vendor.

4. **No machine-readable "declared-unsigned" mechanism.** There is no standardized way for an Application Developer to declare that a deployment intentionally contains unsigned third-party artifacts, so an operator must handle them through trust-policy exceptions (see the companion guide, "Third-party / unsigned artifacts"). A declarative mechanism is deferred to a future revision.

### 8. Affected specification files

| File | Change |
|---|---|
| `system-design/specification/applications/application-registry.md` | Add cross-reference to the signing profile |
| `system-design/specification/security/signing-profile.md` (NEW) | Normative Margo signing profile |

No LinkML schema changes are required.

The removal of the legacy PGP signing prose and the `keyLocation` field is **owned by SUP-01** ("Compose OCI", PR #68), which replaces `ComponentProperties` with `repository`/`revision`/`wait`/`timeout`. SUP-03 does not re-claim those edits and assumes they land with SUP-01's integration. If SUP-03 is integrated before SUP-01's edits reach the specification, confirm the PGP/`keyLocation` state with the docs owners rather than editing those passages here.

## Alternatives considered

**Cosign / Sigstore.** Sigstore's keyless signing model requires an online transparency log (Rekor) and OIDC identity binding via Fulcio. These dependencies are unsuitable for air-gapped operational technology environments where persistent internet connectivity cannot be assumed.

**In-band signature fields in LinkML.** Embedding signature data as schema fields creates a circular dependency: the signature covers the manifest content, but adding the signature to the manifest changes the content that was signed.

**PGP / OpenPGP.** PGP lacks an OCI storage model, has no standardized timestamping mechanism, provides no practical certificate revocation, and has minimal container ecosystem tooling support.

**OCI manifest annotations.** Writing signature data into manifest annotations mutates the manifest digest, invalidating any signature computed over the original content.

## Companion documents

- [SUP-03 Implementation Guide](sup-03-oci-signature-guide.md) — Non-normative guidance with worked examples.

## References

- IETF RFC 2119 — Key words for use in RFCs to Indicate Requirement Levels
- IETF RFC 3161 — Internet X.509 Public Key Infrastructure Time-Stamp Protocol
- IETF RFC 5280 — Internet X.509 PKI Certificate and CRL Profile
- IETF RFC 7515 — JSON Web Signature (JWS)
- IETF RFC 9052 — CBOR Object Signing and Encryption (COSE): Structures and Process
- CNCF Notary Project Specifications v1.1.0 — <https://github.com/notaryproject/specifications/tree/v1.1.0>
- OCI Distribution Specification v1.1.0
- OCI Image Specification v1.1.0

## Rejection reason

> Not applicable — SUP is in active development.

---

*Prepared by Andrii Melashchenko (Belden Inc.), 2026-10-19.*
