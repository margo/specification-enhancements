# SUP-03: OCI Artifact Signing for Margo

| Field | Value |
|-------|-------|
| SUP ID | SUP-03 |
| Title | OCI Artifact Signing for Margo |
| Status | Draft |
| Category | Cat 2 — Enhancement |
| Created | 2025-03-15 |
| Revised | 2026-07-29 |

## Owner

@javatask — Andrii Melashchenko, Belden Inc.

## Summary

This proposal establishes digital signature requirements for Margo OCI artifacts, defining which artifacts **MUST** be signed, which actors sign and verify, and when verification occurs in the deployment lifecycle. It profiles the CNCF Notary Project specifications for Margo's Application Developer, WFM, and WFM Client actors. Cryptographic algorithm selection is deferred to operator security policy; Margo provides interoperability recommendations only.

## Reason for proposal

- The current Margo specification references PGP signing but provides no OCI-native storage model for signatures.
- No verification algorithm or workflow is defined — verifiers have no interoperable procedure to follow.
- No trust policy schema exists to express which keys are trusted for which registry scopes.
- No publisher-to-operator key distribution mechanism is specified.
- Industrial deployments face regulatory requirements (IEC 62443, EU Cyber Resilience Act) that mandate artifact integrity verification with auditable trust chains.

## Technical proposal

### 1. Normative references

This SUP adopts the following specifications by reference:

| Reference | Source |
|-----------|--------|
| Signature Specification | `notaryproject/specifications/specs/signature-specification.md` (main branch, accessed 2026-07-29) |
| COSE Signature Envelope | `notaryproject/specifications/specs/signature-envelope-cose.md` (main branch, accessed 2026-07-29) |
| JWS Signature Envelope | `notaryproject/specifications/specs/signature-envelope-jws.md` (main branch, accessed 2026-07-29) |
| Signing and Verification Workflow | `notaryproject/specifications/specs/signing-and-verification-workflow.md` (main branch, accessed 2026-07-29) |
| Trust Store and Trust Policy | `notaryproject/specifications/specs/trust-store-trust-policy.md` (main branch, accessed 2026-07-29) |
| OCI Distribution Specification | v1.1.0 |
| OCI Image Specification | v1.1.0 |
| RFC 9052 | CBOR Object Signing and Encryption (COSE): Structures and Process |
| RFC 7515 | JSON Web Signature (JWS) |
| RFC 3161 | Internet X.509 PKI Time-Stamp Protocol (TSP) |

Margo adopts these specifications in full. Where Margo narrows or extends, this SUP states the delta explicitly.

### 2. Signing obligations

#### 2.1 What MUST be signed

| Artifact | Signer | Requirement |
|----------|--------|-------------|
| Application Package manifest | Application Developer | **MUST** |
| Compose Archive manifest | Application Developer | **MUST** |
| Helm Chart OCI manifest | Application Developer | **MUST** |
| Container images published by the developer | Application Developer | **SHOULD** |

> **Note:** Artifacts from upstream third parties (base images, community charts) are outside the Application Developer's control. The **MUST** obligation applies only to artifacts the developer publishes to a Margo-compliant registry.

#### 2.2 Pre-publish self-verification

Application Developers **MUST** verify their own signatures before publishing artifacts to a registry. A failed self-verification **MUST** block publication. This ensures that signing toolchain misconfiguration is caught at the source rather than at deployment time.

### 3. Cryptographic recommendations

Margo does not mandate a specific cryptographic profile. Operator security policy governs algorithm selection for each deployment.

The following baseline is **RECOMMENDED** (informative, not normative):

| Parameter | Recommendation |
|-----------|---------------|
| RSA key size | 3072 bits minimum |
| ECDSA curve | P-256 or P-384 |
| Hash algorithm | SHA-256 minimum |
| Signature envelope | COSE_Sign1 for constrained devices; JWS acceptable |
| Signing scheme | `notary.x509` |
| Timestamping | **SHOULD** include RFC 3161 TSA counter-signature when signing certificate validity is shorter than the expected artifact lifetime |

**Normative interoperability requirement:** Verifiers **MUST** support both COSE and JWS signature envelopes. A verifier that rejects a valid signature solely because it uses a supported-but-not-preferred envelope format is non-conformant.

Operators **MAY** impose stricter requirements through deployment-specific trust policies. Customer security policy takes precedence over these recommendations.

### 4. Conformance actors and obligations

| Actor | Obligation | v1-alpha1 | GA |
|-------|-----------|-----------|-----|
| Application Developer | Sign artifacts per §2 | **SHOULD** | **MUST** |
| WFM | Verify signatures when trust policy level ≠ `skip` | **SHOULD** | **MUST** |
| WFM Client | Verify signatures before deploying workloads | **SHOULD** | **MUST** |

