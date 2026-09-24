# SUP-03 Implementation Guide: OCI Artifact Signing

| | |
|---|---|
| Status | Informative (non-normative companion to SUP-03) |
| Author | Andrii Melashchenko, Belden Inc. |
| Date | 2026-09-24 |

## 1. Purpose

This document is an informative companion to [SUP-03: OCI Artifact Signing for Margo](sup-03-oci-signature.md). It provides implementation guidance for the Notary Project signing workflow, PKI deployment advice for different operational environments, and practical examples using the `notation` CLI. This guide does not define requirements — all normative obligations are in SUP-03.

## 2. How Notary Project signing works

### 2.1 Detached referrer architecture

The Notary Project stores signatures as separate OCI artifacts linked to the signed artifact via the OCI Referrers API. The signature never modifies the target artifact's manifest or digest. This detached model is additive-only — new signatures can be attached at any time without affecting existing ones. Multiple parties can independently sign the same artifact. Because signatures reference the target by digest, they remain valid regardless of the registry path the artifact is stored at.

```
┌─────────────────────────────┐
│  Target Artifact Manifest   │◄── sha256:abc123...
│  (e.g., Application Package)│
└──────────────┬──────────────┘
               │ subject.digest
               │
┌──────────────┴──────────────┐
│  Signature Referrer Manifest│    artifactType:
│  (OCI Image Manifest)       │    application/vnd.cncf.notary.signature
└──────────────┬──────────────┘
               │ layers[0]
               │
┌──────────────┴──────────────┐
│  Signature Envelope Blob    │    mediaType:
│  (COSE_Sign1 or JWS)        │    application/cose  -or-
└─────────────────────────────┘    application/jose+json
```

### 2.2 Signature manifest structure

A Notary Project v1.1.0 signature manifest is a standard OCI image manifest with a `subject` field pointing to the signed artifact:

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "config": {
    "mediaType": "application/vnd.cncf.notary.signature",
    "digest": "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "size": 0
  },
  "layers": [
    {
      "mediaType": "application/cose",
      "digest": "sha256:9834876dcfb05cb167a5c24953eba58c4ac89b1adf57f28f2f9d09af107ee8f0",
      "size": 1024
    }
  ],
  "subject": {
    "mediaType": "application/vnd.oci.image.manifest.v1+json",
    "digest": "sha256:abc123def456789...",
    "size": 1234
  },
  "annotations": {
    "io.cncf.notary.x509chain.thumbprint#S256": "[\"<leaf-cert-fingerprint>\"]"
  }
}
```

The `config` descriptor has `mediaType` set to `application/vnd.cncf.notary.signature` with an empty blob. The single layer contains the signature envelope (COSE or JWS). The `subject` field links to the target artifact by digest.

### 2.3 Discovery protocol

A verifier discovers signatures by querying the OCI Referrers API:

```
GET /v2/{name}/referrers/{digest}?artifactType=application/vnd.cncf.notary.signature
```

The response is an OCI Image Index listing all signature manifests attached to the target digest:

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.index.v1+json",
  "manifests": [
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "digest": "sha256:sig-manifest-digest...",
      "size": 512,
      "artifactType": "application/vnd.cncf.notary.signature",
      "annotations": {
        "io.cncf.notary.x509chain.thumbprint#S256": "[\"<fingerprint>\"]"
      }
    }
  ]
}
```

### 2.4 What the signature protects

The signature binds to the target artifact's manifest digest. Since the OCI manifest references all layers (blobs) by their content-addressable digests, verifying the manifest signature guarantees the integrity of every layer *that this manifest references*. Any modification to any referenced layer changes its digest, which changes the manifest content, which invalidates the signature.

**Scope: a single manifest.** This guarantee is digest-bound to one manifest only. It does **not** extend across artifact tiers. References between the Application Package, Component, and Container Image tiers use mutable OCI tags (`revision`), not digests, so a valid Application Package signature gives no cryptographic assurance about the specific container images ultimately resolved. That cross-tier mutable-tag gap is recorded as a known limitation in SUP-03 §7 (limitation 1) and is deferred to a future SUP that carries pinned digests with the deployment. Do not read "digest-bound" as covering the whole tier graph.

