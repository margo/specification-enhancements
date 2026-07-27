# SUP-03 Implementation Guide: OCI Cryptographic Signature — PKI Deployment Models and Operational Guidance

**Status:** Informative (non-normative companion to [SUP-03](sup-03-oci-signature.md))  
**Author:** Margo Security Working Group  
**Date:** 2026-07-27

---

## 1. Purpose and scope

This document is the informative companion to the normative SUP-03 specification ("OCI Cryptographic Signature Architecture and Trust Verification Framework"). It provides implementation guidance, PKI deployment advice, regulatory context, and operational recommendations for organizations adopting Margo artifact signing.

**This document does NOT define requirements.** All normative MUST/SHOULD/MAY statements are in the [normative SUP-03 specification](sup-03-oci-signature.md). This guide references those requirements and explains how to satisfy them in practice across different deployment scenarios.

**Intended audience:** DevSecOps engineers, security architects, OT operators, compliance officers, and ISVs implementing Margo artifact signing.

---

## 2. PKI deployment models

SUP-03 supports three PKI deployment models. The normative specification defines the cryptographic and structural requirements that all three models satisfy identically at the verification layer. This section provides practical guidance for selecting and implementing the appropriate model.

### 2.1 Mode A: Self-Signed PKI (Development and Laboratory)

#### What it is

A locally-generated X.509 certificate hierarchy created by the application developer using standard tools (OpenSSL, cfssl, step-ca). The developer acts as their own Certificate Authority.

#### When to use

- Laboratory and prototyping environments
- CI/CD pipeline testing (pre-production)
- Air-gapped testbeds with no external connectivity
- Learning and evaluating the Margo signing workflow
- Internal demos and proof-of-concept deployments

#### When NOT to use

- Production OT environments (normative: self-signed mode MUST NOT be used in production)
- Any deployment subject to IEC 62443, NIST SP 800-218, or eIDAS compliance
- Multi-vendor environments where trust must be established between organizations
- Anywhere key compromise recovery is required

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

#### Trust bundle delivery

Manual distribution: provide `rootCA.crt` + `trustpolicy-template.json` to the verifying party via secure channel (USB key, encrypted email, internal wiki).


### 2.2 Mode B: Corporate/Enterprise PKI (Production)

#### What it is

An organization-operated Certificate Authority infrastructure that issues code-signing certificates to authorized developers and maintains revocation/timestamping services. Common implementations: Microsoft Active Directory Certificate Services (AD CS), EJBCA, HashiCorp Vault PKI Secrets Engine, AWS Private CA, Google Cloud CAS.

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
│  [OCSP Responder]  ←── queries from WFM/edge devices        │
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

#### Trust bundle delivery

Enterprise configuration management (Ansible, Puppet, Chef, SCCM) distributes the enterprise root CA certificate and trust policy to all WFM instances and edge devices during provisioning. Updates delivered via the same channel during maintenance windows.


### 2.3 Mode C: EU Qualified Trust Service Provider — eIDAS (Regulated Production)

#### What it is

Code-signing certificates issued by a Qualified Trust Service Provider (QTSP) listed on the EU Trusted List. QTSPs are externally audited organizations authorized under the eIDAS Regulation (EU 910/2014, updated by EU 2024/1183 "eIDAS 2.0") to issue electronic certificates with legal recognition across all 27 EU member states.

#### Two eSeal levels

| eSeal Level | eIDAS Article | HSM Required | Trust Level | Indicative Cost | Use Case |
|---|---|---|---|---|---|
| **Advanced Electronic Seal** | Art. 36 | **NO** — software key permitted | High — uniquely linked to creator, detects subsequent data changes | €50–200/year | ISVs, small vendors, CI/CD pipelines without HSM |
| **Qualified Electronic Seal** | Art. 38 | **YES** — QSCD required | Highest — legal presumption of integrity and origin under EU law | €200–800/year | Regulated industries, legal non-repudiation |

#### When to use

- EU-regulated industries requiring legal non-repudiation
- Organizations that do NOT want to operate their own CA infrastructure
- Multi-vendor ecosystems where publicly-auditable trust is needed
- Deployments requiring eIDAS compliance for contractual/legal reasons
- ISVs selling into EU markets where customers require third-party trust attestation

#### Why Advanced eSeals are particularly relevant for Margo

Advanced eSeals provide an attractive middle ground:

