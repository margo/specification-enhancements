# OCI Cryptographic Signature Architecture and Trust Verification Framework

## Owner

[@javatask](https://github.com/javatask) — Andrii Melashchenko, Belden Inc.

## Summary

This SUP establishes a cryptographic signing and verification framework for Margo Application Packages, Compose/Helm component archives, and container images. Signatures are stored as detached OCI referrer manifests using the OCI Distribution Spec v1.1 Referrers API, binding exclusively to artifact manifest digests. The framework defines what is signed, where signatures are stored, envelope format requirements, verification obligations per actor, and trust policy configuration — all in a technology-agnostic manner. The CNCF Notary Project (Notation) is the RECOMMENDED implementation.

Key design properties:

- **Additive-only** — no changes to existing OCI manifests, `margo.yaml`, or LinkML schemas
- **Digest-bound** — signatures bind to SHA-256 manifest digests, not mutable tags or registry paths
- **Path-independent** — signed artifacts can be mirrored across registries without re-signing
- **Fail-closed** — absent trust policy defaults to `strict` enforcement (reject unverifiable artifacts)

## Reason for proposal

The current Margo specification contains a single recommendation to "digitally sign this package" using PGP encryption with a `keyLocation` field in ApplicationDescription. This is insufficient for production OT deployments:

1. **No defined signature storage model** — PGP detached signatures have no standard OCI storage location, preventing interoperable verification across implementations.
2. **No verification algorithm** — The specification does not define what implementations must check, when to reject artifacts, or how to handle certificate expiry in long-lived OT assets (10+ year lifecycle).
3. **No trust policy framework** — Operators have no standardized mechanism to configure which signing identities are trusted for which registry scopes.
4. **PGP limitations** — PGP lacks native support for timestamping (critical for long-lived OT), has no standard revocation infrastructure, and is not aligned with modern supply-chain security practices (CNCF, OCI, SLSA).
5. **Regulatory gap** — Industrial deployments subject to IEC 62443-4-2 or NIST SP 800-218 require auditable cryptographic integrity verification. The current spec provides no compliance path.

SUP-03 resolves these gaps by establishing a complete, interoperable signing and verification contract built on OCI-native primitives and industry-standard X.509 cryptography.

## Requirements alignment acknowledgement

This SUP addresses supply-chain integrity requirements implicit in the Margo security model:

- **IEC 62443-4-2 CR 3.9** (Software Integrity) — requires cryptographic verification of software before installation on OT devices.
- **NIST SP 800-218 (SSDF)** — requires protection of software from unauthorized tampering throughout the supply chain.
- **OCI Distribution Spec v1.1** — provides the Referrers API used for signature storage and discovery.

**Out of scope:**

- LinkML schema changes — signatures are an OCI-layer operational concern (see §9 Schema Change Report)
- Deprecation/removal of the `keyLocation` field — deferred to a separate editorial SUP
- Runtime integrity monitoring — out of Margo scope


## Technical proposal

### Affected files

| File | Change type |
|---|---|
| `system-design/specification/applications/application-registry.md` | Normative addition — new "Signature Verification" subsection |
| `system-design/concepts/applications/application-package.md` | Normative update — replace PGP recommendation with cross-reference |
| _(NEW)_ `system-design/specification/security/signature-verification.md` | New normative file — verification engine specification |
| _(NEW)_ `system-design/specification/security/trust-policy.md` | New normative file — trust policy schema and verification levels |
| `system-design/specification/applications/application-description.md` | Editorial — deprecation notice on `keyLocation` |

**No LinkML schema files are modified.** Signatures are stored as detached OCI referrer manifests and do not appear in `margo.yaml`, ApplicationDescription, or DesiredState.

---

### 1. Signature storage model

#### 1.1 Detached OCI referrer architecture

Signatures are stored as **detached OCI referrer manifests** in the same repository as the signed artifact. They are never embedded inside the artifact manifest or referenced from application metadata.

```
Registry: registry.example.com/org/my-margo-app

  ┌─────────────────────────────────────────────────────┐
  │ Target Artifact Manifest                            │
  │ digest: sha256:8f9a2b...                            │
  │ artifactType: application/vnd.margo.app.v1+json     │
  └────────────────────────┬────────────────────────────┘
                           │ subject digest link
                           ▼
  ┌─────────────────────────────────────────────────────┐
  │ Signature Referrer Manifest                         │
  │ digest: sha256:1c4d5e...                            │
  │ artifactType: application/vnd.cncf.notary.signature │
  │ subject.digest: sha256:8f9a2b...                    │
  └────────────────────────┬────────────────────────────┘
                           │ layer reference
                           ▼
  ┌─────────────────────────────────────────────────────┐
  │ Signature Envelope Blob                             │
  │ mediaType: application/cose                         │
  │ Contains: payload + X.509 chain + RFC 3161 TSA      │
  └─────────────────────────────────────────────────────┘
```

#### 1.2 Signature referrer manifest structure

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "artifactType": "application/vnd.cncf.notary.signature",
  "config": {
    "mediaType": "application/vnd.cncf.notary.signature",
    "digest": "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "size": 2
  },
  "layers": [
    {
      "mediaType": "application/cose",
      "digest": "sha256:<envelope-blob-digest>",
      "size": 4820
    }
  ],
  "subject": {
    "mediaType": "application/vnd.oci.image.manifest.v1+json",
    "digest": "sha256:<target-artifact-manifest-digest>",
    "size": 1024
  },
  "annotations": {
    "io.cncf.notary.x509chain.thumbprint#S256": "[\"<leaf-cert-sha256-fingerprint>\"]"
  }
}
```

- The `subject` field creates the OCI Referrers API linkage to the signed artifact.
- The `annotations` fingerprint enables pre-filtering signatures before downloading envelope blobs.
- Multiple signatures **MAY** exist for a single artifact (e.g., vendor signature + enterprise counter-signature).

#### 1.3 Discovery protocol

Verifiers discover signatures via the OCI Referrers API:

```
GET /v2/{name}/referrers/{digest}?artifactType=application/vnd.cncf.notary.signature
```

Response: OCI Image Index listing all signature referrer manifests for the target digest.

#### 1.4 Registry requirements

Registries hosting Margo artifacts that require signature verification **MUST** support OCI Distribution Spec v1.1 Referrers API.

Signature referrer manifests **MUST NOT** be garbage-collected while the target artifact exists.


---

### 2. What is signed — three-tier artifact binding

#### 2.1 Signing obligations

Application Developers **MUST** sign the complete artifact chain across all three Margo registry tiers:

| Tier | Artifact | What is signed |
|---|---|---|
| **Tier 1: Application Registry** | Application Package | SHA-256 digest of the Application Package OCI image manifest |
| **Tier 2: Component Registry** | Compose Archive / Helm Chart | SHA-256 digest of the component OCI image manifest |
| **Tier 3: Container Image Registry** | Container Image | SHA-256 digest of the container image manifest |

Signing the Application Package manifest transitively protects all layer blobs (their digests are listed in `layers[]`), including `margo.yaml`, icons, and release notes.

#### 2.2 Signature payload format

The signature payload is a JSON document with content type `application/vnd.cncf.notary.payload.v1+json`. It binds exclusively to the OCI manifest digest — it contains no registry hostname, repository path, or tag.

```json
{
  "targetArtifact": {
    "mediaType": "application/vnd.oci.image.manifest.v1+json",
    "digest": "sha256:<manifest-digest>",
    "size": <manifest-size-in-bytes>
  }
}
```

**Key properties:**

1. **Digest-bound, not tag-bound** — eliminates TOCTOU attacks from mutable tags (`:latest`, `:v1.0.0`)
2. **Path-independent** — signed artifacts can be mirrored/promoted across registries without invalidating signatures
3. **Transitive integrity** — signing the manifest protects all referenced layer blobs

#### 2.3 OCI media types

SUP-03 reuses existing CNCF Notary Project and IETF media types. No new Margo-specific media types are introduced for signatures.

| Media Type | Origin | Usage |
|---|---|---|
| `application/vnd.cncf.notary.signature` | CNCF Notary Project v1.2 | `artifactType` on the signature referrer manifest |
| `application/cose` | RFC 9052 | Layer `mediaType` for COSE_Sign1 envelope blob |
| `application/jose+json` | RFC 7515 | Layer `mediaType` for JWS envelope blob |
| `application/vnd.cncf.notary.payload.v1+json` | CNCF Notary Project v1.2 | Content type of the signed payload inside the envelope |


#### 2.4 Pre-publish signature validation

Application Developers **MUST** verify their own signatures before publishing artifacts to any registry. This ensures that signing tooling, certificate chains, and trust policy configurations are correct before downstream consumers (WFM, devices) attempt verification.

The pre-publish validation **MUST** include:

1. Verify the signature envelope is well-formed (COSE_Sign1 or JWS parseable)
2. Verify the `targetArtifact.digest` in the payload matches the actual manifest digest
3. Verify the embedded X.509 certificate chain is complete (leaf → intermediate(s) → root)
4. Verify the leaf certificate has correct EKU (`codeSigning`)
5. Verify the signature cryptographically validates against the leaf public key

Application Developers **SHOULD** execute `notation verify` (or equivalent) against their own trust policy before pushing the signed artifact to the distribution registry. A failed self-verification **MUST** block publication.

#### 2.5 Signing scope — all Margo-defined OCI artifacts

This specification mandates signing for **all** OCI artifacts defined by the Margo specification. Specifically:

| Margo OCI Artifact | Manifest identifier | Signing required |
|---|---|---|
| Application Package | `artifactType: application/vnd.margo.app.v1+json` | **MUST** sign |
| Compose Archive | `artifactType: application/vnd.org.margo.component.compose+json` | **MUST** sign |
| Helm Chart (OCI) | `config.mediaType: application/vnd.cncf.helm.config.v1+json` | **MUST** sign |
| Container Image | `mediaType: application/vnd.oci.image.manifest.v1+json` | **MUST** sign |

> **Note:** Helm charts use the legacy OCI 1.0 pattern where `config.mediaType` serves as the artifact type discriminator (no explicit `artifactType` field). Container images are identified by their manifest `mediaType`. In all cases, what is signed is the OCI manifest digest regardless of how the artifact type is identified.

Any future Margo-defined OCI artifact type (e.g., Quadlet archives) **MUST** also be signed following this specification.

---

### 3. Vendor trust bundle

#### 3.1 Trust bundle definition

When publishing signed Margo Application Packages, the Application Developer **MUST** provide a **vendor trust bundle** — a distributable package containing the cryptographic materials and policy template needed for WFM and device operators to verify the developer's signatures.

#### 3.2 Trust bundle structure

The vendor trust bundle is a directory (or archive) with the following normative structure:

```
<vendor-name>-trust-bundle/
├── certificates/
│   ├── root-ca.crt              # REQUIRED — PEM-encoded root CA certificate (trust anchor)
│   ├── intermediate-ca.crt      # OPTIONAL — PEM-encoded intermediate CA(s) if not embedded in signature
│   └── tsa-root-ca.crt          # OPTIONAL — PEM-encoded TSA root CA (required if TSA used)
├── trustpolicy-template.json    # REQUIRED — Template trustpolicy.json with placeholder scopes
└── README.md                    # REQUIRED — Human-readable installation instructions
```

**File requirements:**

| File | Required | Format | Purpose |
|---|---|---|---|
| `certificates/root-ca.crt` | **REQUIRED** | PEM (X.509 v3) | Trust anchor — the root CA that issued the signing certificate chain |
| `certificates/intermediate-ca.crt` | **OPTIONAL** | PEM (X.509 v3) | Intermediate CA(s) for environments that don't extract from envelope |
| `certificates/tsa-root-ca.crt` | **OPTIONAL** | PEM (X.509 v3) | TSA trust anchor for RFC 3161 timestamp verification |
| `trustpolicy-template.json` | **REQUIRED** | JSON (Notation trust policy schema) | Pre-configured policy with vendor identity; `registryScopes` left as placeholder |
| `README.md` | **REQUIRED** | Markdown | Installation guide for importing into WFM/device trust stores |

#### 3.3 Trust policy template

The `trustpolicy-template.json` **MUST** contain the vendor's `trustedIdentities` (Subject DN) and `trustStores` references pre-filled. The `registryScopes` field **MUST** use a placeholder value that operators replace with their actual registry paths:

```json
{
  "version": "1.0",
  "trustPolicies": [
    {
      "name": "<vendor-name>-production",
      "registryScopes": [
        "<REPLACE_WITH_YOUR_REGISTRY>/margo-apps/*"
      ],
      "signatureVerification": {
        "level": "strict"
      },
      "trustStores": [
        "ca:<vendor-name>-root-ca",
        "tsa:<vendor-name>-tsa"
      ],
      "trustedIdentities": [
        "x509.subject: CN=<Vendor Signing Identity>, O=<Vendor Organization>, C=<Country>"
      ]
    }
  ]
}
```

#### 3.4 Trust bundle discovery — well-known OCI repository

To simplify automated trust bundle import, Application Developers **SHOULD** publish their trust bundle as an OCI artifact in a **well-known repository path** within the same registry namespace as their application packages:

```
<registry>/<namespace>/_trust-bundle
```

**Convention:**

| Application Package location | Trust bundle location |
|---|---|
| `registry.vendor.com/margo-apps/my-app:v1.0.0` | `registry.vendor.com/margo-apps/_trust-bundle:latest` |
| `harbor.enterprise.internal/acme-apps/gateway:v2.1.0` | `harbor.enterprise.internal/acme-apps/_trust-bundle:latest` |

The trust bundle OCI artifact **SHOULD** use:
- `artifactType`: `application/vnd.margo.trust-bundle.v1+tar`
- Layer: gzip-compressed tar of the trust bundle directory structure

WFM implementations **MAY** auto-discover trust bundles by querying the `_trust-bundle` repository in the same namespace. This is a **convenience mechanism** — operators are not required to use it and **MAY** provision trust materials through any vendor-specific channel (USB key, configuration management, manual import).

> **Note:** The `_trust-bundle` convention is RECOMMENDED, not REQUIRED. Trust provisioning remains vendor-specific. This convention reduces friction for initial onboarding.

> **Compatibility note:** The underscore prefix in `_trust-bundle` is intentionally chosen to minimize collision with application repository names. While most OCI registries (Harbor, ACR, Zot, GHCR) accept underscore-prefixed path components, registries strictly implementing the OCI Distribution Spec name grammar (`[a-z0-9]+([._-][a-z0-9]+)*`) may require configuration. Implementations SHOULD fall back to alternative provisioning mechanisms if the `_trust-bundle` repository cannot be created.

#### 3.5 Trust bundle versioning

Trust bundles **SHOULD** be tagged with semantic version tags (e.g., `:v1.0.0`, `:v2.0.0`) when the root CA rotates or trusted identities change. The `:latest` tag **SHOULD** always point to the current active trust bundle.

---

### 4. PKI mode validation

#### 4.1 Self-signed CA (Mode A) — validated scenario

Self-signed PKI is the simplest deployment model. It works correctly with this specification because:

1. Developer generates a local root CA + code-signing leaf certificate
2. Developer signs all 3 tiers using Notation with the leaf key
3. Developer publishes trust bundle containing `root-ca.crt` + policy template
4. Operator imports `root-ca.crt` into WFM/device trust store (`ca:<store-name>`)
5. Operator configures `trustedIdentities` matching the leaf certificate Subject DN
6. Verification engine builds chain: leaf cert → root CA in trust store ✓

**Limitations of self-signed mode:**
- No revocation mechanism — if the key is compromised, the only remedy is removing the root CA from all trust stores
- No TSA — signatures expire when the leaf certificate expires
- No third-party trust — any developer can generate a certificate claiming any Subject DN

Self-signed mode **MUST NOT** be used in production OT environments (as stated in §6.3).

#### 4.2 Corporate/self-hosted enterprise PKI (Mode B) — validated scenario

Enterprise-operated PKI (e.g., Microsoft AD CS, EJBCA, HashiCorp Vault PKI) works correctly because:

1. Enterprise CA issues code-signing certificates to authorized developers
2. Enterprise operates an internal OCSP responder and/or CRL distribution point
3. Enterprise MAY operate an internal RFC 3161 TSA
4. Developer signs using HSM-backed keys (recommended) or software keys
5. Trust bundle contains enterprise root CA + optional intermediate + optional TSA root
6. Verification engine builds chain through enterprise CA hierarchy ✓
7. Revocation checks query enterprise OCSP/CRL infrastructure ✓

**This is the RECOMMENDED model for production OT deployments** where the organization controls both the signing and verification infrastructure.

#### 4.3 EU Qualified Trust Service Provider (Mode C) — validated scenario

Using QTSP-issued certificates works as a middle ground because:

1. Developer obtains a code-signing certificate from a QTSP on the EU Trusted List
2. QTSP provides the trust chain (QTSP root CA → intermediate → leaf)
3. Developer signs using QTSP-provided key (HSM-backed for Qualified, software for Advanced)
4. Trust bundle contains QTSP root CA certificate
5. Verification engine builds chain through QTSP hierarchy ✓
6. OCSP/CRL provided by QTSP infrastructure ✓
7. RFC 3161 TSA available from QTSP or third-party TSA ✓

See §10.1 (Informative Note) for detailed eIDAS guidance.

---

### 5. Signature envelope format

#### 5.1 Generic envelope structure (technology-agnostic)

A signature envelope is a self-contained cryptographic structure stored as a single OCI layer blob within the signature referrer manifest. It comprises:

| Component | Purpose |
|---|---|
| **Protected headers** | Algorithm ID (`alg`), content type, signing time, optional expiry |
| **Payload** | The `targetArtifact` descriptor — digest, mediaType, size |
| **Signature bytes** | Cryptographic signature over (protected headers ‖ payload) |
| **Unprotected headers** | X.509 certificate chain, RFC 3161 timestamp token, signing agent |

#### 5.2 Cryptographic requirements

| Requirement | Specification |
|---|---|
| Certificate type | X.509 v3 code-signing certificates |
| Key Usage | `digitalSignature` |
| Extended Key Usage | Code Signing (`1.3.6.1.5.5.7.3.3`) |
| Minimum key size (RSA) | 2048 bits |
| Minimum key size (ECDSA) | P-256 (secp256r1) or P-384 (secp384r1) |
| Signature algorithms | RSASSA-PSS (PS256, PS384, PS512), ECDSA (ES256, ES384, ES512) |
| Hash algorithm | SHA-256 minimum; SHA-384 and SHA-512 permitted |
| Certificate chain | **MUST** be embedded in the signature envelope unprotected headers |
| Timestamp | RFC 3161 TSA counter-signature **SHOULD** be included; **MUST** be included when signing certificate validity is shorter than expected artifact operational lifetime |

#### 5.3 Envelope format comparison

| Property | COSE_Sign1 (`application/cose`) | JWS (`application/jose+json`) |
|---|---|---|
| Specification | RFC 9052 | RFC 7515 |
| Wire format | Binary CBOR | JSON text |
| Certificate chain header | `x5chain` (unprotected) | `x5c` (unprotected) |
| Timestamp token | `io.cncf.notary.timestampSignature` | Same |
| Edge suitability | **RECOMMENDED** — compact, fast parsing | Acceptable — human-readable |

**Normative requirements:**

- Verifiers (WFM and edge devices) **MUST** support both COSE_Sign1 and JWS envelopes.
- Application Developers **SHOULD** produce COSE_Sign1 envelopes for production artifacts.
- The signing scheme **MUST** be `notary.x509` (not `notary.x509.signingAuthority`).

#### 5.4 RECOMMENDED implementation — CNCF Notary Project Notation

The CNCF Notary Project Notation CLI and `notation-go` library are the **RECOMMENDED** implementation of this specification. Implementations using alternative tooling **MUST** produce signature referrer manifests and envelope structures conforming to sections 1–3 of this proposal.


---

### 6. Conformance obligations

#### 6.1 Actor obligations

| Actor | Obligation | Keyword | Rationale |
|---|---|---|---|
| Application Developer | Sign all three artifact tiers (Application Package, component archives, container images) | **MUST** | Supply chain integrity requires origin authentication at every tier |
| Application Developer | Verify own signature before publishing (pre-publish validation) | **MUST** | Prevents publishing malformed signatures that downstream consumers cannot verify |
| Application Developer | Embed complete X.509 certificate chain in signature envelope | **MUST** | Enables offline verification at edge devices |
| Application Developer | Provide a vendor trust bundle (root CA + policy template) | **MUST** | WFM/device operators need trust materials to configure verification |
| Application Developer | Publish trust bundle to well-known `_trust-bundle` repository | **SHOULD** | Simplifies automated trust onboarding |
| Application Developer | Include RFC 3161 TSA counter-signature for production artifacts | **SHOULD** | Ensures verifiability beyond signing certificate expiry |
| WFM | Validate signatures before catalog ingestion | **SHOULD** | Defense-in-depth; WFM is not the final enforcement boundary |
| WFM | Validate signatures when trust policy `signatureVerification.level` ≠ `skip` | **MUST** | Policy-driven enforcement |
| WFM | Support all four verification levels (`strict`, `permissive`, `audit`, `skip`) | **MUST** | Operational flexibility |
| Edge Device | Validate signatures before expanding any archive or deploying any workload | **MUST** | Zero-trust edge autonomy — device is the final enforcement boundary |
| Edge Device | Support at minimum `strict` and `skip` verification levels | **MUST** | Minimum viable implementation |
| Edge Device | Support `permissive` and `audit` levels | **SHOULD** | Migration scenarios |
| Trust configuration | Operator-specific (registry scopes, trusted identities, trust stores) | Vendor-specific | Deployment-specific, not standardizable |

#### 6.2 Phased adoption timeline

To prevent a de facto breaking change (existing unsigned artifacts becoming undeployable), signature verification follows a phased rollout:

| Margo Version | Signing Obligation | Verification Obligation |
|---|---|---|
| v1-alpha1 | Application Developers **SHOULD** sign artifacts | Verification is **OPTIONAL** |
| v1-alpha2 | Application Developers **MUST** sign artifacts | Verification is **RECOMMENDED** |
| v1 (GA) | Application Developers **MUST** sign artifacts | Edge devices **MUST** verify; WFM **MUST** verify when policy ≠ `skip` |

Implementations **MAY** adopt `strict` verification ahead of the GA timeline. The phased approach ensures ecosystem tooling maturity before mandating enforcement.

#### 6.3 Dual-PKI paradigm

SUP-03 supports two PKI modes covering the full spectrum of industrial deployments:

| Dimension | Mode A: Self-Signed | Mode B: Enterprise PKI |
|---|---|---|
| Trust anchor | Developer-generated self-signed root CA | Enterprise CA or Qualified Trust Service Provider (QTSP) |
| Key storage | Local file or software keystore | HSM (FIPS 140-2 Level 3 recommended) |
| Timestamping | Local system clock only (`signingTime`) | RFC 3161 TSA counter-signature |
| Revocation | None — manual trust store removal | OCSP (RFC 6960) with offline CRL fallback |
| Production use | **MUST NOT** be used in production OT environments | **REQUIRED** for production |
| Verification level | `permissive` or `strict` | `strict` |

> **IMPORTANT:** Self-Signed mode provides NO revocation capability. Application Developers and operators **MUST NOT** rely on self-signed certificates for production OT deployments where key compromise recovery is required.

> **Note:** §4 validates a third deployment scenario — Mode C (QTSP/eIDAS). At the verification layer, Mode C is technically indistinguishable from Mode B (both use third-party-issued X.509 certificates with OCSP/CRL). The distinction is organizational: Mode B uses internally-operated enterprise PKI, Mode C uses externally-operated QTSP services.


---

### 7. Verification engine

The verification algorithm is informed by ETSI EN 319 102-1 (Procedures for Creation and Validation of AdES Digital Signatures). This section specifies *what* implementations must verify. The step ordering is non-normative — implementations may reorder checks provided all required checks execute before a pass/fail decision.

#### 7.1 Verification steps

| Step | Check | Requirement | Failure behavior |
|---|---|---|---|
| 1 | **Manifest retrieval** | Retrieve target artifact manifest by content-addressable digest | FAIL: artifact not found |
| 2 | **Referrers discovery** | Query Referrers API for signatures (`artifactType=application/vnd.cncf.notary.signature`) | FAIL: no signatures found (unless level = `skip`) |
| 3 | **Fingerprint filtering** | Pre-filter signatures by `io.cncf.notary.x509chain.thumbprint#S256` annotation against configured trust stores | SKIP signatures not matching any trust store |
| 4 | **Trust policy scope matching** | Match artifact repository URI against `registryScopes` in trust policy | FAIL: no matching policy scope |
| 5 | **Envelope parsing** | Download and parse signature envelope (COSE_Sign1 or JWS) | FAIL: malformed envelope |
| 6 | **Certificate chain validation** | Build path from leaf certificate to a trust anchor in configured trust stores; verify Key Usage (`digitalSignature`) and EKU (Code Signing) | FAIL: untrusted chain or invalid certificate profile |
| 7 | **Subject identity matching** | Compare leaf certificate Subject DN against `trustedIdentities` patterns | FAIL: identity not in allowlist |
| 8 | **Digest binding verification** | Verify cryptographic signature over payload; confirm `targetArtifact.digest` matches target manifest digest | FAIL: signature invalid or digest mismatch |
| 9 | **Timestamp evaluation** | If signing certificate is expired: verify RFC 3161 TSA token proves signature was created before `notAfter` | FAIL (strict): expired cert without valid timestamp |
| 10 | **Revocation check** | Query OCSP for certificate status; fall back to locally cached CRLs if OCSP is unreachable | FAIL (strict): certificate revoked |