## 3. Verification walkthrough

### 3.1 The 10 verification steps

| Step | What happens | On failure |
|---|---|---|
| 1. Manifest retrieval | Fetch the target artifact manifest by digest | Fail — artifact not found |
| 2. Referrers discovery | Query Referrers API for signatures on the manifest digest | Fail if no signatures found and policy is `strict` |
| 3. Fingerprint filtering | Filter candidate signatures by certificate fingerprint in annotations | Skip non-matching signatures |
| 4. Trust policy scope matching | Match the artifact's registry scope against configured trust policy entries | Fail — no policy covers this scope |
| 5. Envelope parsing | Parse the COSE_Sign1 or JWS envelope from the signature blob | Fail — malformed envelope |
| 6. Certificate chain validation | Validate the signing certificate chain against the configured trust store (RFC 5280 path validation) | Fail — untrusted or invalid chain |
| 7. Subject identity matching | Verify the signing certificate's subject matches `trustedIdentities` in the trust policy | Fail — identity not trusted |
| 8. Digest binding verification | Confirm the signed payload's target digest matches the artifact digest being verified | Fail — digest mismatch |
| 9. Timestamp evaluation | If the signing cert is expired, check for a valid RFC 3161 countersignature proving the signature was created before `notAfter` | Fail if no valid timestamp and cert is expired |
| 10. Revocation check | If the operator's trust policy enables revocation, check the vendor code-signing PKI's CRL or OCSP endpoint for the leaf certificate's status | Fail — certificate revoked |

**Revocation is a vendor-PKI and operator-policy behaviour, not a MIAF requirement.** Revocation here is a property of the *publisher's code-signing PKI* (the CA that issued the signing leaf certificate) and of the *operator's trust policy*, which decides whether to consult CRL/OCSP and how to fail when a status is unavailable. SUP-03 §4 does not mandate CRL or OCSP. This is deliberately independent of — and does not contradict — MIAF's SVID revocation stance: MIAF (see the [Margo Identity and Authorization Framework](../../completed/margo-identity-and-authorization-framework.md), §3 cryptographic requirements and §5 certificate validation) uses **no CRL or OCSP** for SVIDs and forbids AIA fetching. The two answer different questions: MIAF revokes *transport identities* through short SVID lifetimes and operator playbooks, whereas an artifact signature is validated against a *code-signing PKI* that may legitimately publish a CRL or run an OCSP responder.

**Air-gapped handling.** Online CRL/OCSP endpoints are frequently unreachable in air-gapped OT environments. When online revocation cannot be reached, operators SHOULD rely on the mechanisms that do not require connectivity:

- **Short certificate lifetimes** on the signing leaf, so a compromised key ages out quickly without a revocation lookup.
- **Trust-anchor rotation** — retiring and replacing the publisher root/intermediate the operator has installed, which invalidates every certificate beneath it.
- **Operator trust-policy choice** — configuring the policy to treat an unreachable revocation endpoint as either soft-fail (accept with log) or hard-fail per site risk tolerance, rather than assuming an online check is always possible.

These are operator and vendor-PKI concerns; none of them is derived from MIAF.

### 3.2 Behavior by verification level

| Level | Crypto failure | Expired cert (no TSA) | Revoked cert | No signatures found |
|---|---|---|---|---|
| `strict` | Reject | Reject | Reject | Reject |
| `permissive` | Reject | Accept with log | Reject | Reject |
| `audit` | Log and accept | Log and accept | Log and accept | Log and accept |
| `skip` | Not evaluated | Not evaluated | Not evaluated | Not evaluated |

These semantics are defined in the Notary Project [trust-store-trust-policy.md](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/trust-store-trust-policy.md) and adopted by SUP-03 by reference. The "Revoked cert" column applies only when the operator's trust policy enables revocation against the vendor code-signing PKI (see step 10); it is not a MIAF-derived requirement, and in an air-gapped deployment where CRL/OCSP is unreachable the operator's policy determines whether an unavailable status is treated as accept-with-log or reject.

