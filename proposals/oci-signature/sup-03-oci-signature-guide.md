# SUP-03 Implementation Guide: OCI Artifact Signing

**Status:** Informative (non-normative companion to [SUP-03](sup-03-oci-signature.md))
**Author:** Andrii Melashchenko, Belden Inc.
**Date:** 2026-07-29

---

## 1. Purpose and scope

This document is the informative companion to the normative SUP-03 specification ("OCI Cryptographic Signature Architecture and Trust Verification Framework"). It provides:

- Implementation guidance for the Notary Project signing workflow
- PKI deployment advice across three deployment models
- Operational recommendations for OT environments
- Regulatory context and compliance mapping
- Migration paths between deployment models

**This document does NOT define requirements.** All normative MUST/SHOULD/MAY statements are in the [normative SUP-03 specification](sup-03-oci-signature.md). This guide references those requirements and explains how to satisfy them in practice across different deployment scenarios.

**Intended audience:** DevSecOps engineers, security architects, OT operators, compliance officers, and ISVs implementing Margo artifact signing.

---

## 2. How Notary Project signing works

This section provides background on the OCI signature storage model for readers unfamiliar with the CNCF Notary Project. It is informative context — the normative requirements are in SUP-03 §1–§4.

### 2.1 Detached referrer architecture

Signatures in the Notary Project model are **not** embedded inside the artifact they protect. Instead, they are stored as separate OCI manifests that **refer** to the signed artifact via the OCI Distribution Spec v1.1 `subject` field. This is called the "detached referrer" pattern:

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

Key properties of this architecture:

- **Additive-only** — adding a signature never modifies the target artifact's manifest or digest
- **Multiple signatures** — a single artifact can have many signatures (vendor + enterprise counter-signature)
- **Path-independent** — signatures bind to digests, not registry hostnames or repository paths, so mirroring works without re-signing

### 2.2 Signature referrer manifest structure

The signature manifest is a standard OCI image manifest. In the `v1.1.0` shape this SUP pins, the manifest carries no top-level `artifactType`; the config descriptor's `mediaType` identifies it as a Notary signature instead:

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "config": {
    "mediaType": "application/vnd.cncf.notary.signature",
    "digest": "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
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

This is the `v1.1.0` manifest shape SUP-03 pins (§1): no top-level `artifactType`; the config descriptor's `mediaType` carries the signature type instead, and its digest is the SHA-256 of the two-byte `{}` payload. The Notary Project `main` branch has since introduced a different default shape — a top-level `artifactType` paired with `config.mediaType: application/vnd.oci.empty.v1+json` — under which the shape above is termed the legacy format that verifiers SHOULD continue to accept. SUP-03 pins `v1.1.0` because it is the current released version; adopting the newer shape is an Open WG decision pending its release (see SUP-03 §1).

- The `subject` field creates the Referrers API linkage — this is how verifiers discover signatures for a given artifact.
- The `layers[0]` entry points to the actual cryptographic envelope (COSE or JWS).
- The `annotations` fingerprint enables pre-filtering signatures by certificate thumbprint before downloading the full envelope.

### 2.3 Discovery protocol

Verifiers discover signatures for a target artifact by querying the OCI Referrers API:

```
GET /v2/{name}/referrers/{digest}?artifactType=application/vnd.cncf.notary.signature
```

The response is an OCI Image Index listing all signature referrer manifests whose `subject.digest` matches the queried digest. A typical response:

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.index.v1+json",
  "manifests": [
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "digest": "sha256:1c4d5e...",
      "size": 842,
      "artifactType": "application/vnd.cncf.notary.signature",
      "annotations": {
        "io.cncf.notary.x509chain.thumbprint#S256": "[\"abc123...\"]"
      }
    }
  ]
}
```

The verifier then downloads each candidate signature manifest, retrieves the envelope blob from its layer, and runs the verification algorithm.

### 2.4 What the signature protects

The signature payload binds to the target artifact's OCI manifest digest:

```json
{
  "targetArtifact": {
    "mediaType": "application/vnd.oci.image.manifest.v1+json",
    "digest": "sha256:<manifest-digest>",
    "size": 1024
  }
}
```

Because OCI manifests list all layer blob digests in their `layers[]` array, signing the manifest transitively protects all content (margo.yaml, container layers, Helm charts, etc.). Any modification to any layer changes the manifest digest, invalidating the signature.

---

## 3. Verification walkthrough

This section provides an informative explanation of the 10-step verification algorithm. The normative definition is in SUP-03 §6. For full implementation details, see the Notary Project [signing-and-verification-workflow.md](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/signing-and-verification-workflow.md).

### 3.1 The 10 verification steps

| Step | What happens | On failure |
|------|-------------|------------|
| 1. **Manifest retrieval** | Retrieve the target artifact manifest by its content-addressable SHA-256 digest | Artifact not found — abort |
| 2. **Referrers discovery** | Query the Referrers API for signatures (`artifactType=application/vnd.cncf.notary.signature`) | No signatures found — reject (unless level = `skip`) |
| 3. **Fingerprint filtering** | Pre-filter candidate signatures by comparing the `x509chain.thumbprint` annotation against certificates in configured trust stores | Skip non-matching signatures; if none remain, reject |
| 4. **Trust policy scope matching** | Match the artifact's repository URI against `registryScopes` patterns in the local trust policy | No matching policy — reject |
| 5. **Envelope parsing** | Download the signature envelope blob and parse it (COSE_Sign1 or JWS) | Malformed envelope — reject this signature, try next |
| 6. **Certificate chain validation** | Build a path from the leaf certificate (embedded in envelope) to a trust anchor in configured `ca:<name>` trust stores; verify Key Usage = `digitalSignature` and EKU = Code Signing | Untrusted chain or invalid profile — reject |
| 7. **Subject identity matching** | Compare the leaf certificate's Subject DN against `trustedIdentities` patterns in the trust policy | Identity not in allowlist — reject |
| 8. **Digest binding verification** | Verify the cryptographic signature over the payload; confirm `targetArtifact.digest` matches the actual target manifest digest | Invalid signature or digest mismatch — reject |
| 9. **Timestamp evaluation** | If the signing certificate has expired: verify that an RFC 3161 TSA token proves the signature was created before `notAfter` | (strict) Expired cert without valid timestamp — reject |
| 10. **Revocation check** | Query OCSP for certificate status; fall back to locally cached CRLs if OCSP is unreachable | (strict) Revoked certificate — reject |

### 3.2 Verification flow summary

```
Artifact digest ──► Referrers API ──► Candidate signatures
                                            │
                                    ┌───────┴───────┐
                                    │ For each      │
                                    │ candidate:    │
                                    ▼               │
                              Parse envelope        │
                                    │               │
                              Build cert chain      │
                                    │               │
                              Match identity        │
                                    │               │
                              Verify signature      │
                                    │               │
                              Check timestamp       │
                                    │               │
                              Check revocation      │
                                    │               │
                              ┌─────┴─────┐         │
                              │ ALL PASS? │         │
                              └─────┬─────┘         │
                                YES │    NO ────────┘
                                    ▼         (try next)
                              ACCEPT artifact
