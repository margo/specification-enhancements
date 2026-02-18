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

#### Signing

Cosign supports the use of:

- Self-signed certificates
- CA-signed certificates
- So-walled "keyless singing" using freely available Sigstore services
- So-walled "keyless singing" using self-hosted Sigstore services

This enables different scenarios with different implications.

##### Verification

Verification depends strongly on the signing approach.

If the signer has used self-signed certificates, then the corresponding public
keys need to be made somehow reachable for verification.

If the signer uses self-signed certificates, then a consumer needs to obtain
the corresponding public keys and explicitly trust them.