1. **No HSM required** — private keys MAY be stored in software keystores, cloud KMS, or CI/CD secret managers. This enables automated CI/CD signing pipelines without physical hardware.
2. **Third-party trust** — the signing certificate chains to a publicly-audited QTSP root CA, providing external trust validation that self-signed mode cannot offer.
3. **Built-in infrastructure** — the QTSP operates OCSP, CRL, and often RFC 3161 TSA as part of the service. No infrastructure to build or maintain.
4. **Regulatory compliance** — satisfies eIDAS requirements without the cost/complexity of Qualified eSeals.
5. **Transparent to verifiers** — at the verification layer, an Advanced eSeal signature is indistinguishable from a Qualified eSeal or enterprise PKI signature. WFM/devices just need the QTSP root CA in their trust store.

#### Workflow

```bash
# 1. Obtain eSeal certificate from QTSP
#    - Register with QTSP (e.g., DigiCert, Sectigo, D-TRUST, QuoVadis)
#    - Complete identity verification (organization validation)
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

#### Trust bundle delivery

QTSP root CA certificates are publicly available on the EU Trusted Lists (TSL). Operators can download them from the QTSP website or from the EU TSL aggregator. For automated provisioning, include the QTSP root CA in the vendor trust bundle alongside the trust policy template.

#### Example QTSPs with code-signing/eSeal services

| Provider | Headquarters | eSeal Types | TSA Included |
|---|---|---|---|
| D-TRUST (Bundesdruckerei) | Germany | Advanced, Qualified | Yes |
| DigiCert | USA/EU | Advanced | Yes |
| QuoVadis (DigiCert) | Switzerland | Advanced, Qualified | Yes |
| Sectigo | UK/EU | Advanced | Yes |
| InfoCert | Italy | Advanced, Qualified | Yes |
| Certum (Asseco) | Poland | Advanced, Qualified | Yes |

> **Note:** This list is illustrative, not exhaustive. Check the [EU Trusted List browser](https://eidas.ec.europa.eu/efda/tl-browser/) for the current list of QTSPs in your jurisdiction.


### 2.4 Comparison matrix

| Dimension | Mode A: Self-Signed | Mode B: Corporate PKI | Mode C: eIDAS QTSP |
|---|---|---|---|
| **Trust anchor** | Developer-generated root CA | Enterprise-operated root CA | QTSP root CA (EU Trusted List) |
| **Key storage** | Local file / software keystore | HSM (FIPS 140-2 Level 3) | Software (Advanced) or QSCD (Qualified) |
| **Certificate issuance** | Self-issued (openssl) | Internal CA (AD CS, EJBCA, Vault) | External QTSP (identity-verified) |
| **Revocation** | None — manual trust store removal | OCSP + CRL (enterprise-operated) | OCSP + CRL (QTSP-operated) |
| **Timestamping** | None (signing time only) | RFC 3161 TSA (enterprise) | RFC 3161 TSA (QTSP) |
| **Cost** | Free | Infrastructure + staffing | €50–800/year per certificate |
| **CA infrastructure to operate** | None | Full (Root CA, OCSP, CRL, TSA) | None — QTSP handles everything |
| **Production suitable** | **NO** | **YES** (recommended) | **YES** |
| **Regulatory compliance** | None | IEC 62443, NIST (if properly operated) | eIDAS + IEC 62443 + NIST |
| **Legal non-repudiation** | No | Organizational-internal only | EU-wide (Qualified) or organizational (Advanced) |
| **Key compromise recovery** | Impossible (no revocation) | OCSP/CRL revocation within hours | OCSP/CRL revocation within hours |
| **10+ year verifiability** | No (no TSA) | Yes (with TSA) | Yes (with TSA) |
| **Multi-vendor trust** | Requires manual CA exchange | Requires CA certificate distribution | Publicly auditable (EU Trusted List) |

---


## 3. OT environment operational guidance

### 3.1 Air-gapped networks

Industrial OT environments frequently operate without persistent internet connectivity. The following practices ensure signature verification works in disconnected environments:

**CRL pre-provisioning:**
- Download CRLs from CA/QTSP distribution points during device commissioning
- Store CRLs in a well-known local directory (e.g., `/etc/notation/truststore/x509/crl/`)
- Configure a maximum CRL staleness threshold appropriate to your threat model (e.g., 30 days)
- Refresh CRLs during planned maintenance windows

**Trust bundle updates:**
- Deliver trust bundle updates (new root CAs, policy changes) via USB maintenance keys during scheduled downtime
- Use WFM-mediated distribution during brief connectivity windows (if available)
- Configuration management tools (Ansible, Puppet) over local network segments

**OCSP stapling alternative:**
- If the signing toolchain supports OCSP stapling (embedding a time-bounded OCSP response in the signature envelope), this can provide revocation evidence without runtime network access
- Not all Notation plugins support this; verify with your implementation

### 3.2 Long-lived OT assets (10+ year lifecycle)

OT devices commonly operate for 10–25 years. Code-signing certificates typically have 2–3 year validity periods. Without timestamping, all signatures become unverifiable after certificate expiry.

**Why TSA is critical:**

```
Timeline:
  Year 0: Developer signs artifact (cert valid 2022-2025)
  Year 0: TSA timestamps signature (proves signing occurred in 2022)
  Year 3: Certificate expires (2025)
  Year 5: Device needs to verify artifact...
  
  WITHOUT TSA: Verification FAILS — cert expired, no proof of signing time
  WITH TSA:    Verification PASSES — TSA proves signature existed before expiry