#### 7.2 Offline operation (air-gapped environments)

Industrial OT environments frequently lack network connectivity to OCSP responders. Implementations **MUST** support the following offline fallbacks:

- If OCSP is unreachable, implementations **MUST** fall back to locally cached Certificate Revocation Lists (CRLs).
- CRL distribution points **SHOULD** be pre-provisioned during device commissioning.
- If neither OCSP nor CRL is available and `signatureVerification.override.revocation` is `"enforce"`, the verification **MUST** fail.
- If `override.revocation` is `"log"` or `"skip"`, verification proceeds with appropriate audit logging.

#### 7.3 Long-lived OT assets and timestamping

OT devices commonly operate for 10+ years. Signing certificates typically have 2–3 year validity periods. Without timestamping, packages become unverifiable after certificate expiry.

- Application Developers **SHOULD** include RFC 3161 TSA counter-signatures on all production signatures.
- When `signatureVerification.level` is `strict` and the signing certificate has expired, a valid RFC 3161 timestamp proving the signature was created before expiry is **REQUIRED** for verification to succeed.
- TSA certificates used for timestamp verification **MUST** be provisioned in a `tsa:<name>` trust store.


---

### 8. Trust policy framework

The trust policy is a WFM/device-local configuration file. It is NOT stored in OCI registries and is NOT part of any LinkML schema. It governs how the verification engine evaluates signatures at deployment time.

