# PR3 inputs

This directory collects exploratory design work that feeds into the **PR3** phase of the [Margo Identity and Authorization Framework](../margo-identity-and-authorization-framework.md). The active PR2 specification is narrower than these documents assume; they are preserved as candidate inputs to PR3 deliberations, **not** as committed direction.

## Status

The active normative specification is:

- **[Margo Identity and Authorization Framework](../margo-identity-and-authorization-framework.md)** (PR2)
- **[Margo WFM Identity Profile and Management Interface Update](../wfm-identity-profile.md)** (PR2)

PR2 defers a number of decisions — automated enrollment / renewal / revocation protocols, JWT-SVID support, a device identity profile, the bootstrap mechanism set, the WFM Client enrollment protocol, traffic-inspecting-proxy authentication, and a native revocation mechanism. Most of these are scoped to PR3; some (notably the device identity profile, which pairs naturally with DFM work) may land later. The documents in this directory are working sketches that the relevant release may engage with, replace, or set aside as it makes its decisions.

Some of these documents were written when MIAF specified an enrollment-protocol surface and a bootstrap-method registry (the v0 draft). PR2 narrowed MIAF's scope; design assumptions in such documents do not reflect the current MIAF specification. They remain useful as analysis input.

## Index by PR3 topic

Items can feed into more than one PR3 decision; the grouping below reflects the primary one.

### Enrollment, renewal, and revocation protocol

PR3 will select the protocol(s) used to issue, renew, and revoke SVIDs. Candidates under consideration include Lightweight CMP ([RFC 9483](https://datatracker.ietf.org/doc/html/rfc9483)), EST ([RFC 7030](https://datatracker.ietf.org/doc/html/rfc7030)), ACME with Device Attestation, and a Margo-specific JSON API.

- **[miaf-margo-json-enrollment-protocol.md](miaf-margo-json-enrollment-protocol.md)** — Captures the enrollment surface specified by the MIAF v0 draft (the "Margo-specific JSON API" candidate): `POST /api/v1/identities`, `POST /api/v1/identities/{id}/renewal`, the `bootstrapCredential` envelope, the Enrollment Subject Identifier (ESI) concept, rate-limiting policies, and the RFC 9457 error model.
- **[miaf-revocation-list.md](miaf-revocation-list.md)** — MIS revocation list endpoint + JSON revocation model. One candidate input to the PR3 revocation mechanism.

### Bootstrap mechanisms

PR3 will define the set of bootstrap mechanisms by which a principal obtains its first SVID.

- **[miaf-factory-certificate-bootstrap-method.md](miaf-factory-certificate-bootstrap-method.md)** — Pre-provisioned X.509 client certificate (e.g., IEEE 802.1AR DevID) presented via mTLS, with ESI derived as the SHA-256 of the leaf certificate.
- **[miaf-fdo-bootstrap-method.md](miaf-fdo-bootstrap-method.md)** — FIDO Device Onboard.
- **[miaf-enrollment-token-bootstrap-method.md](miaf-enrollment-token-bootstrap-method.md)** — Operator-issued enrollment token.

### Device identity profile

A normative device identity profile is on the table for PR3 but not yet committed; it may land later alongside DFM work post-GA. When it is specified, the framing decision — foundation for WFM Client identity vs. peer profile — is decided then.

- **[miaf-edge-compute-device-identity-profile.md](miaf-edge-compute-device-identity-profile.md)** — The MIAF v0 draft of the Edge Compute Device Identity Profile: Logical Device Identity (LDI) under `/margo/device/<uuid-v4>`, profile-specific SVID constraints, device key protection, and profile-specific enrollment rules.
- **[miaf-device-replacement.md](miaf-device-replacement.md)** — LDI rebinding / replacement semantics after hardware replacement, with the `replacementAuthorization` request field.

### WFM Client enrollment

PR3 will specify the automated path by which a WFM Client receives its initial SVID. Operator-pre-provisioning (the PR2 baseline) remains a valid path in PR3.

- **[wfm-client-binding-assertion-onboarding.md](wfm-client-binding-assertion-onboarding.md)** — The original WFM Client onboarding design from the v0 SUP draft: a WFM-issued JWT Binding Assertion validated by MIS at issuance, with ESI derivation from `wfm-id || wfm-client-id`, and the supporting Trust Domain signer policy and replay/DoS controls.

### JWT-SVID and traffic-inspecting-proxy environments

PR3 will decide whether MIAF supports JWT-SVIDs and how to authenticate in environments where end-to-end mTLS is operationally infeasible.

- **[miaf-non-mtls-environments.md](miaf-non-mtls-environments.md)** — JWT-SVID profile, JWT-SVID Exchange endpoint, JWT-Bearer authentication for SVID renewal, and a Factory Certificate (JWT Assertion) bootstrap method variant.

### Multi-holder identities and cluster topology

PR2 covers single-holder identities. Multi-node cluster topology with credential sharing across cooperating nodes is out of scope for PR2.

- **[miaf-multi-holder-identities-and-cluster-topology.md](miaf-multi-holder-identities-and-cluster-topology.md)** — Framework-level multi-holder logical identity primitive and a WFM Client cluster-topology profile, **contingent on PR3 choosing the device-as-foundation direction for WFM Client identity**. Wire-compatible with the single-holder SVIDs PR2 already specifies; independent of the PR3 enrollment protocol choice. Under the peer-profile direction this primitive is not needed.

### Independent extensions

Items below are not tied to any specific PR3 decision; they could be promoted through the standard SUP workflow at any time without breaking changes to PR2.

- **[miaf-oauth2-bridge.md](miaf-oauth2-bridge.md)** — OAuth 2.0 Token Exchange bridge mapping MIAF SVIDs onto OAuth access tokens for API-gateway interop. Informative only; no MIAF compliance impact.

## Document status

Documents in this directory are not on the TWG decision path until material is promoted out of `pr3-inputs/` and through the standard SUP workflow (typically as part of PR3). They may be edited, split, merged, or removed as PR3 work progresses.
