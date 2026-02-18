# Proposal for Artifacts Signing

This proposal addresses
[feature #138](https://github.com/margo/specification/issues/138):
"Define the Margo application image and component signing strategy"

## The Importance of Signing Margo Artifacts

Digitally signing the various artifacts within Margo workflows is a critical
practice that underpins security, trust, and accountability.
This process is essential for the following key reasons:

- Integrity
- Authenticity
- Non-Repudiation

### Integrity

Ensuring artifact integrity guarantees that software artifacts remain precisely
as intended, free from unauthorized or accidental alteration.

Why it's crucial:

- Protection against malicious modification:
  Signed artifacts provide a robust defense against malicious actors who might
  attempt to inject malware, backdoors, or other harmful code.
- Safeguarding against unintended changes:
  It also protects against accidental modifications or incomplete provisioning
  that could lead to unexpected behavior or system failures.

By verifying an artifact's signature, we can confirm that its content is
exactly what was expected, assuring its reliability and safety.

### Authenticity

In Margo's interoperable ecosystem, where multiple components originate from
diverse actors, verifying the source of an artifact is paramount.
Authenticity ensures that an artifact genuinely comes from its declared and
trusted organization.

Beyond the signature:
- While an artifact's signature confirms its integrity, verifying authenticity
  requires an additional step.
  It is crucial that the signature can be reliably traced back to a specific,
  recognized organization, thereby establishing a clear chain of trust.

### Non-Repudiation

In the industrial domain, which Margo targets, clear accountability for software
artifacts is vital due to potential liabilities and commercial implications.
Non-repudiation ensures that the signer of an artifact cannot legitimately deny
its authorship.

Commercial and legal implications:
- Establishing accountability:
  If an artifact causes damage or unexpected issues, the ability to
  unequivocally identify the responsible organization is essential for assigning
  liability.
- Preventing misattribution of costs:
  Without non-repudiation, an organization could disclaim responsibility for a
  faulty artifact, potentially shifting the financial burden and legal
  consequences onto another, innocent party.

Therefore, digital signatures provide a verifiable record of authorship,
securing both the integrity of the software supply chain and the commercial
interests of all stakeholders.

## Artifacts Subject to Signing

While any data is eligible for signing, the following are current Margo
artifacts that are strong candidates for digital signatures:

- Application packages
- Desired state

**Philip Comment**: I think it makes sense to require the application package, Helm charts, and container images to be signed. I would also suggest the proposal updates the spec to indicate the Compose tar files should be signed with Sigstore/Cosign as well, instead of GPG. I don't know that the desired state needs to be signed because this is communicated directly through the API. There is already some security in place for how the API works, so I don't know that signing the desired state adds anything. We could potentially indicate that the device can be configured to allow unsigned artifacts, but it seems like the default would be to require them all to be signed, and then the customer could choose to change this to just a warning or something if they are not signed, if the device allows it.

## Requirements for the Signing Solution

The proposed signing solution must address the following critical requirements:

- Avoid Reinventing the Wheel
- Flexibility
- Support for Air-Gapped Scenarios

### Avoid Reinventing the Wheel

The landscape of software artifact signing is rich with multiple
well-established tools, technologies, and approaches, developed and refined by
security experts.

- Leverage existing expertise:
  Attempting to create a proprietary Margo signing mechanism is inherently prone
  to being less secure and less robust than existing, battle-tested solutions.
- Focus on Margo-specific value:
  The proposed solution should therefore rely on the best-fitting existing
  technologies, allowing Margo development efforts to concentrate on adding
  unique value to its ecosystem rather than duplicating complex security
  infrastructure. Developing security-critical software like a signing mechanism
  is a complex endeavor that would divert valuable resources from Margo's core
  mission.

### Flexibility

The solution must be adaptable to various artifact types and diverse user
personas within the Margo ecosystem.

- Broad applicability:
  It should be capable of signing all identified strong candidate artifacts and
  remain open to accommodating future artifact types.
- Support for diverse stakeholders:
  Margo involves different personas with varying capabilities and expectations
  regarding signing.
  For instance:
    - Large corporations often possess their own certificate infrastructure.
    - Small application developers might rely on freely available certification
      services.
    - Consumers may obtain applications from third-party sources rather than
      directly from authors.
The proposed solution must gracefully accommodate this diversity.

**Philip Comment**: One of the things we need to be careful of here is having CAs/trust bundles that are too generic. For example, if someone just has a Digicert CA that could potentially be used to verify a lot of different things. It might not be possible to make this a requirement, but we should at least make it a recommended practice to have specific trust chains used for this purpose that don't introduce security risk for CAs being used to verify more than what is intended.

### Support for Air-Gapped Scenarios

Margo's application domain includes numerous devices that may not have direct
internet connectivity but still require the ability to verify artifact
signatures.
These devices operate in air-gapped environments.

- Offline verification capability:
  The solution proposed in this document MUST support the possibility of
  verifying software artifact signature(s) without requiring access to
  internet-dependent services.
- Full offline mode (ideal):
  Ideally, the solution would even support a complete offline verification
  mode.

## Proposed Solution

In order to cover the above mentioned requirements,
this Specification-Update-Proposal (SUP) proposes the use of the Open Source
Software (OSS) solution provided by [Sigstore](https://www.sigstore.dev/) and
relying primarily on the component [Cosign](https://docs.sigstore.dev/cosign/)
in its version v3 for signing and verifying.

The use of Cosign for signing and verifying not only covers the above mentioned
requirements, but also incorporates other features that will be explained in
this section of the document.

### Well-Established Solution

Cosign is a software component of the Sigstore project that is around 5 years in
use for productive systems consuming container images.

Initially is was created to
[sign container images](https://blog.sigstore.dev/cosign-signed-container-images-c1016862618/),
but nowadays it can be used to sign any kind of software artifact (from a file
to any string or binary).

The project has the participation of developers, organizations and companies
with a strong background in security like
[ChainGuard](https://www.chainguard.dev/) or the
[Secure Systems Lab](https://ssl.engineering.nyu.edu/overview/) of the NYU.

It integrates pre-existing and battle-proofed mechanisms and algorithms adding
to them other mechanisms covering aspects of signing not well-covered by those
integrated solutions.

In the end Cosign is a mature and secure approach to software artifacts signing.

### Openness

Due to its architecture, Cosign is very open to different setups that can help
addressing the above mentioned requirements.

Openness has two facets:
It offers many options but forces decisions.
Additionally, it increases potential incompatibilities.

It is out of the scope of Margo mandating an approach on signing (at least
within the scope of what is supported by Cosign).
That way it's a relationship between artifact creators and consumers in which
Margo remains unopinionated.

Cosign's openness, unconstrained by Margo requirements, fosters a lively
and potentially heterogeneous ecosystem.

#### Signing

Cosign supports the use of:

- Self-signed certificates
- CA-signed certificates
- So-called "keyless signing" using freely available Sigstore services
- So-called "keyless signing" using self-hosted Sigstore services

It is out of the scope of Margo mandating which of these approaches must be
supported.
Each one of them enables different scenarios
but have also different implications.

#### Verification

Verification depends strongly on the signing approach.

Any of the above mentioned approaches for signing is technically supported by
Cosign.
Therefore they can be easily supported if no artificial limitation is added,
for example with some policies.

#### Self-signed Certificates

If the signer has used self-signed certificates, then the corresponding public
keys need to be made somehow available for verification.

Different possibilities exist for making self-signed certificates available to
consumers:

- Out-of-band distribution: Through email, chat, or other communication channels
- Enterprise tools:
  Using existing PKI infrastructure, certificate management systems, or internal
  artifact repositories
- Artifacts registry:
  Embedding public keys within the artifacts registry metadata
- Physical distribution:
  On physical media like USB drives, smart cards, or hardware tokens
- Documentation:
  Publishing keys in publicly accessible documentation or websites
- TUF (The Update Framework):
  Leveraging TUF's trusted delegation system for key distribution

#### CA-signed Certificates

If the signer uses CA-signed certificates, then a consumer needs to obtain
the corresponding CA certificate(s) and add them to their trust store.

**Philip Comment**: We have an opinion on this that we'd rather any CA/trust bundles be added directly to the device by the device vendor or customer instead of those coming through the workload fleet manager. We can discuss this some and see. Part of this is a trust issue; if the workload fleet manager is providing the trust bundles, they could potentially also be recreating the artifacts and signing them with their own keys. 

Different possibilities exist for making CA-signed certificates available to
consumers:

- System trust stores:
  Pre-installed CA certificates in operating systems and applications
- Enterprise PKI:
  Corporations can push CA certificates to all devices in their domain using
  group policies or MDM
- Public CAs:
  Well-known public certificate authorities are already included in most trust
  stores
- Certificate bundles:
  CA certificates can be distributed in bundles alongside artifacts
- Directory services:
  Using LDAP or other directory services for certificate distribution
- Configuration management:
  Tools like Ansible, Chef, Puppet, or SaltStack can distribute CA certificates
- Platform-specific mechanisms:
  Vendor-specific platforms may have built-in trust distribution mechanisms
- Cloud platforms:
  Cloud providers often have trusted CA infrastructure and utilities for
  certificate management

#### Keyless Signing (Public Sigstore Services)

Keyless signing is a signing approach that eliminates the need for the signer
to manage long-lived private keys.
Instead, it uses Sigstore's publicly available services:

- [Fulcio](https://github.com/sigstore/fulcio):
  A certificate authority that issues short-lived certificates based on OpenID
  Connect (OIDC) identity
- [Rekor](https://github.com/sigstore/rekor):
  A transparency log that records all signed artifacts and their signatures

How it works:

1. The signer authenticates using an OIDC identity provider (e.g., GitHub,
   Google, corporate SSO)
2. Fulcio issues a short-lived certificate containing the identity information
3. The artifact is signed with the short-lived certificate
4. The signature and certificate are recorded in Rekor's transparency log
5. Rekor issues a log entry that can be used to verify that the short-lived
   certificate was valid a the time of signing

Benefits:

- No long-lived private key management required
- Identity is embedded in the certificate
- All signatures are transparent and publicly auditable
- Time-bound certificates (typically 10 minutes expiration), which reduce
  risk of leaked certificate misuse
- Simplified key rotation (automatic with short-lived certificates)

Requirements for signers:

- Access to the public Sigstore services (internet connectivity)
- OIDC identity provider (GitHub, Google, email-based, or custom)
- Authentication credentials for the OIDC provider

Verification:

With Cosign v3, consumers verify signatures by:
1. Using the trusted Trust Root document to validate the signature
2. The Trust Root contains all necessary verification information (keys,
   transparency log data)
3. Optionally checking the identity claims embedded in the certificate

Trust model:

Consumers trust the Trust Root document obtained from official Sigstore
infrastructure.

For air-gapped environments:
Consumers need to pre-provision the Trust Root document instead of relying on
network access.

#### Keyless Signing (Self-Hosted Sigstore Services)

Self-hosted keyless signing provides the same fundamental approach as public
Sigstore services but with full control over the infrastructure.

Key differences from public services:

Infrastructure ownership:
- Fulcio and Rekor instances are deployed and maintained within your
  organization
- Requires dedicated operations resources for deployment, maintenance, and
  monitoring
- Greater control over availability and configuration

Identity providers:
- Can use internal OIDC providers (corporate SSO, internal systems)
- Support for custom identity claims and verification policies
- Often tighter integration with existing authentication infrastructure

**Trust model:**
- Consumers trust your organization's Trust Root document
- Potentially simplified trust if using corporate PKI and pre-shared Trust Roots
- Verification requires only the Trust Root document, not separate keys

**Air-gapped scenarios:**
- Can operate entirely without internet connectivity
- All components (Fulcio, Rekor, artifact registry) can be internal
- More suitable for highly restricted environments
- Trust Root document can be distributed offline

**Operational considerations:**
- Signers require access to your internal infrastructure
- Verification requires only the pre-provisioned Trust Root document
- Requires proper backup and disaster recovery planning for Trust Root
  distribution

Security trade-offs:
- Greater control over security configurations
- Potential for more tailored identity verifications
- Additional responsibility for securing infrastructure components

#### Consequences

Both artifact creators as well as consumers must be aware of the consequences
of taking one approach or another.

An application creator which decides to use self-signed certificates must be
aware that it will be losing some consumers that only trust some root
certificates.
But on the other side, a small company providing a custom application for
another company can take the low-effort of self-signed certificates knowing on
advance, that the consumer will trust them.

An application creator which wants to reach a wide audience will remain
abstinent from using self-signed certificates and will go for CA-signed
certificates.

An application consumer with high security requirements might only accept
keyless signing approaches, implicitly excluding some applications with that
decission.

### Extensibility

Signatures are not embedded into the artifacts, therefore it's very easy to:

- Update signatures
- Append additional signatures 

For packages using OCI mechanisms for distribution, it's also
very easy to generate packages containing the signatures embedded in them.