## 4. PKI deployment models

### 4.1 Self-Signed (Development and Laboratory)

**What:** A locally-generated X.509 certificate hierarchy with a self-signed root CA and a leaf signing certificate.

**When:** Laboratory environments, CI/CD pipelines, air-gapped testbeds, rapid prototyping. Not suitable for production.

**Quick workflow:**

```bash
# Generate root CA (valid 10 years)
openssl req -x509 -sha256 -nodes -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
  -keyout root-ca.key -out root-ca.crt -days 3650 -subj "/CN=Lab Root CA"

# Generate leaf signing cert (valid 1 year)
openssl req -new -nodes -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
  -keyout leaf.key -out leaf.csr -subj "/CN=developer@example.com"
openssl x509 -req -in leaf.csr -CA root-ca.crt -CAkey root-ca.key \
  -CAcreateserial -out leaf.crt -days 365 \
  -extfile <(printf "keyUsage=digitalSignature\nextendedKeyUsage=codeSigning")

# Configure notation
notation cert add --type ca --store lab-store root-ca.crt
notation key add --name lab-key --plugin-config "key=leaf.key" --plugin-config "cert=leaf.crt"

# Sign
notation sign --key lab-key registry.example.com/app-package@sha256:abc123...

# Verify
notation verify registry.example.com/app-package@sha256:abc123...
```

**Limitations:** No revocation mechanism, no timestamping authority, no third-party trust anchoring. Suitable only for non-production environments.

### 4.2 Enterprise PKI (Production)

**What:** Organization-operated CA infrastructure providing certificate lifecycle management, revocation services (OCSP/CRL), and timestamping.

**When:** Production OT environments, corporate deployments, IEC 62443 compliance scenarios.

**Architecture:**

```
┌──────────────┐
│   Root CA    │  (offline, HSM-backed)
└──────┬───────┘
       │
┌──────┴───────┐
│Intermediate CA│  (online, issues leaf certs)
└──────┬───────┘
       │
┌──────┴───────┐    ┌─────────┐    ┌─────────┐
│  Leaf Cert   │    │  OCSP   │    │   TSA   │
│ (code signing)│    │Responder│    │ Server  │
└──────────────┘    └─────────┘    └─────────┘
```

**Advantages:**

| Capability | Benefit |
|---|---|
| Certificate revocation (OCSP/CRL) | Immediate invalidation of compromised keys |
| RFC 3161 timestamping | Signatures remain valid beyond certificate expiry |
| HSM key storage | Private keys never exported in plaintext |
| Audit trail | Full issuance and signing history |

**Quick workflow:**

```bash
# Configure notation with KMS-backed key
notation key add --name prod-key --id "arn:aws:kms:..." --plugin aws-kms

# Sign with timestamping
notation sign --key prod-key --timestamp-url https://tsa.corp.example.com \
  registry.example.com/app-package@sha256:abc123...

# Verify
notation verify registry.example.com/app-package@sha256:abc123...
```

### 4.3 eIDAS QTSP (Regulated Production)

**What:** Signing certificates issued by an EU Qualified Trust Service Provider, providing legal standing under the eIDAS regulation.

**When:** EU-regulated environments, scenarios where the organization does not operate its own CA infrastructure, multi-vendor supply chains requiring cross-organizational trust.

**Two eSeal levels:**

| Level | Key storage | Legal standing |
|---|---|---|
| Advanced Electronic Seal | Software key or standard HSM | Legally recognized, non-qualified |
| Qualified Electronic Seal | Qualified Signature Creation Device (QSCD) | Highest legal standing under eIDAS |

**Quick workflow:**

```bash
# Configure notation with QTSP-issued certificate (PKCS#11 or KMS)
notation key add --name qtsp-key --plugin pkcs11 --id "pkcs11:token=eIDAS;object=seal-key"

# Sign with QTSP's timestamping service
notation sign --key qtsp-key --timestamp-url https://tsa.qtsp-provider.eu \
  registry.example.com/app-package@sha256:abc123...

# Verify (root CA from EU Trusted List)
notation verify registry.example.com/app-package@sha256:abc123...
```