```

If at least one signature passes all checks, the artifact is accepted. If no candidate signature passes, the artifact is rejected (at `strict` or `permissive` level) or the failure is logged (at `audit` level).

### 3.3 Behavior by verification level

| Level | Crypto failure | Expired cert (no TSA) | Revoked cert | No signatures |
|-------|---------------|----------------------|--------------|---------------|
| `strict` | Reject | Reject | Reject | Reject |
| `permissive` | Reject | Warn + proceed | Warn + proceed | Reject |
| `audit` | Log + proceed | Log + proceed | Log + proceed | Log + proceed |
| `skip` | Not checked | Not checked | Not checked | Not checked |


---

## 4. PKI deployment models

SUP-03 supports three PKI deployment models. The normative specification defines the cryptographic and structural requirements that all three models satisfy identically at the verification layer. This section provides practical guidance for selecting and implementing the appropriate model.

### 4.1 Mode A: Self-Signed PKI (Development and Laboratory)

#### What it is

A locally-generated X.509 certificate hierarchy created by the application developer using standard tools (OpenSSL, cfssl, step-ca). The developer acts as their own Certificate Authority.

#### When to use

- Laboratory and prototyping environments
- CI/CD pipeline testing (pre-production)
- Air-gapped testbeds with no external connectivity
- Learning and evaluating the Margo signing workflow
- Internal demos and proof-of-concept deployments

#### When NOT to use

- Production OT environments (SUP-03: self-signed mode MUST NOT be used in production)
- Any deployment subject to IEC 62443, NIST SP 800-218, or eIDAS compliance
- Multi-vendor environments where trust must be established between organizations

#### Limitations

| Limitation | Impact |
|---|---|
| No revocation mechanism | If a signing key is compromised, the only remedy is removing the root CA from ALL trust stores manually |
| No RFC 3161 timestamping | Signatures become unverifiable after the signing certificate expires (typically 1–3 years) |
| No third-party trust | Any developer can generate a certificate claiming any Subject DN — there is no external validation |
| No audit trail | No external CA logs, no certificate transparency, no OCSP history |

#### Quick-start workflow

```bash
# 1. Generate a local root CA (4096-bit RSA, 10-year validity)
openssl req -x509 -new -nodes \
  -newkey rsa:4096 -sha256 -days 3650 \
  -subj "/C=US/O=My Dev Lab/CN=Margo Dev Root CA" \
  -keyout private/rootCA.key -out certs/rootCA.crt

# 2. Generate code-signing leaf key (ECDSA P-256)
openssl ecparam -name prime256v1 -genkey -out private/developer.key

# 3. Create CSR with Code Signing EKU
openssl req -new -key private/developer.key \
  -subj "/C=US/O=My Dev Lab/OU=Engineering/CN=Dev Signer" \
  -out developer.csr

# 4. Sign with Root CA (include Code Signing EKU)
openssl x509 -req -in developer.csr \
  -CA certs/rootCA.crt -CAkey private/rootCA.key -CAcreateserial \
  -out certs/developer.crt -days 365 -sha256 \
  -extfile <(printf "basicConstraints=CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=critical,codeSigning")

# 5. Configure Notation
notation key add --default margo-dev-key \
  --key private/developer.key --cert certs/developer.crt
notation cert add --type ca --store margo-dev-store certs/rootCA.crt

# 6. Sign an artifact
notation sign ${REGISTRY}@${DIGEST} --key margo-dev-key --signature-format cose

# 7. Verify locally
notation verify ${REGISTRY}@${DIGEST}
```

#### Publisher signing material delivery

Manual distribution: provide `rootCA.crt` + `trustpolicy-template.json` to the verifying party via secure channel (USB key, encrypted email, internal wiki).


### 4.2 Mode B: Corporate/Enterprise PKI (Production)

#### What it is

An organization-operated Certificate Authority infrastructure that issues code-signing certificates to authorized developers and maintains revocation/timestamping services. Common implementations: Microsoft AD CS, EJBCA, HashiCorp Vault PKI Secrets Engine, AWS Private CA, Google Cloud CAS.

#### When to use

- Production OT environments
- Corporate environments with existing PKI infrastructure
- Deployments requiring key compromise recovery (OCSP/CRL revocation)
- Multi-team organizations needing centralized certificate governance
- Environments subject to IEC 62443 or NIST SP 800-218 compliance

#### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Enterprise PKI Hierarchy                   │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  [Enterprise Root CA]  (offline, HSM-protected)              │
│         │                                                    │
│         ├── [Intermediate CA - Code Signing]                 │
│         │         │                                          │
│         │         ├── Leaf: Developer A (code-signing)       │
│         │         ├── Leaf: CI/CD Pipeline (code-signing)    │
│         │         └── Leaf: Release Manager (code-signing)   │
│         │                                                    │
│         └── [Intermediate CA - TSA]                          │
│                   │                                          │
│                   └── TSA Signing Certificate                │
│                                                              │
│  [OCSP Responder]  ←── queries from WFM Clients             │
│  [CRL Distribution Point]  ←── periodic download            │
│  [RFC 3161 TSA]  ←── timestamp requests from CI/CD          │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

#### Advantages

| Advantage | Description |
|---|---|
| Full revocation | OCSP (real-time) + CRL (offline fallback) — compromised keys can be invalidated |
| RFC 3161 TSA | Internal timestamping service ensures signatures remain valid beyond cert expiry |
| HSM key storage | Private keys stored in FIPS 140-2 Level 3 hardware — never exportable |
| Organizational control | Central team governs who can sign, with what identity, for how long |
| Audit trail | CA logs all certificate issuance; OCSP logs all verification queries |
| Long-term verifiability | TSA timestamps + CRL archival enable 10+ year signature validation |

#### Key storage options

| Option | Mechanism | Security Level | Use Case |
|---|---|---|---|
| PKCS#11 HSM | YubiHSM2, SafeNet Luna, Thales | FIPS 140-2 Level 3 | On-premise signing stations |
| Cloud KMS | AWS KMS, Azure Key Vault, GCP Cloud KMS | FIPS 140-2 Level 2/3 | Cloud CI/CD pipelines |
| HashiCorp Vault Transit | Vault Transit backend | Software + optional HSM | Hybrid environments |

#### Workflow

```bash
# 1. Request code-signing certificate from enterprise CA
#    (process varies: AD CS web enrollment, EJBCA REST API, Vault PKI issue)