Trust policy configuration is **vendor-specific** — the content (registry scopes, trusted identities, trust stores) is a deployment concern of the WFM or device operator, not the application developer.

#### 8.1 Verification levels

| Level | Integrity & Authenticity | Expiry | Revocation | Timestamp | Failure behavior |
|---|---|---|---|---|---|
| **`strict`** | Reject on failure | Reject without valid TSA | Reject on revoked cert | Required if cert expired | **Abort** deployment; emit security event |
| **`permissive`** | Reject on failure | Warn on failure | Warn on failure | Optional; log if absent | **Fail** on tamper; **warn** on expiry/revocation |
| **`audit`** | Logged only | Logged | Logged | Logged | **Always proceed**; write structured audit log |
| **`skip`** | Skipped | Skipped | Skipped | Skipped | No verification. **DEBUG/DEV ONLY** |

**Normative requirements on verification levels:**

- Default level when no trust policy exists **MUST** be `strict` (fail-closed).
- Transitioning between levels **MUST** be achievable via policy-only change — no artifact re-signing required.
- `skip` level **MUST NOT** be used in production environments.
- Implementations **SHOULD** log a security warning when `skip` is the active policy for any scope.
- Production Margo deployments **MUST** configure `strict`.

#### 8.2 Abstract trust policy requirements (normative)

