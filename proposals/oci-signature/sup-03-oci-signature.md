# SUP-03: OCI Artifact Signing for Margo

| Field | Value |
|-------|-------|
| SUP ID | SUP-03 |
| Title | OCI Artifact Signing for Margo |
| Status | Draft |
| Category | Cat 2 — Enhancement |
| Created | 2025-03-15 |
| Revised | 2026-07-29 |
| Supersedes | SUP-03 v1 (see "Changes in v2" below) |
| Depends on | SUP-00 "The Helm Way" (`margo/specification-enhancements` PR #67, MERGED) — defines the Helm Chart OCI manifest that §2.1 obligates signing for. SUP-01 "Compose OCI" (`margo/specification-enhancements` PR #68, MERGED) — defines the Compose Archive manifest that §2.1 obligates signing for. |
| Anchor feature | `margo/specification` issue #138 — "Define the Margo application image and component signing strategy" (open). Related: issue #36 — "Signing app packages" (open). |

## Owner

@javatask — Andrii Melashchenko, Belden Inc.

## Summary

This proposal establishes digital signature requirements for Margo OCI artifacts, defining which artifacts **MUST** be signed, which actors sign and verify, and when verification occurs in the deployment lifecycle. It profiles the CNCF Notary Project specifications for Margo's Application Developer, WFM, and WFM Client actors. Cryptographic algorithm selection is deferred to operator security policy; Margo provides interoperability recommendations only.

## Changes in v2 (informative)

Commit `9829693` rewrote this SUP as a focused signing proposal that adopts the CNCF Notary Project specifications by reference, replacing a prior version that defined signing mechanics inline. This pass (2026-07-29 revision) makes further corrections found during review.

| # | Change | Why |
|---|--------|-----|
| 1 | Adopt Notary Project specifications by reference (signature format, envelopes, verification workflow, trust policy) instead of defining an inline signing mechanism | The prior version's inline mechanism duplicated work an existing CNCF specification already does, and coupled Margo conformance to a bespoke format no external toolchain could verify. |
| 2 | Remove the superseded inline mechanism | A direct consequence of Change 1 — once the Notary Project specs are adopted by reference, the inline mechanism they replace has no normative role. |
| 3 | Purify the normative spec of product names (Notation CLI, `notation-go`) that v1's ballot text carried directly in its technical-proposal sections, achieving this SUP's zero-product-names-in-normative-text discipline. The companion guide already existed as a separate file; this change moved implementation detail that had leaked into the spec itself into that existing guide. | Keeps product names, CLI invocations, and PKI deployment advice out of the ballot text, matching this SUP's zero-product-names-in-normative-text discipline. |
| 4 | Re-pin the Notary Project references from "main branch, accessed 2026-07-29" to the released tag `v1.1.0` | A moving branch reference under ballot text is not a stable normative citation; `main` has since diverged from `v1.1.0` on exactly the files this SUP leans on. |
| 5 | Add a normative §6 Verification section | The guide's verification walkthrough pointed at "SUP-03 §6" for the normative definition, but no such section existed — the verification procedure was undefined in the ballot text itself. |
| 6 | Re-key the §4 conformance ladder from a nonexistent `v1-alpha1` milestone to Margo's actual Phase/GA vocabulary | `v1-alpha1` does not appear anywhere in Margo's release-versioning scheme; the ladder was built against a milestone label that was never adopted. |
| 7 | Add the `## Requirements alignment acknowledgement` section required by the SUP template | This section was missing; the SUP template requires it between "Reason for proposal" and "Technical proposal," with linked backlog features and an explicit out-of-scope statement. (v1 had carried a differently-scoped version of this section, not linked to a specific backlog issue; it was dropped in the v2 rewrite and is restored here in improved, issue-anchored form.) |

## Reason for proposal

- The current Margo specification references PGP signing but provides no OCI-native storage model for signatures.
- No verification algorithm or workflow is defined — verifiers have no interoperable procedure to follow.
- No trust policy schema exists to express which keys are trusted for which registry scopes.
- No publisher-to-operator key distribution mechanism is specified.
- Industrial deployments face a growing compliance surface: the EU Cyber Resilience Act obligates manufacturers to protect artifact and configuration integrity against unauthorised modification (Annex I Part I, point 2(f)) and to run a documented vulnerability-handling process including an SBOM (Annex I Part II), while the voluntary IEC 62443 series defines software-integrity verification as a component-level capability (see Guide §9.1). Neither instrument names a specific signing mechanism; a cryptographically verifiable, auditable signature chain is an architecturally defensible way to demonstrate the outcomes both call for.

## Requirements alignment acknowledgement

This SUP is scoped against `margo/specification` issue #138, "Define the Margo application image and component signing strategy" (open, opened 2026-01-22), and its related issue #36, "Signing app packages" (open). Issue #138 states five acceptance criteria; this SUP addresses them as follows:

| # | Acceptance criterion | Status | Where |
|---|----------------------|--------|-------|
| 1 | Define how application components are signed within Margo — app package / Helm chart / compose packages | Addressed | §2.1 |
| 2 | Define how OCI images are signed within Margo | Partially addressed | §2.1, `SHOULD` tier for developer-published images; third-party upstream images are explicitly out of scope (§2.1 note) |
| 3 | Define how WFMs and Devices are able to verify the signature during application deployment | Addressed | §4, §6 |
| 4 | Define, if necessary, any Margo infrastructure required to enable this verification between vendors | Partially addressed | §5 defines publisher signing material contents; automated distribution over the Management Interface is deferred to a future SUP (§5.3, Known Limitation 3) |
| 5 | Describe how these signatures live on even if the end user replicates the artifacts to a local repository | Addressed | §6 — signatures bind to the target manifest digest, not to registry hostname or repository path |

**Out of scope for this SUP**, agreed as such against issue #138:

- Digest pinning across the artifact tier hierarchy (Application Package → Component → Container Image), which currently uses mutable tags between tiers — deferred to a future SUP (Known Limitation 1).
- Runtime interposition on image pulls (kubelet, container engine) — an inherent platform constraint, not a gap this SUP can close (Known Limitation 2).
- Automated publisher signing material distribution over the Margo Management Interface — deferred to a future SUP (Known Limitation 3, §5.3).
- Mandating a specific cryptographic algorithm profile — deferred to operator security policy (§3).

## Technical proposal

### 1. Normative references

This SUP adopts the following specifications by reference:

| Reference | Source |
|-----------|--------|
| Signature Specification | `notaryproject/specifications/specs/signature-specification.md` (tag `v1.1.0`) |
| COSE Signature Envelope | `notaryproject/specifications/specs/signature-envelope-cose.md` (tag `v1.1.0`) |
| JWS Signature Envelope | `notaryproject/specifications/specs/signature-envelope-jws.md` (tag `v1.1.0`) |
| Signing and Verification Workflow | `notaryproject/specifications/specs/signing-and-verification-workflow.md` (tag `v1.1.0`) |
| Trust Store and Trust Policy | `notaryproject/specifications/specs/trust-store-trust-policy.md` (tag `v1.1.0`) |
| OCI Distribution Specification | v1.1.0 |
| OCI Image Specification | v1.1.0 |
| RFC 9052 | CBOR Object Signing and Encryption (COSE): Structures and Process |
| RFC 7515 | JSON Web Signature (JWS) |
| RFC 3161 | Internet X.509 PKI Time-Stamp Protocol (TSP) |
| RFC 5280 | Internet X.509 Public Key Infrastructure Certificate and CRL Profile |

Margo adopts these specifications in full. Where Margo narrows or extends, this SUP states the delta explicitly.

> **Open WG decision (unresolved):** the Notary Project `main` branch has, since this SUP's `v1.1.0` pin, introduced a different default signature-manifest shape (see §6 and the companion guide §2.2). Whether Margo should track that shape once it ships in a Notary Project release, or continue pinning the last released tag until a WG-scheduled review, is not decided by this revision and is left for the WG.

> **Open WG decision (unresolved):** ETSI TS 119 152-1 (CB-AdES), published 2026-03, defines an AdES-conformant profile of COSE (RFC 9052) that this SUP's COSE_Sign1 envelope does not currently implement. Whether a future revision should additionally profile CB-AdES's mandatory header parameters to achieve formal AdES-family membership for the COSE path is not addressed by this revision and is left for the WG.

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

**Constrained-device exception.** A WFM Client **MAY** declare a `signatureEnvelopes: cose-only` capability. Where a WFM Client makes this declaration, the operator's trust policy **MUST** restrict that device's `registryScopes` to repository paths whose publishers commit to COSE-only signing, and the declaration **MUST** be explicit — a WFM Client that has not declared `signatureEnvelopes: cose-only` is held to the full dual-format requirement above. This follows the same honest-declaration pattern this SUP already uses in §4's default-behavior table (and that WG-PROPOSAL-04 uses for `secretsAtRest`): a device states its actual capability, and the trust-policy layer — not a silent capability gap — carries the consequence. Under this exception, the interoperability guarantee moves from "every device parses every format" to "every artifact offered to a declared-COSE-only device is COSE," enforced at the trust-policy `registryScopes` layer this SUP already defines (§5.2); the anti-fragmentation guarantee for the general fleet is unchanged.

> **Open WG decision (unresolved):** whether the WG prefers this capability-declaration route, or the unqualified universal MUST with no exception, is not settled by this revision. Nor is the schema or interface through which a WFM Client's `signatureEnvelopes: cose-only` declaration and a publisher's COSE-only commitment would be expressed and discovered — both are asserted here as organizational/policy facts with no defined carrier artifact, and resolving that is a precondition for this exception being independently testable.

Operators **MAY** impose stricter requirements through deployment-specific trust policies. Customer security policy takes precedence over these recommendations.

### 4. Conformance actors and obligations

| Actor | Obligation | From SUP-03 Approved (Phase 3) | From first Margo GA specification release integrating this profile |
|-------|-----------|-----------|-----|
| Application Developer | Sign artifacts per §2 | **SHOULD** | **MUST** |
| WFM | Verify signatures when trust policy level ≠ `skip` | **SHOULD** | **MUST** |
| WFM Client | Verify signatures before deploying workloads | **SHOULD** | **MUST** |

**Default behavior once this SUP reaches Approved (Phase 3) in `margo/specification-enhancements`:** When no trust policy is configured, verification is not performed. Unsigned artifacts remain deployable. This permits incremental adoption.

**Default behavior from the first Margo GA specification release (plain SemVer) that integrates this signing profile as normative text:** When no trust policy is configured, the default is `strict` — unsigned artifacts are rejected. Exact enforcement semantics follow the Notary Project `trust-store-trust-policy.md` specification.

> **Open WG decision (unresolved):** the exact milestone wording above is for the WG to settle. This SUP is not itself a versioned Margo deliverable — Margo's release model versions the Specification, the OpenAPI spec, and (from GA) the Reference Implementation and Conformance Test Toolkit, not individual SUPs. This proposal therefore acquires a concrete MUST-tier trigger date only once it lands inside a numbered specification release; until then, "Approved (P3)" is the strongest milestone this SUP can claim for itself.

### 5. Publisher signing material

#### 5.1 Definition

Application Developers **MUST** provide publisher signing material to enable downstream verification of their artifacts. This material allows WFM and WFM Client actors to establish trust in the developer's signing identity.

> **Terminology note:** This SUP uses the term "publisher signing material." The term "trust bundle" is reserved for MIAF SPIFFE Trust Bundles and **MUST NOT** be used in this context. See §5.4 for how this SUP's identity model relates to MIAF's.

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

#### 5.4 Relationship to SUP-04 (secret delivery)

SUP-03 and SUP-04 (secret delivery) run two deliberately separate identity systems. This is by design, not oversight:

- SUP-03 uses a **developer-side X.509 code-signing PKI**: the actor is the Application Developer, the artifact is signed once at publish time, the trust anchor is a root CA in the verifier's trust store, and validity must survive 10–25 year OT asset lifetimes (hence the RFC 3161 timestamping in §3 and §6).
- SUP-04 uses **MIAF SPIFFE X.509-SVID device identity**: the actor is a device authenticating to a live service, credentials are short-lived and continuously rotated, and the trust domain is operational rather than publication-time.

The two are not interchangeable and neither substitutes for the other: signing attests *provenance of an artifact* to anyone who later verifies it offline; an SVID attests *identity of a device* to a service during an online exchange. SUP-03 introduces **no MIAF dependency**, and a device with no SPIFFE identity can still verify signatures.

### 6. Verification

WFM and WFM Client verifiers **MUST** implement the signing-and-verification workflow of the Notary Project `signing-and-verification-workflow.md` specification at the version pinned in §1.

Verification **MUST** be performed against the target artifact's manifest digest. A verifier **MUST** discover candidate signatures via the OCI Referrers API filtered on `artifactType=application/vnd.cncf.notary.signature`. Certificate path validation **MUST** conform to RFC 5280 and terminate at a trust anchor in the configured trust store; the leaf certificate **MUST** carry Key Usage `digitalSignature` and Extended Key Usage Code Signing.

**Signature validity binds to digest, not location.** A signature's validity is bound to the target manifest's SHA-256 digest, not to a registry hostname or repository path. A signature therefore remains valid after the signed artifact is replicated or mirrored to another registry or repository path, provided the referrer manifest carrying the signature is copied alongside the subject manifest. A verifier **MUST NOT** incorporate registry hostname or repository path into signature validity. (Trust-policy `registryScopes` matching, per §5.2, remains a separate, policy-layer concern — an operator may still restrict which repository paths a trust policy applies to; that restriction does not change whether the underlying signature is valid.)

Where the signing certificate has expired, a verifier **MUST** accept the signature only if an RFC 3161 timestamp token proves the signature predates the certificate's `notAfter`.

The per-level (`strict` / `permissive` / `audit` / `skip`) pass-fail semantics for verification failures are adopted by reference from the Notary Project `trust-store-trust-policy.md` specification pinned in §1.

The companion guide's 10-step verification walkthrough (§3) is informative elaboration of this section — it does not introduce an independent requirement.

### 7. Known limitations

1. **Mutable tag references between tiers.** Signing binds a signature to a single manifest digest. However, references between Application Package → Component → Container Image currently use OCI tags, which are mutable. Digest pinning across the artifact tier hierarchy is deferred to a future SUP.

2. **WFM Client cannot interpose on image pulls.** On Kubernetes, the kubelet pulls container images; on Compose, the container engine pulls them. The WFM Client can verify archive artifacts it receives directly but cannot enforce signature verification on images at runtime without a device-level admission controller. This is an inherent platform constraint.

3. **No automated publisher signing material distribution.** Per §5.3, distribution is operator-managed. Automated provisioning over the Management Interface is out of scope for this SUP.

### 8. Affected specification files

| File | Change |
|------|--------|
| `system-design/specification/applications/application-registry.md` | Add cross-reference to signing profile |
| `system-design/specification/security/signing-profile.md` (NEW) | Normative Margo signing profile document |

No LinkML schema files are modified by this proposal.

### 9. Schema change report

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
- IETF RFC 5280 — Internet X.509 Public Key Infrastructure Certificate and CRL Profile
- IETF RFC 7515 — JSON Web Signature (JWS)
- IETF RFC 9052 — CBOR Object Signing and Encryption (COSE): Structures and Process
- CNCF Notary Project Specifications — https://github.com/notaryproject/specifications
- OCI Distribution Specification v1.1.0 — https://github.com/opencontainers/distribution-spec
- OCI Image Specification v1.1.0 — https://github.com/opencontainers/image-spec

## Rejection reason

> Not applicable — SUP is in active development.

---

*Prepared by Andrii Melashchenko (Belden Inc.), 2026-07-29.*