# 2. Configure Notation with PKCS#11 or KMS plugin
notation key add --default enterprise-key \
  --plugin notation-kms-plugin \
  --id "arn:aws:kms:eu-central-1:123456789:key/mrk-abc123"

# 3. Sign with HSM-backed key + enterprise TSA
notation sign ${REGISTRY}@${DIGEST} \
  --key enterprise-key \
  --signature-format cose \
  --timestamp-url https://tsa.enterprise.internal/rfc3161

# 4. Verify
notation verify ${REGISTRY}@${DIGEST}
```

#### Publisher signing material delivery

Enterprise configuration management (Ansible, Puppet, Chef, SCCM) distributes the enterprise root CA certificate and trust policy to all WFM Client instances during provisioning. Updates delivered via the same channel during maintenance windows.


### 4.3 Mode C: eIDAS QTSP (Regulated Production)

#### What it is

Code-signing certificates issued by a Qualified Trust Service Provider (QTSP) listed on the EU Trusted List. QTSPs are externally audited organizations authorized under the eIDAS Regulation (EU 910/2014, updated by EU 2024/1183 "eIDAS 2.0") to issue electronic certificates with legal recognition across all 27 EU member states. eIDAS 2.0 is cited here only for its continuity of QTSP supervision and the EU Trusted List mechanism — its marquee content (the EU Digital Identity Wallet, Qualified Electronic Attestations of Attributes) has no bearing on artifact signing and is out of scope for this SUP.

#### Two eSeal levels

| eSeal Level | eIDAS Article | HSM Required | Trust Level | Use Case |
|---|---|---|---|---|
| **Advanced Electronic Seal** | Art. 36 | No — software key permitted | High — uniquely linked to creator, detects subsequent data changes | ISVs, small vendors, CI/CD pipelines without HSM |
| **Qualified Electronic Seal** | Art. 38 | Yes — QSCD required | Highest — legal presumption of integrity and origin under EU law | Regulated industries, legal non-repudiation |

#### When to use

- EU-regulated industries requiring legal non-repudiation
- Organizations that do NOT want to operate their own CA infrastructure
- Multi-vendor ecosystems where publicly-auditable trust is needed
- Deployments requiring eIDAS compliance for contractual/legal reasons
- ISVs selling into EU markets where customers require third-party trust attestation

#### Why Advanced eSeals are particularly relevant for Margo

Advanced eSeals provide an attractive middle ground:

1. **No HSM required** — private keys may be stored in software keystores, cloud KMS, or CI/CD secret managers. This enables automated CI/CD signing pipelines without physical hardware.
2. **Third-party trust** — the signing certificate chains to a publicly-audited QTSP root CA, providing external trust validation that self-signed mode cannot offer.
3. **Built-in infrastructure** — the QTSP operates OCSP, CRL, and often RFC 3161 TSA as part of the service. No infrastructure to build or maintain.
4. **Regulatory compliance** — satisfies eIDAS requirements without the cost/complexity of Qualified eSeals.
5. **Transparent to verifiers** — at the verification layer, an Advanced eSeal signature is indistinguishable from a Qualified eSeal or enterprise PKI signature. WFM Clients just need the QTSP root CA in their trust store.

#### Workflow

```bash
# 1. Obtain eSeal certificate from QTSP
#    - Register with QTSP (identity verification required)
#    - Receive: leaf certificate + intermediate chain + QTSP root CA
#    - For Advanced: receive private key or generate locally
#    - For Qualified: key generated inside QSCD/HSM provided by QTSP

# 2. Configure Notation
#    For Advanced eSeal (software key):
notation key add --default qtsp-advanced-seal \
  --key private/qtsp-seal.key --cert certs/qtsp-seal-chain.crt

#    For Qualified eSeal (HSM/QSCD via PKCS#11):
notation key add --default qtsp-qualified-seal \
  --plugin notation-pkcs11-plugin \
  --id "pkcs11:token=QTSP-QSCD;object=margo-seal"

# 3. Sign with QTSP TSA
notation sign ${REGISTRY}@${DIGEST} \
  --key qtsp-advanced-seal \
  --signature-format cose \
  --timestamp-url https://tsa.qtsp-provider.eu/rfc3161

# 4. Verify (same as any other X.509 signature)
notation verify ${REGISTRY}@${DIGEST}
```

#### Finding a QTSP

Consult the [EU Trusted List browser](https://eidas.ec.europa.eu/efda/tl-browser/) for current QTSPs in your jurisdiction. Pricing and service terms vary by provider. Filter by service type "QCert for ESeal" to find providers offering electronic seal certificates.

#### Publisher signing material delivery

QTSP root CA certificates are publicly available on the EU Trusted Lists (TSL). Operators can download them from the QTSP website or from the EU TSL aggregator. For automated provisioning, include the QTSP root CA in the publisher signing material alongside the trust policy template.


### 4.4 Comparison matrix

| Dimension | Mode A: Self-Signed | Mode B: Corporate PKI | Mode C: eIDAS QTSP |
|---|---|---|---|
| **Trust anchor** | Developer-generated root CA | Enterprise-operated root CA | QTSP root CA (EU Trusted List) |
| **Key storage** | Local file / software keystore | HSM (FIPS 140-2 Level 3) | Software (Advanced) or QSCD (Qualified) |
| **Certificate issuance** | Self-issued (openssl) | Internal CA (AD CS, EJBCA, Vault) | External QTSP (identity-verified) |
| **Revocation** | None — manual trust store removal | OCSP + CRL (enterprise-operated) | OCSP + CRL (QTSP-operated) |
| **Timestamping** | None (signing time only) | RFC 3161 TSA (enterprise) | RFC 3161 TSA (QTSP) |
| **Cost** | Free | Infrastructure + staffing | Per-certificate annual fee (varies by provider) |
| **CA infrastructure to operate** | None | Full (Root CA, OCSP, CRL, TSA) | None — QTSP handles everything |
| **Production suitable** | **NO** | **YES** (recommended) | **YES** |
| **Standards & regulatory alignment** | None | IEC 62443, NIST (if properly operated) | eIDAS + IEC 62443 + NIST |
| **Legal non-repudiation** | No | Organizational-internal only | EU-wide (Qualified) or organizational (Advanced) |
| **Key compromise recovery** | Impossible (no revocation) | OCSP/CRL revocation within hours | OCSP/CRL revocation within hours |
| **10+ year verifiability** | No (no TSA) | Yes (with TSA) | Yes (with TSA) |
| **Multi-vendor trust** | Requires manual CA exchange | Requires CA certificate distribution | Publicly auditable (EU Trusted List) |


### 4.5 Deployment decision tree

```
┌─────────────────────────────────────────┐
│ Is this a PRODUCTION OT deployment?     │
└────────────────┬────────────────────────┘
                 │
         ┌───YES───┐───NO───┐
         │                   │
         ▼                   ▼