Regardless of implementation tooling, a conforming trust policy mechanism **MUST** support:

1. **Registry scope binding** — mapping OCI repository URI patterns to policy entries
2. **Enforcement level selection** — one of `strict`, `permissive`, `audit`, `skip`
3. **Per-check overrides** — ability to override expiry, revocation, and timestamp behavior independently
4. **Trust anchor configuration** — specifying CA certificates used for chain validation
5. **TSA anchor configuration** — specifying TSA certificates for timestamp verification
6. **Identity allowlist** — specifying permitted X.509 Subject DN patterns

#### 8.3 RECOMMENDED implementation — Notation `trustpolicy.json`

When using the CNCF Notary Project Notation as the verification implementation, the trust policy is expressed as `trustpolicy.json`. The following documents all fields:

##### Schema

```json
{
  "version": "1.0",
  "trustPolicies": [
    {
      "name": "<unique-policy-name>",
      "registryScopes": ["<scope-pattern>", "..."],
      "signatureVerification": {
        "level": "<strict|permissive|audit|skip>",
        "override": {
          "expiry": "<enforce|log|skip>",
          "revocation": "<enforce|log|skip>",
          "timestampVerification": "<enforce|log|skip>"
        }
      },
      "trustStores": ["<type>:<name>", "..."],
      "trustedIdentities": ["<identity-expression>", "..."]
    }
  ]
}
```