The EU Trusted List of QTSPs is available at: <https://eidas.ec.europa.eu/efda/tl-browser/>

### 4.4 Comparison matrix

| Criterion | Self-Signed | Enterprise PKI | eIDAS QTSP |
|---|---|---|---|
| Trust anchor | Self-generated root | Organization root CA | QTSP root (EU Trusted List) |
| Key storage | File system | HSM recommended | QSCD required (Qualified) |
| Revocation | None | OCSP / CRL | OCSP / CRL (provider-managed) |
| Timestamping | None | Organization TSA | Provider TSA |
| Cost | Free | Infrastructure + operational | Per-certificate or subscription |
| Production suitable | No | Yes | Yes |
| Regulatory alignment | None | IEC 62443, internal policy | eIDAS, EU CRA, IEC 62443 |

## 5. Publisher signing material examples

### 5.1 Directory structure

Application Developers package their signing material with the following layout:

```
acme-corp-signing-material/
├── certificates/
│   ├── root-ca.crt            # PEM-encoded root CA certificate
│   └── tsa-root-ca.crt        # PEM-encoded TSA root CA (if timestamping used)
├── trustpolicy-template.json   # Trust policy with registryScopes placeholder
└── README.md                   # Installation instructions
```

### 5.2 Trust policy template

```json
{
  "version": "1.0",
  "trustPolicies": [
    {
      "name": "acme-corp-artifacts",
      "registryScopes": [
        "REPLACE_WITH_YOUR_REGISTRY/acme-corp/*"
      ],
      "signatureVerification": {
        "level": "strict"
      },
      "trustStores": [
        "ca:acme-corp"
      ],
      "trustedIdentities": [
        "x509.subject: C=US, O=Acme Corp, CN=release-signing@acme-corp.example.com"
      ]
    }
  ]
}
```

### 5.3 Operator installation

1. **Copy root CA certificate** to the notation trust store directory:
   ```bash
   mkdir -p ~/.config/notation/truststore/x509/ca/acme-corp/
   cp certificates/root-ca.crt ~/.config/notation/truststore/x509/ca/acme-corp/
   ```

2. **Copy TSA root CA** (if timestamping is used):
   ```bash
   mkdir -p ~/.config/notation/truststore/x509/tsa/acme-corp/
   cp certificates/tsa-root-ca.crt ~/.config/notation/truststore/x509/tsa/acme-corp/
   ```

3. **Edit registryScopes** in `trustpolicy-template.json` to match your registry configuration:
   ```bash
   sed -i 's|REPLACE_WITH_YOUR_REGISTRY|registry.yourorg.example.com|g' trustpolicy-template.json
   ```

4. **Merge into your trust policy** at `~/.config/notation/trustpolicy.json`:
   ```bash
   # If no existing policy, copy directly:
   cp trustpolicy-template.json ~/.config/notation/trustpolicy.json

   # If existing policy, append the trust policy entry to the trustPolicies array.
   ```

## 6. References

- CNCF Notary Project Specifications v1.1.0 — <https://github.com/notaryproject/specifications/tree/v1.1.0>
- Notation CLI — <https://github.com/notaryproject/notation>
- OCI Distribution Specification v1.1.0 — <https://github.com/opencontainers/distribution-spec/tree/v1.1.0>
- OCI Image Specification v1.1.0 — <https://github.com/opencontainers/image-spec/tree/v1.1.0>
- IETF RFC 3161 — Internet X.509 PKI Time-Stamp Protocol
- IETF RFC 5280 — Internet X.509 PKI Certificate and CRL Profile
- IETF RFC 7515 — JSON Web Signature (JWS)
- IETF RFC 9052 — CBOR Object Signing and Encryption (COSE): Structures and Process
- EU Trusted List Browser — <https://eidas.ec.europa.eu/efda/tl-browser/>

---

*This document is informative. All normative requirements are in [SUP-03](sup-03-oci-signature.md).*

*Prepared by Andrii Melashchenko (Belden Inc.), 2026-09-24.*