┌─────────────────┐  ┌──────────────────────┐
│ Mode A is       │  │ Mode A: Self-Signed  │
│ PROHIBITED.     │  │ Use for dev/lab/test │
│ Choose B or C.  │  └──────────────────────┘
└────────┬────────┘
         │
┌────────────────────────────────────────────┐
│ Do you operate (or want to operate)        │
│ your own CA infrastructure?                │
└────────────────┬───────────────────────────┘
                 │
         ┌───YES───┐───NO───┐
         │                   │
         ▼                   ▼
┌─────────────────┐  ┌───────────────────────────────────┐
│ Mode B:         │  │ Do you need EU legal recognition  │
│ Corporate PKI   │  │ or prefer managed PKI?            │
│ (recommended)   │  └───────────────┬───────────────────┘
└─────────────────┘                  │
                             ┌───YES───┐───NO───┐
                             │                   │
                             ▼                   ▼
                     ┌───────────────┐   ┌──────────────────┐
                     │ Mode C: QTSP  │   │ Mode B: Set up   │
                     │ (eIDAS)       │   │ Corporate PKI    │
                     └───────────────┘   │ (or use managed  │
                                         │ private CA like   │
                                         │ AWS Private CA)  │
                                         └──────────────────┘
```

**Additional decision factors:**

| If you need... | Choose... |
|---|---|
| Fastest time-to-production | Mode C (Advanced eSeal) — no infrastructure to build |
| Maximum organizational control | Mode B — you own the entire trust chain |
| Legal non-repudiation in EU | Mode C (Qualified eSeal) |
| Multi-vendor trust without bilateral CA exchange | Mode C — QTSP root CAs are publicly available |
| Air-gapped signing (no external dependencies) | Mode B — fully internal infrastructure |
| Minimal ongoing operational cost | Mode C — QTSP handles OCSP, CRL, TSA |

---

## 5. Publisher signing material examples

This section provides practical examples of the publisher signing material that Application Developers distribute to WFM operators and device administrators. The normative requirements for this material are in SUP-03 §5.

### 5.1 Directory structure

The publisher signing material is a directory (or archive) containing everything an operator needs to verify the publisher's signatures:

```
acme-corp-signing-material/
├── certificates/
│   ├── root-ca.crt              # PEM-encoded root CA certificate (trust anchor)
│   ├── intermediate-ca.crt      # PEM-encoded intermediate CA(s) if not embedded in signature
│   └── tsa-root-ca.crt          # PEM-encoded TSA root CA (if TSA is used)
├── trustpolicy-template.json    # Template trustpolicy.json with placeholder scopes
└── README.md                    # Human-readable installation instructions
```

**File contents:**

| File | Format | Purpose |
|------|--------|---------|
| `certificates/root-ca.crt` | PEM (X.509 v3) | Trust anchor — the root CA that issued the signing certificate chain |
| `certificates/intermediate-ca.crt` | PEM (X.509 v3) | Intermediate CA(s) for environments that don't extract from envelope |
| `certificates/tsa-root-ca.crt` | PEM (X.509 v3) | TSA trust anchor for RFC 3161 timestamp verification |
| `trustpolicy-template.json` | JSON | Pre-configured policy with vendor identity; `registryScopes` as placeholder |
| `README.md` | Markdown | Installation guide for importing into WFM Client trust stores |

### 5.2 Trust policy template example

The trust policy template contains the vendor's `trustedIdentities` (Subject DN) and `trustStores` references pre-filled. The `registryScopes` field uses a placeholder value that operators replace with their actual registry paths:

```json
{
  "version": "1.0",
  "trustPolicies": [
    {
      "name": "acme-corp-production",
      "registryScopes": [
        "<REPLACE_WITH_YOUR_REGISTRY>/margo-apps/*"
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
        "ca:acme-corp-root-ca",
        "tsa:acme-corp-tsa"
      ],
      "trustedIdentities": [
        "x509.subject: CN=ACME Corp Release Signing, O=ACME Corporation, C=DE"
      ]
    }
  ]
}
```

**Operator installation steps** (to include in README.md):

1. Copy `certificates/root-ca.crt` to `{NOTATION_CONFIG}/truststore/x509/ca/acme-corp-root-ca/root-ca.crt`
2. Copy `certificates/tsa-root-ca.crt` to `{NOTATION_CONFIG}/truststore/x509/tsa/acme-corp-tsa/tsa-root-ca.crt`
3. Edit `trustpolicy-template.json`: replace `<REPLACE_WITH_YOUR_REGISTRY>` with your mirror registry hostname and path
4. Merge the policy entry into your existing `{NOTATION_CONFIG}/trustpolicy.json`

Where `{NOTATION_CONFIG}` defaults to `~/.config/notation/` on Linux.

### 5.3 Discovery via well-known OCI repository (informative)

As a convenience mechanism, Application Developers may publish their signing material as an OCI artifact in a well-known repository path:

```
<registry>/<namespace>/_trust-bundle
```

| Application Package location | Signing material location |
|---|---|
| `registry.vendor.com/margo-apps/my-app:v1.0.0` | `registry.vendor.com/margo-apps/_trust-bundle:latest` |
| `harbor.enterprise.internal/acme-apps/gateway:v2.1.0` | `harbor.enterprise.internal/acme-apps/_trust-bundle:latest` |

The signing material OCI artifact uses:
- `artifactType`: `application/vnd.margo.trust-bundle.v1+tar`
- Layer: gzip-compressed tar of the directory structure above

WFM implementations may auto-discover signing material by querying the `_trust-bundle` repository in the same namespace.

**Important notes:**

- This is a **convenience mechanism**, not a normative requirement. Operators are not required to use it and may provision trust materials through any channel (USB, configuration management, internal wiki, etc.).
- The underscore prefix in `_trust-bundle` minimizes collision with application repository names but may conflict with strict OCI Distribution Spec name grammar (`[a-z0-9]+([._-][a-z0-9]+)*`). Some registries may reject the leading underscore. Implementations should fall back to alternative provisioning mechanisms if the repository cannot be created.
- Signing material should be tagged with semantic versions (`:v1.0.0`, `:v2.0.0`) when the root CA rotates. The `:latest` tag should always point to the current active version.

---

## 6. Cryptographic considerations

This section presents algorithm and format guidance. Your organization's security policy governs — these are starting points, not mandates. SUP-03 §3 presents an informative, **RECOMMENDED** baseline; it is not a normative minimum.

### 6.1 Recommended algorithms by deployment context

| Context | Signing Algorithm | Key Size | Hash | Rationale |
|---------|------------------|----------|------|-----------|
| General production | ECDSA P-256 (ES256) | 256-bit | SHA-256 | Good balance of security, performance, and compatibility |
| High-security / long-lived | ECDSA P-384 (ES384) | 384-bit | SHA-384 | Higher security margin for 10+ year asset lifecycles |
| Legacy interop required | RSASSA-PSS (PS256) | 3072-bit | SHA-256 | When ECDSA not supported by existing HSM/tooling |
| Constrained devices | ECDSA P-256 (ES256) | 256-bit | SHA-256 | Smallest signatures, fastest verification |
| Post-quantum preparedness | ECDSA P-384 (ES384) | 384-bit | SHA-384 | Larger security margin against future quantum threats |

### 6.2 MIAF alignment note

The Margo Interoperability and Alignment Framework (MIAF) references these algorithms for its own signing purposes:

- RSA 3072-bit (minimum)
- ECDSA P-256 and P-384
- Ed25519

SUP-03's §3 informative RECOMMENDED baseline (RSA 3072-bit minimum, ECDSA P-256 or P-384) already matches MIAF's own minimums — there is no gap to bridge. Organizations aligning with both Margo and MIAF can use either recommendation directly to satisfy both frameworks simultaneously.

**Note:** MIAF uses the term "trust bundle" for its own distribution mechanism. To avoid confusion, this guide uses "publisher signing material" for the Margo artifact signing trust materials described in §5.

### 6.3 COSE vs JWS envelope format

SUP-03 requires verifiers to support both formats as the general-production default (§3). §3 also defines a constrained-device exception: a WFM Client **MAY** declare a `signatureEnvelopes: cose-only` capability, in which case the operator's trust policy restricts that device's `registryScopes` to publishers who commit to COSE-only signing. An undeclared device is still held to the full dual-format requirement — the exception applies only where explicitly declared. For publishers choosing which to produce:

| Factor | COSE_Sign1 (`application/cose`) | JWS (`application/jose+json`) |
|--------|----------------------------------|-------------------------------|
| Wire size | Compact (binary CBOR) | Larger (Base64-encoded JSON) |
| Parse complexity | Low — direct binary access | Higher — Base64 decode + JSON parse |
| Memory allocation | Minimal — no string expansion | ~33% overhead from Base64 encoding |
| Human readability | No (binary) | Yes (JSON text) |
| Debugging ease | Requires CBOR diagnostic tools | Standard JSON tools work |
| Edge suitability | Excellent (<128MB RAM devices) | Acceptable (>256MB RAM) |
| Library availability | `t_cose` (C), `cose-rust` (Rust), `go-cose` (Go) | Widely available in all languages |

**Guidance:**

- For constrained WFM Clients with <128MB RAM, the device SHOULD declare the `signatureEnvelopes: cose-only` capability (SUP-03 §3), and publishers targeting that `registryScopes` path should produce COSE_Sign1 envelopes exclusively. Without the declaration, the device remains obligated to parse both formats regardless of its RAM budget.
- For general production, COSE_Sign1 is recommended (more compact, faster to verify) even where the dual-format MUST applies.
- JWS is acceptable when human-readability or existing JSON tooling integration is a priority.
- Publishers may produce both formats (two signatures on the same artifact) to support mixed environments during migration.

### 6.4 Timestamping guidance for long-lived OT assets

OT devices commonly operate for 10–25 years. Code-signing certificates typically have 2–3 year validity. Without an RFC 3161 timestamp, signatures become unverifiable after the signing certificate expires.

**When to include timestamps:**

| Scenario | Timestamp needed? | Rationale |
|----------|-------------------|-----------|
| Development/lab (Mode A) | No | Short-lived environments, no production risk |
| Production, cert valid >5 years | Recommended | Provides safety margin |
| Production, cert valid 1–3 years | Strongly recommended | Assets will likely outlive certificates |
| Regulated production (IEC 62443) | Yes | Auditors expect long-term verifiability |
| Assets with 10+ year lifecycle | Yes | Essential for continued verification |

**TSA selection criteria:**

- TSA must issue RFC 3161-compliant timestamp tokens
- TSA certificate should have a longer validity than signing certificates (10+ years typical for TSA roots)
- For air-gapped verification, the TSA root CA must be pre-provisioned in `tsa:<name>` trust stores
- Enterprise TSA (Mode B) gives full control; QTSP TSA (Mode C) avoids infrastructure overhead

---

## 7. OT environment operational guidance

### 7.1 Air-gapped networks

Industrial OT environments frequently operate without persistent internet connectivity. The following practices ensure signature verification works in disconnected environments:

**CRL pre-provisioning:**

- Download CRLs from CA/QTSP distribution points during device commissioning
- Store CRLs in a well-known local directory (e.g., `/etc/notation/truststore/x509/crl/`)
- Configure a maximum CRL staleness threshold appropriate to your threat model (e.g., 30 days for high-security, 90 days for moderate)
- Refresh CRLs during planned maintenance windows or brief connectivity periods

**Publisher signing material updates:**

- Deliver signing material updates (new root CAs, policy changes) via USB maintenance keys during scheduled downtime
- Use WFM-mediated distribution during brief connectivity windows (if available)
- Configuration management tools (Ansible, Puppet) over local network segments
- Document and track all trust material changes in your change management system

**OCSP stapling alternative:**

If the signing toolchain supports OCSP stapling (embedding a time-bounded OCSP response in the signature envelope), this can provide revocation evidence without runtime network access. Not all Notation plugins support this — verify with your implementation before relying on it.

### 7.2 Long-lived OT assets (10+ year lifecycle)

OT devices commonly operate for 10–25 years. Code-signing certificates typically have 2–3 year validity periods. Without timestamping, all signatures become unverifiable after certificate expiry.

**Why TSA is critical — timeline illustration:**

```
Year 0   ──── Developer signs artifact
               Certificate valid: 2026-01-01 to 2029-01-01
               TSA timestamp: 2026-03-15T10:30:00Z