##### Field specification

| Field | Required | Customer-configurable | Constraints |
|---|---|---|---|
| `version` | **REQUIRED** | No | Fixed `"1.0"` |
| `trustPolicies` | **REQUIRED** | Yes (contents) | Array; ≥1 entry |
| `trustPolicies[].name` | **REQUIRED** | Yes | Unique identifier; used for audit logging |
| `trustPolicies[].registryScopes` | **REQUIRED** | Yes | OCI repository patterns with `*` suffix wildcard. Each repository **MUST** match at most one policy. |
| `trustPolicies[].signatureVerification` | **REQUIRED** | Yes | Contains `level` and optional `override` |
| `trustPolicies[].signatureVerification.level` | **REQUIRED** | Yes | `strict` &#124; `permissive` &#124; `audit` &#124; `skip` |
| `trustPolicies[].signatureVerification.override` | **OPTIONAL** | Yes | Per-check behavior overrides. Override values take precedence over the defaults implied by the selected `level`. |
| `trustPolicies[].signatureVerification.override.expiry` | **OPTIONAL** | Yes | `enforce` (default for strict) &#124; `log` &#124; `skip` |
| `trustPolicies[].signatureVerification.override.revocation` | **OPTIONAL** | Yes | `enforce` (default for strict) &#124; `log` &#124; `skip` |
| `trustPolicies[].signatureVerification.override.timestampVerification` | **OPTIONAL** | Yes | `enforce` (default for strict) &#124; `log` &#124; `skip` |
| `trustPolicies[].trustStores` | **REQUIRED** (unless level=`skip`) | Yes | Format: `<type>:<name>` where type ∈ {`ca`, `tsa`}. Maps to `{NOTATION_CONFIG}/truststore/x509/{type}/{name}/`. At least one `ca:` entry required. |
| `trustPolicies[].trustedIdentities` | **REQUIRED** (unless level=`skip`) | Yes | X.509 Subject DN patterns. Format: `x509.subject: CN=<val>, O=<val>, C=<val>`. Wildcards per RDN: `CN=*`. Special value `"*"` trusts all identities — **SHOULD NOT** be used in production. |