```

**Planning for certificate rotation:**

1. Sign new artifact versions with renewed certificates as part of regular release cycles
2. Maintain previous root CAs in trust stores until all artifacts signed under them are decommissioned
3. When root CA rotates, publish a new trust bundle version (`:v2.0.0`) and communicate to operators
4. Operators add the new root CA to trust stores — old and new coexist during transition

**TSA certificate lifecycle:**
- TSA certificates also expire. Ensure TSA root CAs are in `tsa:<name>` trust stores
- TSA timestamps are self-contained — they remain valid as long as the TSA root CA is trusted
- Rotate TSA root CAs with the same caution as signing CAs

### 3.3 Constrained edge devices

Edge devices in OT environments may have limited CPU, memory, and storage. The normative spec is designed to accommodate these constraints:

**Format choice:**
- COSE_Sign1 (binary CBOR) is significantly more compact than JWS (JSON text)
- COSE parsing requires less memory allocation — no Base64 decode step
- For devices with <128MB RAM, COSE should be the only format used by publishers

**Implementation options:**
- The verification algorithm is specified abstractly — implementations in C, Rust, or other native languages are possible without a Go runtime or full Notation CLI
- Minimal viable implementation: COSE_Sign1 parsing + ECDSA P-256 verification + X.509 chain building
- Libraries: `t_cose` (C), `cose-rust` (Rust), `go-cose` (Go)

**Storage considerations:**
- Trust stores (root CA certificates) typically require <10KB total
- CRL cache: size varies; plan for 10–100KB per CA
- Trust policy: <2KB (JSON text)

### 3.4 Hardware trust anchors

**TPM 2.0 (recommended where available):**
- Seal trust store decryption keys to TPM PCR values
- Ensures trust store integrity — if device firmware is tampered, PCR values change, trust store becomes inaccessible
- Use TPM for measured boot chain validation before signature verification begins

**Alternative tamper-detection (when TPM unavailable):**
- dm-verity: read-only verified filesystem for trust store partition
- IMA (Integrity Measurement Architecture): kernel-level file integrity monitoring
- Filesystem permissions: root-owned, mode 0444, immutable flag (`chattr +i`)
- Secure boot: UEFI Secure Boot chain validates kernel → initrd → rootfs integrity

---


## 4. Regulatory and compliance alignment

This section maps SUP-03 capabilities to common industrial cybersecurity standards. This mapping is informative — formal compliance assessments require engagement with qualified auditors.

### 4.1 IEC 62443-4-2 CR 3.9 — Software Integrity

**Requirement:** "The component shall provide the capability to verify the integrity of software before installation."

**How SUP-03 satisfies this:**
- Content-addressable SHA-256 digests provide tamper-evident integrity for all OCI layers
- X.509 signature verification at the edge device (the "component") before any archive extraction or workload deployment
- The 10-step verification engine in the normative spec (§6) provides a complete, auditable verification procedure

**Evidence for auditors:** Verification engine logs showing step-by-step pass/fail for each artifact deployment.

### 4.2 NIST SP 800-218 (SSDF) — Secure Software Development Framework

**Requirement:** PO.1.3 — "Verify the integrity and check the provenance of acquired software."

**How SUP-03 satisfies this:**
- Notation signatures bound to manifest digests prove provenance (who signed) and integrity (what was signed)
- Trust policy identity matching (`trustedIdentities`) restricts deployment to known, authorized publishers
- Vendor trust bundles provide the provenance chain from signing identity back to a trust anchor

### 4.3 NIST SP 800-207 — Zero Trust Architecture

**Requirement:** "Never trust, always verify" — every access request and resource deployment must be independently validated.

**How SUP-03 satisfies this:**
- Edge devices independently verify signatures using local trust stores — no reliance on WFM or upstream services at verification time
- Fail-closed default: absent trust policy = `strict` = reject everything unsigned
- Device-level enforcement boundary: even if WFM is compromised, edge devices reject unverifiable artifacts

### 4.4 ISO/IEC 27001 A.12.6.2 — Restrictions on Software Installation

**Requirement:** "Rules governing the installation of software by users should be established and implemented."

**How SUP-03 satisfies this:**
- Subject DN matching in `trustedIdentities` restricts which signing identities are authorized per registry scope
- Only artifacts signed by authorized identities pass verification
- Policy is declarative and auditable — `trustpolicy.json` documents exactly who is trusted for what

### 4.5 ETSI EN 319 102-1 — Relationship

The SUP-03 verification engine (normative §6) is informed by ETSI EN 319 102-1 (Procedures for Creation and Validation of AdES Digital Signatures). The 10-step algorithm covers the same fundamental checks:

1. Signature cryptographic validation
2. Certificate chain building and path validation
3. Certificate status checking (revocation)
4. Timestamp token validation
5. Signing certificate attribute verification (EKU, Key Usage)

Organizations seeking **formal ETSI compliance** should reference ETSI EN 319 102-1 V1.4.1+ directly. The SUP-03 verification engine can satisfy ETSI requirements when:
- Certificates are issued by ETSI EN 319 411-1/2 conformant CAs
- Timestamps are produced by ETSI EN 319 422 conformant TSAs
- Signing keys are stored in ETSI EN 419 221-5 conformant HSMs

**Formal ETSI compliance is not required by SUP-03.** The verification engine stands on its own technical merits independent of any ETSI conformance claim.

### 4.6 eIDAS Electronic Seals — Detailed Guidance

#### What are Electronic Seals?

Under eIDAS, an Electronic Seal is the organizational equivalent of an electronic signature. While signatures are linked to natural persons, seals are linked to legal persons (organizations). For Margo artifact signing, eSeals are the correct eIDAS instrument — the signing identity is the software vendor organization, not an individual developer.

#### Advanced vs Qualified: Decision factors

| Factor | Advanced eSeal | Qualified eSeal |
|---|---|---|
| Legal presumption of integrity | No (but strong evidential value) | Yes (Art. 35(2) eIDAS) |
| Admissible in court across EU | Yes (cannot be denied legal effect) | Yes (presumption of integrity of data) |
| HSM/QSCD required | No | Yes |
| CI/CD automation friendly | Yes (software keys in pipelines) | Challenging (HSM signing ceremonies) |
| Cost | Lower (€50–200/year) | Higher (€200–800/year + HSM) |
| Regulatory audit weight | Medium | Highest |

**Recommendation for most Margo ISVs:** Start with Advanced eSeals. They provide third-party trust, revocation, and TSA without requiring HSM infrastructure. Upgrade to Qualified eSeals only if contractual or regulatory requirements demand it.

#### Integration with Margo verification

At the verification layer, Advanced and Qualified eSeal signatures are **technically indistinguishable**. Both produce standard X.509 Notation signatures. The WFM/device verifier:
1. Builds certificate chain to QTSP root CA (in `ca:<store>` trust store)
2. Matches Subject DN against `trustedIdentities`
3. Validates TSA timestamp (QTSP-issued TSA cert in `tsa:<store>`)

The verifier does not know or care whether the signing key was in software (Advanced) or HSM (Qualified).

---


## 5. Deployment decision tree

Use this decision tree to select the appropriate PKI mode:

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

## 6. Migration paths

### 6.1 From Self-Signed (Mode A) to Corporate PKI (Mode B)

1. **Set up enterprise CA** — Deploy root CA (offline HSM) + intermediate CA (online) + OCSP + TSA
2. **Issue new code-signing certificates** — Generate keys in HSM, issue certs from enterprise CA
3. **Re-sign artifacts** — Sign all current artifacts with new enterprise certificates (old signatures remain valid alongside new ones — multiple signatures are supported)
4. **Update trust bundles** — Publish new vendor trust bundle containing enterprise root CA
5. **Operator transition** — Operators add enterprise root CA to trust stores; old self-signed root CA can be removed after transition period
6. **Policy remains unchanged** — `strict` level still works; only trust stores and trusted identities change in `trustpolicy.json`

### 6.2 From Self-Signed (Mode A) to QTSP (Mode C)

1. **Obtain eSeal certificate** from chosen QTSP
2. **Re-sign artifacts** with QTSP-issued certificate (+ QTSP TSA)
3. **Update trust bundle** — Replace self-signed root CA with QTSP root CA
4. **Operator transition** — Operators replace self-signed root CA in trust stores with QTSP root CA; update `trustedIdentities` to match new Subject DN

### 6.3 From Permissive to Strict verification

This requires **no artifact changes at all** — it is a policy-only transition:

1. Start with `"level": "audit"` — all checks run, nothing is blocked, violations are logged
2. Review audit logs — identify any artifacts/signatures that would fail under strict
3. Fix identified issues (missing TSA, expired certs, wrong identity)
4. Transition to `"level": "permissive"` — crypto failures block, others warn
5. Review remaining warnings
6. Transition to `"level": "strict"` — full enforcement

This phased approach is safe because verification level changes never require re-signing artifacts.

---

## 7. References

### Normative specification
- [SUP-03: OCI Cryptographic Signature Architecture and Trust Verification Framework](sup-03-oci-signature.md) — the normative companion to this guide

### Standards and regulations
- [eIDAS Regulation (EU) No 910/2014](https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=uriserv:OJ.L_.2014.257.01.0073.01.ENG) — Electronic identification and trust services
- [eIDAS 2.0 (EU) 2024/1183](https://eur-lex.europa.eu/eli/reg/2024/1183/oj) — Amendment establishing European Digital Identity Framework
- [EU Trusted List Browser](https://eidas.ec.europa.eu/efda/tl-browser/) — Find QTSPs by country and service type
- [ETSI EN 319 102-1](https://www.etsi.org/deliver/etsi_en/319100_319199/31910201/) — Procedures for Creation and Validation of AdES Digital Signatures
- [ETSI EN 319 411-1](https://www.etsi.org/deliver/etsi_en/319400_319499/31941101/) — Policy and security requirements for TSPs issuing certificates
- [ETSI EN 319 412-2](https://www.etsi.org/deliver/etsi_en/319400_319499/31941202/) — Certificate Profiles for certificates issued to natural persons
- [ETSI EN 319 422](https://www.etsi.org/deliver/etsi_en/319400_319499/319422/) — Time-stamping protocol and profiles
- [IEC 62443-4-2](https://webstore.iec.ch/publication/34421) — Security for Industrial Automation and Control Systems: Technical security requirements for IACS components
- [NIST SP 800-218](https://csrc.nist.gov/publications/detail/sp/800-218/final) — Secure Software Development Framework (SSDF)
- [NIST SP 800-207](https://csrc.nist.gov/publications/detail/sp/800-207/final) — Zero Trust Architecture

### Tooling and specifications
- [CNCF Notary Project — Notation CLI](https://github.com/notaryproject/notation) — Reference signing/verification implementation
- [CNCF Notary Project — Trust Store and Trust Policy Specification](https://github.com/notaryproject/specifications/blob/main/specs/trust-store-trust-policy.md)
- [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md)
- [RFC 9052 — COSE (CBOR Object Signing and Encryption)](https://www.rfc-editor.org/rfc/rfc9052)
- [RFC 7515 — JSON Web Signature (JWS)](https://www.rfc-editor.org/rfc/rfc7515)
- [RFC 3161 — Time-Stamp Protocol (TSP)](https://www.rfc-editor.org/rfc/rfc3161)
- [RFC 6960 — Online Certificate Status Protocol (OCSP)](https://www.rfc-editor.org/rfc/rfc6960)

---

*This document is informative. All normative requirements are defined in [SUP-03](sup-03-oci-signature.md).*

*Prepared by Margo Security Working Group, 2026-07-27. Subject to the Open Web Foundation Contributor License Agreement governing the Margo specification.*