Year 3   ──── Certificate expires (2029-01-01)

Year 5   ──── WFM Client needs to verify artifact for deployment
               │
               ├── WITHOUT TSA: ✗ FAIL
               │   Certificate expired. No proof signing occurred
               │   while certificate was valid. Verification rejects.
               │
               └── WITH TSA: ✓ PASS
                   TSA token proves signature existed on 2026-03-15,
                   which is before certificate expiry (2029-01-01).
                   Verification succeeds.

Year 10  ──── Device still operational, artifact still deployed
               TSA timestamp continues to prove validity.

Year 15  ──── Device decommissioned
               Entire lifecycle covered by original TSA timestamp.
```

**Planning for certificate rotation:**

1. Sign new artifact versions with renewed certificates as part of regular release cycles
2. Maintain previous root CAs in trust stores until all artifacts signed under them are decommissioned
3. When root CA rotates, publish a new signing material version (`:v2.0.0`) and communicate to operators
4. Operators add the new root CA to trust stores — old and new coexist during transition

**TSA certificate lifecycle:**

- TSA certificates also expire. Ensure TSA root CAs are in `tsa:<name>` trust stores.
- TSA timestamps are self-contained — they remain valid as long as the TSA root CA is trusted.
- Rotate TSA root CAs with the same caution as signing CAs.

### 7.3 Constrained WFM Clients

WFM Clients in OT environments may have limited CPU, memory, and storage. The signing framework is designed to accommodate these constraints:

**Format choice:**

- COSE_Sign1 (binary CBOR) is significantly more compact than JWS (JSON text)
- COSE parsing requires less memory allocation — no Base64 decode step
- For devices with <128MB RAM, the device SHOULD declare the `signatureEnvelopes: cose-only` capability (SUP-03 §3) rather than silently dropping JWS support; publishers targeting that device's declared `registryScopes` then produce COSE-only. Without the declaration, dropping JWS parsing support is non-conformant — SUP-03's dual-format MUST still applies.

**Implementation options:**

The verification algorithm is specified abstractly — implementations in C, Rust, or other native languages are possible without a Go runtime or full Notation CLI.

| Approach | Language | Memory footprint | Notes |
|----------|----------|------------------|-------|
| Notation CLI | Go | ~50MB | Full feature set, plugin ecosystem |
| `notation-go` library | Go | ~30MB | Embeddable, no CLI overhead |
| Custom minimal verifier | C/Rust | ~2–5MB | COSE + ECDSA P-256 + X.509 chain only |
| `t_cose` + `mbedtls` | C | ~1–3MB | Smallest footprint, constrained devices |

**Minimum viable verification implementation:**

- COSE_Sign1 parsing (RFC 9052)
- ECDSA P-256 signature verification
- X.509 certificate chain building and path validation
- Trust store lookup (filesystem directory of PEM certificates)
- Trust policy matching (JSON parse of `trustpolicy.json`)

**Storage considerations:**

- Trust stores (root CA certificates): typically <10KB total
- CRL cache: 10–100KB per CA (varies by revocation volume)
- Trust policy: <2KB (JSON text)
- Total signing infrastructure overhead: <200KB on-device

### 7.4 Hardware trust anchors

For high-security deployments, protect the integrity of the trust store itself:

**TPM 2.0 (recommended where available):**

- Seal trust store decryption keys to TPM PCR values
- Ensures trust store integrity — if device firmware is tampered, PCR values change, trust store becomes inaccessible
- Use TPM for measured boot chain validation before signature verification begins
- Platform Configuration Registers (PCRs) extend through boot stages: BIOS → bootloader → kernel → trust store

**Alternative tamper-detection (when TPM unavailable):**

| Mechanism | Protection | Overhead | Availability |
|-----------|-----------|----------|-------------|
| dm-verity | Read-only verified filesystem for trust store partition | Low (kernel-level) | Linux 3.4+ |
| IMA (Integrity Measurement Architecture) | Kernel-level file integrity monitoring with audit log | Medium | Linux 2.6.30+ |
| Filesystem permissions | root-owned, mode 0444, immutable flag (`chattr +i`) | Minimal | Any Linux |
| UEFI Secure Boot | Full boot chain validation: firmware → kernel → rootfs | Low (firmware-level) | UEFI systems |

**Defense-in-depth approach:**

Combine multiple layers: Secure Boot validates the kernel, dm-verity protects the trust store partition, and the verification engine uses those protected trust stores to validate artifact signatures. Each layer independently prevents a different class of attack.

---

## 8. Revocation in OT environments

Certificate revocation — the ability to invalidate a compromised signing key — works differently in OT environments than in typical web PKI. This section provides operational guidance.

### 8.1 Context: MIAF vs code-signing PKI

MIAF does not use CRL/OCSP for its own trust bundle distribution mechanism. The revocation discussion here applies to a **different context**: the X.509 code-signing PKI used for OCI artifact signatures. These are standard PKI certificates issued by enterprise CAs or QTSPs, and they use standard revocation mechanisms.

### 8.2 Revocation mechanisms

| Mechanism | How it works | Network required | Latency | OT suitability |
|-----------|-------------|-----------------|---------|----------------|
| **OCSP** | Real-time query to CA responder | Yes (per-verification) | Seconds | Poor in air-gapped environments |
| **CRL** | Periodic download of revocation list | Yes (periodic) | Hours to days | Good with pre-provisioning |
| **OCSP Stapling** | Embed time-bounded OCSP response in signature | No (at verification time) | Pre-fetched | Best for air-gapped |

### 8.3 Air-gapped revocation strategies

For environments without network connectivity at verification time:

**Option A: Pre-provisioned CRLs**

- Download CRLs during maintenance windows
- Store locally on each WFM Client
- Accept staleness risk (CRL may be hours/days old)
- Configure acceptable staleness via trust policy `override.revocation`

**Option B: Revocation = log**

- Set `"override": { "revocation": "log" }` in trust policy
- Verification proceeds even when revocation status is unknown
- Revocation failures are logged for later audit review
- Appropriate when availability is prioritized over immediate revocation enforcement

**Option C: Revocation = skip (not recommended)**

- Set `"override": { "revocation": "skip" }` in trust policy
- No revocation checking at all
- Only appropriate for isolated lab environments

### 8.4 Operator decision framework

The operator decides the enforcement level via trust policy. The tradeoff is between security (blocking compromised keys immediately) and availability (allowing deployment when revocation status is unknown):

| Priority | Configuration | Behavior |
|----------|--------------|----------|
| Security first | `"revocation": "enforce"` + fresh CRLs | Rejects artifacts signed with revoked keys; fails if CRL is stale beyond threshold |
| Balanced | `"revocation": "enforce"` + generous staleness | Rejects known-revoked; proceeds if CRL is stale but within threshold |
| Availability first | `"revocation": "log"` | Always proceeds; logs revocation failures for audit |

### 8.5 Revocation response timeline

When a signing key is compromised:

1. **Hour 0:** Compromise detected
2. **Hour 1–4:** CA revokes certificate (OCSP updated, new CRL issued)
3. **Next maintenance window:** Air-gapped devices receive updated CRL
4. **Gap period:** Between compromise and CRL refresh, air-gapped devices cannot detect revocation

To minimize the gap period:
- Keep CRL refresh intervals short (daily if connectivity allows)
- Use OCSP stapling in signatures when the signing toolchain supports it
- Consider emergency maintenance windows for critical compromises

---

## 9. Regulatory alignment (informative)

This section maps SUP-03 capabilities to common industrial cybersecurity standards. This mapping is informative — formal compliance assessments require engagement with qualified auditors.

### 9.1 IEC 62443-4-2 CR 3.4 — Software and Information Integrity

**Requirement (paraphrased — the exact clause text is behind the IEC paywall and should be confirmed against a licensed copy):** the capability to verify the integrity of software before installation, and automated notification when an integrity violation is detected.

**How SUP-03 satisfies this:**

- Content-addressable SHA-256 digests provide tamper-evident integrity for all OCI layers
- X.509 signature verification at the WFM Client (the "component") before any archive extraction or workload deployment
- The 10-step verification engine provides a complete, auditable verification procedure
- Verification logs provide evidence for compliance audits

### 9.2 NIST SP 800-218 (SSDF) — Secure Software Development Framework

**Requirement:** PO.1.3 — "Communicate requirements to all third parties who will provide commercial software components to the organization for reuse by the organization's own software," with the implementation example "Require third parties to provide provenance data and integrity verification mechanisms for all components of their software." [Triangulated from secondary sources — the NIST PDF could not be parsed directly; confirm wording against the primary document.]

**How SUP-03 satisfies this:**

- Notation signatures bound to manifest digests prove provenance (who signed) and integrity (what was signed)
- Trust policy identity matching (`trustedIdentities`) restricts deployment to known, authorized publishers
- Publisher signing material provides the provenance chain from signing identity back to a trust anchor
- Pre-publish validation (SUP-03 §2.2) ensures developers verify their own signatures before distribution

### 9.3 ISO/IEC 27001:2022 A.8.19 (supersedes 2013's A.12.5.1 / A.12.6.2) — Installation of Software on Operational Systems

**Requirement (paraphrased — the exact 2022 clause text is behind the ISO paywall and should be confirmed against a licensed copy):** rules governing the installation of software on operational systems should be established and implemented.

**How SUP-03 satisfies this:**

- Subject DN matching in `trustedIdentities` restricts which signing identities are authorized per registry scope
- Only artifacts signed by authorized identities pass verification
- Policy is declarative and auditable — `trustpolicy.json` documents exactly who is trusted for what
- Verification levels (`strict`, `permissive`, `audit`) provide graduated enforcement

### 9.4 eIDAS Electronic Seals

For organizations operating in the European Union, eIDAS electronic seals (see §4.3) provide:

- **Legal recognition** across all 27 EU member states without bilateral agreements
- **Non-repudiation** — Qualified eSeals carry legal presumption of integrity (Art. 35(2))
- **External auditability** — QTSP operations are subject to conformity assessment by EU supervisory bodies
- **Publicly verifiable trust** — QTSP root CAs are published on the EU Trusted Lists

Consult the [EU Trusted List browser](https://eidas.ec.europa.eu/efda/tl-browser/) to find QTSPs offering electronic seal certificates in your jurisdiction.

### 9.5 ETSI EN 319 102-1 — Relationship

The SUP-03 verification engine is informed by ETSI EN 319 102-1 (Procedures for Creation and Validation of AdES Digital Signatures). Organizations seeking formal ETSI compliance should reference ETSI EN 319 102-1 V1.4.1+ directly.

Certificates, timestamps, and key storage in this SUP's PKI can independently conform to ETSI EN 319 411-2, EN 319 422, and EN 419 221-5 respectively. This does not extend to EN 319 102-1 conformance for the overall verification engine.

**The AdES signature-format family is not closed at CAdES/XAdES/PAdES/JAdES.** ETSI TS 119 152-1 V1.1.1 (2026-03) defines CB-AdES ("CBOR-AdES"), a fifth member, built directly on CBOR Object Signing and Encryption (COSE, RFC 9052) — the same base RFC this SUP's COSE_Sign1 envelope uses. EN 319 102-1's own scope clause (§1, V1.4.1, 2024-06) names only three formats it governs — EN 319 122-1 (CAdES), EN 319 132-1 (XAdES), EN 319 142-1 (PAdES) — and contains no reference to JAdES or CB-AdES. Both TS 119 182-1 (JAdES) and TS 119 152-1 (CB-AdES) state that validation procedures for their own format are outside their own document's scope and point toward EN 319 102-1 for "other types," but EN 319 102-1's own scope clause does not, as published, incorporate either.

This SUP's envelopes are not AdES signatures of any current type — not because the family's base encodings exclude COSE or JWS (COSE now has a legitimate member in CB-AdES, and JAdES is itself JWS-based), but because neither the Notary Project COSE_Sign1 envelope nor its JWS envelope populates the qualifying header parameters an AdES format actually mandates. Concretely: CB-AdES's baseline B-B level mandates exactly one signed header parameter beyond `alg` — a CWT Claims header (RFC 9597) carrying `iat`, the claimed signing time, cardinality exactly 1 at every baseline level. The Notary Project COSE envelope instead carries `io.cncf.notary.signingTime`, a Notary-namespaced protected header with different semantics (explicitly untrusted under the `notary.x509` scheme) and a different wire structure. At the B-T level, CB-AdES additionally mandates a `sigTst` CBOR map wrapping an RFC 3161 timestamp token inside the unsigned header parameter; the Notary envelope carries an analogous RFC 3161 timestamp, but as its own unprotected header, not CB-AdES's defined container. The same populates-vs-doesn't-populate logic applies to JAdES over this SUP's JWS envelope option.

A future revision of this SUP could additionally profile the CB-AdES header parameters onto its COSE_Sign1 envelope to become a genuine, ETSI-numbered AdES format — that is a distinct, additive profiling exercise this revision does not undertake. Whether the WG wants to pursue it is not addressed here.

Organizations requiring formal AdES conformance today should evaluate a dedicated CAdES-, XAdES-, PAdES-, JAdES-, or CB-AdES-producing toolchain separately from this SUP's Notary Project-based mechanism.

Formal ETSI compliance is not required by SUP-03.

---

## 10. Migration paths

### 10.1 Self-Signed (Mode A) → Corporate PKI (Mode B)

| Step | Action | Impact on operators |
|------|--------|-------------------|
| 1 | Deploy enterprise CA hierarchy (root CA offline/HSM + intermediate CA online + OCSP + TSA) | None yet |
| 2 | Issue new code-signing certificates from enterprise CA | None yet |
| 3 | Re-sign current artifacts with new enterprise certificates (old signatures remain — multiple signatures supported) | None yet |
| 4 | Publish new publisher signing material containing enterprise root CA + updated trust policy template | Operators receive new material |
| 5 | Operators add enterprise root CA to trust stores; update `trustedIdentities` in trust policy | Brief configuration change |
| 6 | After transition period: operators remove self-signed root CA from trust stores | Old signatures no longer verify |

**Key point:** During the transition period, both old (self-signed) and new (enterprise) signatures coexist. Operators can verify against either until they remove the old root CA.

### 10.2 Self-Signed (Mode A) → QTSP (Mode C)

| Step | Action | Impact on operators |
|------|--------|-------------------|
| 1 | Obtain eSeal certificate from chosen QTSP | None |
| 2 | Re-sign current artifacts with QTSP-issued certificate (+ QTSP TSA) | None |
| 3 | Publish new publisher signing material: replace self-signed root CA with QTSP root CA | Operators receive new material |
| 4 | Operators replace self-signed root CA in trust stores with QTSP root CA; update `trustedIdentities` to match new Subject DN | Brief configuration change |

### 10.3 Permissive → Strict (policy-only transition)

This requires **no artifact re-signing** — it is a policy-only transition on the operator side:

| Phase | Trust policy level | Behavior |
|-------|-------------------|----------|
| 1. Discovery | `"level": "audit"` | All checks run, nothing blocked, violations logged |
| 2. Assessment | Review audit logs | Identify artifacts/signatures that would fail under strict |
| 3. Remediation | Fix issues (missing TSA, expired certs, wrong identity) | Re-sign problematic artifacts if needed |
| 4. Intermediate | `"level": "permissive"` | Crypto failures block; expiry/revocation warn only |
| 5. Validation | Review remaining warnings | Confirm all artifacts pass cleanly |
| 6. Enforcement | `"level": "strict"` | Full enforcement — reject any verification failure |

This phased approach is safe because verification level changes never require re-signing artifacts. The artifacts and their signatures are unchanged — only the local enforcement policy changes.

---

## 11. References

### Normative specification

- [SUP-03: OCI Cryptographic Signature Architecture and Trust Verification Framework](sup-03-oci-signature.md) — the normative companion to this guide

### Standards and regulations

- [eIDAS Regulation (EU) No 910/2014](https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=uriserv:OJ.L_.2014.257.01.0073.01.ENG) — Electronic identification and trust services
- [eIDAS 2.0 (EU) 2024/1183](https://eur-lex.europa.eu/eli/reg/2024/1183/oj) — Amendment establishing European Digital Identity Framework
- [EU Trusted List Browser](https://eidas.ec.europa.eu/efda/tl-browser/) — Find QTSPs by country and service type
- [ETSI EN 319 102-1](https://www.etsi.org/deliver/etsi_en/319100_319199/31910201/) — Procedures for Creation and Validation of AdES Digital Signatures
- [ETSI TS 119 182-1](https://www.etsi.org/deliver/etsi_ts/119100_119199/11918201/) — JAdES Digital Signatures
- [ETSI TS 119 152-1](https://www.etsi.org/deliver/etsi_ts/119100_119199/11915201/) — CB-AdES Digital Signatures (CBOR Object Signing and Encryption profile)
- [IEC 62443-4-2](https://webstore.iec.ch/publication/34421) — Security for Industrial Automation and Control Systems: Technical security requirements for IACS components
- [NIST SP 800-218](https://csrc.nist.gov/publications/detail/sp/800-218/final) — Secure Software Development Framework (SSDF)
- [ISO/IEC 27001](https://www.iso.org/standard/27001) — Information security management systems

### Tooling and specifications

- [CNCF Notary Project — Notation CLI](https://github.com/notaryproject/notation) — Reference signing/verification implementation
- [CNCF Notary Project — Signing and Verification Workflow](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/signing-and-verification-workflow.md) — Normative verification algorithm
- [CNCF Notary Project — Trust Store and Trust Policy Specification](https://github.com/notaryproject/specifications/blob/v1.1.0/specs/trust-store-trust-policy.md)
- [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md)
- [RFC 9052 — COSE (CBOR Object Signing and Encryption)](https://www.rfc-editor.org/rfc/rfc9052)
- [RFC 7515 — JSON Web Signature (JWS)](https://www.rfc-editor.org/rfc/rfc7515)
- [RFC 3161 — Time-Stamp Protocol (TSP)](https://www.rfc-editor.org/rfc/rfc3161)
- [RFC 6960 — Online Certificate Status Protocol (OCSP)](https://www.rfc-editor.org/rfc/rfc6960)

### Libraries for constrained implementations

- [`t_cose`](https://github.com/laurencelundblade/t_cose) — C library for COSE signing and verification
- [`cose-rust`](https://github.com/nickel-env/cose-rust) — Rust COSE implementation
- [`go-cose`](https://github.com/veraison/go-cose) — Go COSE implementation
- [`mbedtls`](https://github.com/Mbed-TLS/mbedtls) — Lightweight C crypto library (X.509, ECDSA)

---

*This document is informative. All normative requirements are defined in [SUP-03](sup-03-oci-signature.md).*

*Prepared by Andrii Melashchenko (Belden Inc.), 2026-07-29. Subject to the Open Web Foundation Contributor License Agreement governing the Margo specification.*