##### Margo-specific constraints on trust policy

1. Production deployments **MUST** use `signatureVerification.level: "strict"`.
2. `trustedIdentities: ["*"]` **SHOULD NOT** be used in production — it disables Subject DN enforcement.
3. At least one `ca:<name>` trust store **MUST** be configured when verification is active.
4. A `tsa:<name>` trust store **MUST** be configured when `override.timestampVerification` is `"enforce"` and OT asset lifetimes exceed signing certificate validity.
5. Trust policy file location **MUST** be `{NOTATION_CONFIG}/trustpolicy.json` where `{NOTATION_CONFIG}` is implementation-defined (default: `~/.config/notation/` on Linux).

##### Example: enterprise production policy

```json
{
  "version": "1.0",
  "trustPolicies": [
    {
      "name": "margo-production-strict",
      "registryScopes": [
        "harbor.enterprise.internal/margo-apps/*",
        "registry.vendor.com/certified-margo/*"
      ],
      "signatureVerification": {
        "level": "strict",
        "override": {
          "expiry": "enforce",
          "revocation": "enforce",
          "timestampVerification": "enforce"
        }
      },
      "trustStores": [
        "ca:enterprise-root-ca",
        "tsa:enterprise-rfc3161-tsa"
      ],
      "trustedIdentities": [
        "x509.subject: CN=*, O=Certified Margo Vendor Inc., C=DE",
        "x509.subject: CN=Margo Release Authority, O=Enterprise OT Corp., C=US"
      ]
    },
    {
      "name": "margo-dev-permissive",
      "registryScopes": ["localhost:5000/*"],
      "signatureVerification": {
        "level": "permissive"
      },
      "trustStores": ["ca:dev-self-signed"],
      "trustedIdentities": ["*"]
    }
  ]
}
```


---

### 9. Schema change report

| Dimension | Assessment |
|---|---|
| Schema file(s) modified | None |
| Change type | Non-breaking (OCI-layer operational concern only) |
| New LinkML elements | None |
| Modified LinkML elements | None |
| Removed LinkML elements | None |
| Backward compatibility | **MAINTAINED** |

**Rationale:**

| Question | Answer |
|---|---|
| Where are signatures stored? | Detached OCI referrer manifests — not in `margo.yaml` or DesiredState |
| How does WFM discover signatures? | OCI Referrers API (`GET /v2/{name}/referrers/{digest}`) |
| Does ApplicationDescription reference signatures? | No. Signatures reference the artifact via `subject.digest`. |
| Does ApplicationDescription need key fields? | No. Trust is managed via local trust policy + trust stores. |
| What about `keyLocation`? | Legacy PGP field. Not used by SUP-03. Editorial deprecation notice only. |
| Does DesiredState need changes? | No. Verification is operational at pull time. |

---

### 10. Informative notes

#### 10.1 Regulatory alignment — eIDAS Electronic Seals (INFORMATIVE)

> **INFORMATIVE NOTE — EU eIDAS Electronic Seals for Margo OCI Artifact Signing:**
>
> Application Developers operating within (or selling into) EU jurisdictions **MAY** use certificates issued by Qualified Trust Service Providers (QTSPs) conforming to eIDAS (EU Regulation 910/2014, updated by EU 2024/1183 "eIDAS 2.0") as a cost-effective, highly-trusted signing solution. eIDAS Electronic Seals (eSeals) provide a compelling middle ground between self-signed certificates (no external trust, no revocation) and operating a full private enterprise PKI (expensive, complex, requires internal CA governance).
>
> **Two eSeal levels are relevant to Margo artifact signing:**
>
> | eSeal Level | eIDAS Article | HSM Required | Trust Level | Cost (indicative) | Use Case |
> |---|---|---|---|---|---|
> | **Advanced Electronic Seal** | Art. 36 | **NO** — software key storage permitted | High — uniquely linked to creator, capable of identifying seal creator, linked to data such that subsequent change is detectable | €50–200/year | ISVs, small vendors, CI/CD pipelines where HSM infrastructure is not available |
> | **Qualified Electronic Seal** | Art. 38 | **YES** — QSCD (Qualified Seal Creation Device) required | Highest — legal presumption of integrity and origin under EU law | €200–800/year | Enterprise vendors, regulated industries, legal non-repudiation requirements |
>
> **Key advantages over private PKI:**
> - No CA infrastructure to operate — QTSP handles certificate lifecycle, OCSP, CRL, TSA
> - Publicly audited trust anchors — QTSP root CAs are on EU Trusted Lists (TSL), pre-trusted
> - Built-in revocation — OCSP and CRL operated by the QTSP
> - Built-in timestamping — most QTSPs offer RFC 3161 TSA as part of the service
> - Legal recognition across all 27 EU member states
>
> **Advanced eSeals (no HSM)** are particularly relevant for Margo because they allow automated CI/CD signing pipelines to operate with software-stored keys while still providing third-party trust, revocation infrastructure, and regulatory compliance. The signing certificate is issued by a QTSP and chains to a publicly-audited root CA, but the private key MAY be stored in a software keystore, cloud KMS, or CI/CD secret manager — no physical HSM hardware is required.
>
> **Integration with CNCF Notation:**
>
> To seal a Margo OCI artifact using a QTSP-issued eSeal certificate via Notation:
>
> ```bash
> # Sign with QTSP-issued Advanced eSeal certificate (software key)
> notation sign ${REGISTRY}@${DIGEST} \
>   --key qtsp-advanced-seal \
>   --signature-format cose \
>   --timestamp-url https://tsa.qtsp-provider.eu
> ```
>
> The resulting signature is indistinguishable from any other X.509 Notation signature at the verification layer. WFM and device verifiers require only the QTSP root CA in their trust store — they do not need to know whether the signing key was Advanced (software) or Qualified (HSM).
>
> **This alignment is entirely OPTIONAL and does not affect technical conformance with this specification.** Organizations outside the EU, or those preferring private PKI, are not affected by this note.