**Default behavior at v1-alpha1:** When no trust policy is configured, verification is not performed. Unsigned artifacts remain deployable. This permits incremental adoption.

**Default behavior at GA:** When no trust policy is configured, the default is `strict` — unsigned artifacts are rejected. Exact enforcement semantics follow the Notary Project `trust-store-trust-policy.md` specification.

### 5. Publisher signing material

#### 5.1 Definition

Application Developers **MUST** provide publisher signing material to enable downstream verification of their artifacts. This material allows WFM and WFM Client actors to establish trust in the developer's signing identity.

> **Terminology note:** This SUP uses the term "publisher signing material." The term "trust bundle" is reserved for MIAF SPIFFE Trust Bundles and **MUST NOT** be used in this context.

#### 5.2 Required contents

Publisher signing material **MUST** include:

1. **Root CA certificate(s)** — PEM-encoded X.509 v3 certificates anchoring the signing certificate chain.
2. **Trust policy template** — A document conforming to the Notary Project `trust-store-trust-policy.md` schema, with `registryScopes` left as a placeholder for operator-specific configuration.
3. **README** — Human-readable installation and configuration instructions.

#### 5.3 Distribution

Distribution of publisher signing material is an operator responsibility. Mechanisms are deployment-specific and include:

- Configuration management systems (Ansible, Puppet, etc.)
- WFM vendor administrative UI
- Manual provisioning via secure out-of-band channel

A future SUP **MAY** define automated distribution over the Margo Management Interface.

### 6. Known limitations

1. **Mutable tag references between tiers.** Signing binds a signature to a single manifest digest. However, references between Application Package → Component → Container Image currently use OCI tags, which are mutable. Digest pinning across the artifact tier hierarchy is deferred to a future SUP.

2. **WFM Client cannot interpose on image pulls.** On Kubernetes, the kubelet pulls container images; on Compose, the container engine pulls them. The WFM Client can verify archive artifacts it receives directly but cannot enforce signature verification on images at runtime without a device-level admission controller. This is an inherent platform constraint.

3. **No automated publisher signing material distribution.** Per §5.3, distribution is operator-managed. Automated provisioning over the Management Interface is out of scope for this SUP.

### 7. Affected specification files

| File | Change |
|------|--------|
| `system-design/specification/applications/application-registry.md` | Add cross-reference to signing profile |
| `system-design/specification/security/signing-profile.md` (NEW) | Normative Margo signing profile document |

No LinkML schema files are modified by this proposal.

### 8. Schema change report

None. Digital signatures are an OCI-layer operational concern stored as OCI artifacts referencing signed manifests. No LinkML schema changes are required. This proposal is fully backward compatible with the existing Margo data model.

## Alternatives considered

### Alternative A: Cosign / Sigstore

Rejected. Cosign relies on an online transparency log (Rekor) and short-lived certificates via Fulcio/OIDC. Industrial edge deployments cannot guarantee connectivity to public infrastructure, and the OIDC identity model does not map to X.509 PKI trust chains required by IEC 62443.

### Alternative B: In-band signature fields in LinkML schema

Rejected. Embedding signature data as schema fields creates a circular dependency — the signature covers the manifest content, but adding the signature to the manifest changes the content. This approach also couples signing tightly to the Margo schema, preventing use of standard OCI signing toolchains.

### Alternative C: PGP status quo

Rejected. PGP has no native OCI storage model (signatures cannot be stored as referrers in an OCI registry), no standardized timestamping, and no practical revocation mechanism. Tooling support for PGP in container ecosystems is minimal.

### Alternative D: Signatures embedded in OCI annotations

Rejected. Writing signature data into manifest annotations mutates the manifest digest, creating a chicken-and-egg problem. Annotation size limits also restrict signature payload capacity.

## Companion documents

- [SUP-03 Implementation Guide](sup-03-oci-signature-guide.md) — Non-normative guidance with worked examples for Application Developers, WFM vendors, and operators.

## References

- IETF RFC 2119 — Key words for use in RFCs to Indicate Requirement Levels
- IETF RFC 3161 — Internet X.509 PKI Time-Stamp Protocol (TSP)
- IETF RFC 7515 — JSON Web Signature (JWS)
- IETF RFC 9052 — CBOR Object Signing and Encryption (COSE): Structures and Process
- CNCF Notary Project Specifications — https://github.com/notaryproject/specifications
- OCI Distribution Specification v1.1.0 — https://github.com/opencontainers/distribution-spec
- OCI Image Specification v1.1.0 — https://github.com/opencontainers/image-spec

## Rejection reason

> Not applicable — SUP is in active development.

---

*Prepared by Andrii Melashchenko (Belden Inc.), 2026-07-29.*
