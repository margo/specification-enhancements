# SUP-03: OCI Artifact Signing for Margo

| Field | Value |
|-------|-------|
| SUP ID | SUP-03 |
| Title | OCI Artifact Signing for Margo |
| Status | Draft |
| Category | Cat 2 — Enhancement |
| Created | 2025-03-15 |
| Revised | 2026-08-21 |
| Depends on | SUP-00 "The Helm Way" (PR #67, MERGED), SUP-01 "Compose OCI" (PR #68, MERGED) |
| Anchor feature | margo/specification issue #138 |

## Owner

@javatask — Andrii Melashchenko, Belden Inc.

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

**Out of scope:** Automated distribution of publisher signing material over the Management Interface. Distribution is operator-managed (see §5). Digest pinning across artifact tiers is deferred to a future SUP.

## Technical proposal

### 1. Normative references

Margo adopts the following CNCF Notary Project specifications (v1.1.0) in full. Where Margo narrows or extends, this SUP states the delta explicitly.

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

**MUST** applies only to artifacts the Application Developer directly publishes to a registry. Upstream third-party container images are outside the scope of this SUP.

Application Developers **MUST** verify their own signatures before publishing a signed artifact to a registry.

### 3. Cryptographic recommendations

Margo does not mandate a specific cryptographic profile. Operator security policy governs algorithm and key-size selection.

The following baseline is RECOMMENDED (informative):

| Parameter | Recommendation |
|---|---|
| RSA key size | 3072 bits minimum |
| ECDSA curve | P-256 or P-384 |
| Hash algorithm | SHA-256 minimum |
| Signature envelope | COSE_Sign1 or JWS |
| Signing scheme | notary.x509 |
| Timestamping | Include RFC 3161 TSA response when certificate validity period is shorter than expected artifact lifetime |

Normative requirement: Verifiers **MUST** support both COSE and JWS signature envelopes.

### 4. Verification

The Workload Fleet Manager (WFM) **MUST** verify artifact signatures when the configured trust policy verification level is not `skip`.

The WFM Client **SHOULD** verify artifact signatures before deploying workloads to the device.

Verification requirements:

1. Verification **MUST** be performed against the target artifact's OCI manifest digest.
2. Verifiers **MUST** discover signatures via the OCI Referrers API, filtered on `artifactType=application/vnd.cncf.notary.signature`.
3. Certificate path validation **MUST** conform to RFC 5280.
4. The leaf certificate **MUST** carry the Key Usage `digitalSignature` and the Extended Key Usage `id-kp-codeSigning`.
5. Signature validity binds to the manifest digest, not the registry path. A signature remains valid after the artifact is mirrored to a different registry.
6. An expired signing certificate **MUST** be accepted only if an embedded RFC 3161 timestamp proves the signature was created before the certificate's `notAfter` time.
7. Per-level verification semantics (`strict`, `permissive`, `audit`, `skip`) are adopted by reference from the Notary Project [trust-store-trust-policy.md](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/trust-store-trust-policy.md).

### 5. Publisher signing material

Publisher signing material is the set of artifacts an Application Developer provides to enable downstream signature verification.

Publisher signing material **MUST** include:

1. **Root CA certificate(s)** — PEM-encoded X.509 v3 certificates forming the trust anchor for the signing certificate chain.
2. **Trust policy template** — A trust policy document conforming to the Notary Project [trust-store-trust-policy.md](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/trust-store-trust-policy.md) schema, with `registryScopes` left as a placeholder for operator customization.
3. **README** — Human-readable installation instructions.

Distribution of publisher signing material to WFM and WFM Client instances is the operator's responsibility. Mechanisms include configuration management tooling, WFM administrative interfaces, and manual provisioning. Automated distribution over the Margo Management Interface is out of scope.

**Terminology note:** This SUP uses the term "publisher signing material" exclusively. The term "trust bundle" is reserved for MIAF SPIFFE Trust Bundles to avoid ambiguity.

### 6. Conformance actors and obligations

| Actor | Obligation | Current (Phase 3 Approved) | GA specification release |
|---|---|---|---|
| Application Developer | Sign artifacts per §2 | **SHOULD** | **MUST** |
| WFM | Verify signatures when trust policy level ≠ `skip` | **SHOULD** | **MUST** |
| WFM Client | Verify signatures before deploying workloads | **MAY** | **SHOULD** |

When no trust policy is configured for a given registry scope, verification is not performed. Unsigned artifacts remain deployable. This default permits incremental adoption during the transition period.

### 7. Known limitations

1. **Mutable tag references between tiers.** Signing binds to a single manifest digest, but references between Application Package, Component, and Container Image tiers use mutable OCI tags. A consumer verifying an Application Package signature has no cryptographic assurance that referenced container images have not changed. Digest pinning is deferred to a future SUP.

2. **WFM Client cannot interpose on image pulls.** The container engine (e.g., kubelet, podman) pulls container images directly. The WFM Client cannot enforce signature verification on those pulls without a device-level admission controller, which is outside Margo's scope.

3. **No automated publisher signing material distribution.** Distribution of signing material is operator-managed. Automated provisioning over the Management Interface is out of scope for this SUP.

### 8. Affected specification files

| File | Change |
|---|---|
| `system-design/specification/applications/application-registry.md` | Add cross-reference to the signing profile |
| `system-design/specification/security/signing-profile.md` (NEW) | Normative Margo signing profile |

No LinkML schema changes are required.

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

*Prepared by Andrii Melashchenko (Belden Inc.), 2026-08-21.*