#### 10.2 ETSI EN 319 102-1 relationship (INFORMATIVE)

> **INFORMATIVE NOTE:** The verification engine in §7 is informed by ETSI EN 319 102-1 (Procedures for Creation and Validation of AdES Digital Signatures). The 10-step algorithm can stand on its own technical merits without mandatory ETSI compliance. Implementations seeking formal ETSI alignment should reference EN 319 102-1 V1.4.1+ directly.

#### 10.3 Industrial compliance mapping (INFORMATIVE)

| Standard | Requirement | How SUP-03 satisfies |
|---|---|---|
| IEC 62443-4-2 CR 3.9 | Software integrity verification | Content-addressable SHA-256 digest + X.509 signature verification |
| NIST SP 800-218 (SSDF) | Protect software from tampering | Notation signature bound to manifest digest |
| NIST SP 800-207 (Zero Trust) | Independent runtime verification | Local edge trust store + device-side mandatory verification |
| ISO/IEC 27001 A.12.6.2 | Software installation restrictions | Subject DN identity matching via trust policy |

---

### 11. OT environment considerations

#### 11.1 Air-gapped networks

The normative fallback requirements in §7.2 apply. Additionally:

- Trust stores and CRLs **SHOULD** be updated during planned maintenance windows.
- Operators **SHOULD** establish a maximum CRL cache staleness threshold (e.g., 30 days) appropriate to their threat model.
- Trust bundle updates **MAY** be delivered via USB maintenance keys, local network synchronization, or WFM-mediated distribution during connectivity windows.

#### 11.2 Hardware trust anchors

- TPM 2.0 for trust store sealing is **RECOMMENDED** but not required.
- Edge devices without TPM **MUST** protect trust store integrity via filesystem permissions, dm-verity, or equivalent tamper-detection mechanisms.
- Implementations **MUST NOT** require specific hardware security modules — the specification supports software-based verification on constrained devices.

#### 11.3 Constrained devices

- The verification algorithm is specified abstractly so that C, Rust, or other native implementations are possible without a Go runtime or the full Notation CLI.
- Edge device implementations **MUST** support COSE_Sign1 parsing (binary, compact) as the primary envelope format.

#### 11.4 Minimum viable verification profile

For initial Margo compliance, implementations **MUST** support at minimum:

1. Digest binding verification (Step 8)
2. Certificate chain validation to a configured trust anchor (Step 6)
3. Subject identity matching (Step 7)

TSA timestamp evaluation (Step 9) and revocation checks (Step 10) are **REQUIRED** only when `signatureVerification.level` is `strict`. Implementations supporting only `skip` and a reduced `strict` profile (steps 6–8) **MAY** claim partial conformance during the v1-alpha1 transition period.

> **Partial conformance:** Implementations claiming partial conformance **SHOULD** document which verification steps they support in their product documentation and **SHOULD** expose their conformance level via a capability query or configuration attribute so that operators can assess the security posture of their deployment.


---

### 12. Conformance impact

| RFC 2119 Keyword | Statement |
|---|---|
| **MUST** | Application Developers MUST sign all three artifact tiers (Application Package, component archives, container images). |
| **MUST** | Application Developers MUST verify their own signatures before publishing (pre-publish validation). |
| **MUST** | Application Developers MUST provide a vendor trust bundle containing root CA certificate(s) and trust policy template. |
| **MUST** | Application Developers MUST embed the complete X.509 certificate chain in the signature envelope. |
| **MUST** | All Margo-defined OCI artifact types MUST be signed (Application Package, Compose Archive, Helm Chart, Container Image). |
| **MUST** | Signatures MUST be stored as detached OCI referrer manifests with `artifactType: application/vnd.cncf.notary.signature`. |
| **MUST** | Signature payload MUST bind to the target artifact's SHA-256 manifest digest. |
| **MUST** | Verifiers MUST support both COSE_Sign1 and JWS envelope formats. |
| **MUST** | Edge devices MUST validate signatures before expanding archives or deploying workloads. |
| **MUST** | WFM MUST validate signatures when trust policy level ≠ `skip`. |
| **MUST** | Default verification level (absent trust policy) MUST be `strict`. |
| **MUST** | Production deployments MUST use `strict` verification level. |
| **MUST** | Registries hosting verifiable Margo artifacts MUST support OCI Distribution Spec v1.1 Referrers API. |
| **MUST** | If OCSP is unreachable, implementations MUST fall back to locally cached CRLs. |
| **MUST NOT** | Self-signed mode MUST NOT be used in production OT environments. |
| **MUST NOT** | `skip` verification level MUST NOT be used in production. |
| **MUST NOT** | Signature referrer manifests MUST NOT be garbage-collected while target artifact exists. |
| **SHOULD** | WFM SHOULD validate signatures before catalog ingestion. |
| **SHOULD** | Application Developers SHOULD publish trust bundles to the well-known `_trust-bundle` repository path. |
| **SHOULD** | Application Developers SHOULD produce COSE_Sign1 envelopes for production artifacts. |
| **SHOULD** | Application Developers SHOULD include RFC 3161 TSA counter-signatures. |
| **SHOULD** | Edge devices SHOULD support `permissive` and `audit` levels for migration. |
| **RECOMMENDED** | CNCF Notary Project Notation is the RECOMMENDED implementation. |
| **RECOMMENDED** | TPM 2.0 for trust store sealing is RECOMMENDED. |
| **MAY** | Multiple signatures MAY exist for a single artifact. |
| **MAY** | EU deployments MAY use QTSP certificates (Advanced or Qualified eSeals) for eIDAS alignment. |

---

### 13. Backward compatibility

This SUP is **additive-only**. No existing OCI manifests, LinkML schemas, or `margo.yaml` structures are modified.

| Dimension | Impact |
|---|---|
| OCI manifest structure | None — signatures are referrers, not embedded |
| `margo.yaml` / ApplicationDescription | None — no new fields |
| WFM ingest pipeline | Additive — new verification step (policy-driven) |
| Edge device agent | Additive — new pre-deployment verification gate |
| Registry requirements | OCI v1.1 Referrers API required (soft upgrade) |
| Existing unsigned artifacts | Remain deployable unless operator configures `strict` with no fallback |

Implementations that do not yet support signature verification can continue operating unchanged. The phased timeline (§6.2) ensures ecosystem readiness before enforcement becomes mandatory.

---

### 14. References

- [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md)
- [OCI Image Specification v1.1.0](https://github.com/opencontainers/image-spec/blob/v1.1.0/manifest.md)
- [CNCF Notary Project — Notation Specification](https://github.com/notaryproject/specifications)
- [CNCF Notary Project — Trust Policy Specification](https://github.com/notaryproject/specifications/blob/main/specs/trust-store-trust-policy.md)
- [RFC 9052 — CBOR Object Signing and Encryption (COSE)](https://www.rfc-editor.org/rfc/rfc9052)
- [RFC 7515 — JSON Web Signature (JWS)](https://www.rfc-editor.org/rfc/rfc7515)
- [RFC 3161 — Internet X.509 PKI Time-Stamp Protocol (TSP)](https://www.rfc-editor.org/rfc/rfc3161)
- [RFC 6960 — Online Certificate Status Protocol (OCSP)](https://www.rfc-editor.org/rfc/rfc6960)
- [RFC 2119 — Key words for use in RFCs to Indicate Requirement Levels](https://www.rfc-editor.org/rfc/rfc2119)
- [ETSI EN 319 102-1 — Procedures for Creation and Validation of AdES Digital Signatures](https://www.etsi.org/deliver/etsi_en/319100_319199/31910201/)
- [eIDAS Regulation (EU) No 910/2014](https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=uriserv:OJ.L_.2014.257.01.0073.01.ENG)
- [IEC 62443-4-2 — Security for Industrial Automation and Control Systems](https://webstore.iec.ch/publication/34421)
- [NIST SP 800-218 — Secure Software Development Framework (SSDF)](https://csrc.nist.gov/publications/detail/sp/800-218/final)

## Alternatives considered

### Alternative A — Cosign (Sigstore) keyless signing

[Sigstore Cosign](https://github.com/sigstore/cosign) provides keyless signing using ephemeral certificates from the Fulcio CA, with transparency logging via Rekor. Rejected for Margo because:

1. **Online dependency** — Keyless verification requires querying the Rekor transparency log. Air-gapped OT edge devices cannot reach public Sigstore infrastructure.
2. **No long-term verifiability** — Fulcio certificates have 10-minute validity. Without Rekor access, signatures become unverifiable. OT assets with 10+ year lifetimes require offline-capable verification.
3. **OIDC identity model** — Cosign ties signatures to OIDC identities (email, GitHub Actions workflow), not X.509 Subject DNs. This does not align with industrial PKI practices or IEC 62443 certificate requirements.

Cosign's OCI storage model (also uses Referrers API) is compatible. Organizations already using Sigstore **MAY** store cosign signatures alongside Notation signatures — verifiers select based on `artifactType` filtering.

### Alternative B — In-band signature fields in ApplicationDescription

Adding `signature` or `signatureDigest` fields to the LinkML schema was rejected because:

1. **Circular dependency** — Signing the manifest that contains its own signature reference creates a chicken-and-egg problem.
2. **Schema coupling** — Every schema version change would require signature workflow updates.
3. **Single-signature limitation** — In-band fields support only one signature per artifact. Detached referrers support unlimited signatures (vendor + enterprise counter-signatures).
4. **Registry-standard approach** — OCI Referrers API is the industry-standard mechanism for artifact accessories (signatures, SBOMs, attestations).

### Alternative C — PGP detached signatures (status quo)

The existing specification recommends PGP signing with `keyLocation`. Rejected as the long-term solution because:

1. **No standard OCI storage** — PGP signatures have no defined location in OCI registries, preventing automated discovery.
2. **No timestamping** — PGP has no native RFC 3161 equivalent. Signatures become unverifiable after key expiry.
3. **No revocation infrastructure** — PGP key revocation relies on keyserver propagation, which is unreliable in OT environments.
4. **Ecosystem decline** — Major registries and supply-chain tools (Notation, Cosign, ORAS) have converged on X.509/OCI; PGP tooling is not being maintained for container ecosystems.

The `keyLocation` field is retained with an editorial deprecation notice (not removed in this SUP to avoid a breaking change).

### Alternative D — Embedded signatures in OCI manifest annotations

Storing signature bytes directly in the target manifest's `annotations` field was rejected because:

1. **Digest mutation** — Adding annotations changes the manifest digest, invalidating existing references.
2. **Size limitations** — X.509 chains + COSE/JWS envelopes can exceed annotation size limits on some registries.
3. **No multi-signature support** — Only one signature fits in annotations without complex multiplexing.

## Rejection reason

> Not applicable — SUP is in active development.

---

*Prepared by Andrii Melashchenko (Belden Inc.), 2026-07-26. Subject to the Open Web Foundation Contributor License Agreement governing the Margo specification.*
