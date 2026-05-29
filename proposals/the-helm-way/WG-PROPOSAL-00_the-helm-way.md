# WG-PROPOSAL-00: The Helm Way -- Canonical OCI Component Publishing Pattern

| Field | Value |
|---|---|
| Date | 2026-05-04 |
| Category | Informational |
| Issues | None (pattern definition) |
| Affects | None (no spec diffs) |
| Status | Rev 1 -- ready for public review |

## Purpose

The Margo specification already provides a mature, OCI-native publishing workflow for Helm-based components. This document names that workflow "The Helm Way" and distills it into five reusable elements so that future component types (Compose, Quadlet, or any type added later) can reference a single, well-defined pattern rather than repeating the same requirements in every proposal. This document is informational only -- it introduces no normative changes and does not modify any specification file.

> **Note on signing**: Artifact signing (provenance, supply chain attestation) is out of scope for this proposal package. The OCI content-addressable digest already provides transport integrity. Signing policy belongs in a dedicated Margo security specification and is deferred to a future proposal.

## The Pattern

"The Helm Way" is a five-element pattern for storing, referencing, identifying, verifying, and parameterizing component artifacts within the Margo ecosystem. Any component type that follows all five elements is said to conform to The Helm Way.

### 1. Storage

The component artifact MUST be stored in an OCI-compliant [Component Registry](../../specification/applications/application-registry.md). The registry MUST support OCI artifacts as defined in the OCI Image Specification v1.1.0. The publishing tool (e.g., `helm push`, `oras push`) MUST produce a valid OCI image manifest.

### 2. Reference

The [ApplicationDescription](../../specification/applications/application-description.md#componentproperties-attributes) MUST identify the component using two fields:

- `repository` -- an `oci://` URI pointing to the registry and repository path (e.g., `oci://registry.example.com/org/component-name`)
- `revision` -- an OCI tag identifying the component version (e.g., `1.0.0`, `2.3.1`) (SemVer 2.0 version without leading `v`)

Together, `repository` + `revision` provide a deterministic, human-readable coordinate for the artifact. No other location field (e.g., a plain-HTTPS URL) is required.

### 3. Media Types

The OCI image manifest MUST declare a component-type-specific `artifactType` so that registries and tooling can distinguish Margo component artifacts from other OCI content. The layer blob containing the component payload MUST declare a component-type-specific `mediaType` so that consumers can verify they are processing the correct binary format.

Media type strings MUST follow the `application/vnd.org.margo.<hierarchy>` naming convention (reversed domain `org.margo` for the Margo project domain `margo.org`) and MUST be registered in the Margo-Specific Media Types table in [application-registry.md](../../specification/applications/application-registry.md).

### 4. Integrity

Integrity verification is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md), which the Margo Application Registry already requires. Implementations MUST verify the digest after pulling and before extracting or processing the artifact. No separate `digest` or hash field is needed in the ApplicationDescription.

### 5. Parametrization

Each component type SHOULD define how ApplicationDescription `Parameter` values are delivered
to the running component at deployment time. The delivery mechanism is component-type-specific
and declared in the component type proposal.

Each new component type proposal SHOULD specify:
- The **parameter artifact** -- the file or mechanism by which resolved parameter values reach
  the component (e.g., a values override file, an env file, a configuration file).
- The **`pointer` syntax** -- the convention used in `Target.pointer` to identify the
  parameter's location within the artifact (e.g., dot-notation for a YAML tree, a named
  environment variable).

A single `Parameter` MAY declare targets with different pointer syntaxes, allowing the same
logical value to be delivered to components of different types simultaneously.

## Helm as the Reference Implementation

Helm already satisfies all five elements of The Helm Way:

- **Storage**: `helm push` stores Helm charts in any OCI-compliant registry.
- **Reference**: The ApplicationDescription uses `repository: oci://...` and `revision: <tag>` to identify the chart.
- **Media types**: The OCI manifest uses `artifactType: application/vnd.cncf.helm.config.v1+json` and the layer blob uses `mediaType: application/vnd.cncf.helm.chart.content.layer.v1.tar+gzip`, both community-standard CNCF media types.
- **Integrity**: The OCI content-addressable digest on the chart layer blob provides integrity verification per the OCI Distribution Specification v1.1.0.
- **Parametrization**: `helm install` accepts a `values.yaml` override file (via `-f values.yaml`)
  or individual `--set` flags. The ApplicationDescription `Target.pointer` uses dot-notation to
  identify the target key within the values tree (e.g., `config.database.host`).

Helm's compliance with this pattern is inherent -- no specification changes are required for Helm components. The Helm Way simply names what Helm already does.

## Pattern Compliance Table

Any new component type proposal MUST demonstrate compliance with all five elements. The table below shows the reference implementation (Helm) alongside the two component types introduced by WG-PROPOSAL-01 and WG-PROPOSAL-02.

| Element | Requirement | Helm (reference) | Compose (P-01) | Quadlet (P-02) |
|---------|------------|-----------------|----------------|----------------|
| Registry | OCI-compliant, MUST | `helm push` | `oras push` | `oras push` |
| `type` value | versioned string, MUST | `helm.v3` | `compose.v1` | `quadlet.v1` |
| `repository` field | `oci://` URI, MUST | `oci://registry/org/chart` | `oci://registry/org/app` | `oci://registry/org/app` |
| `revision` field | OCI tag, MUST | `1.0.0` | `1.0.0` | `1.0.0` |
| `artifactType` | Margo-registered, MUST | `application/vnd.cncf.helm.config.v1+json` | `application/vnd.org.margo.component.compose.v1+json` | `application/vnd.org.margo.component.quadlet.v1+json` |
| Layer `mediaType` | Margo-registered, MUST | `application/vnd.cncf.helm.chart.content.layer.v1.tar+gzip` | `application/vnd.org.margo.component.compose.v1.tar.gzip` | `application/vnd.org.margo.component.quadlet.v1.tar.gzip` |
| Integrity | OCI digest per OCI Distribution Spec v1.1.0 | Verified by registry and Helm CLI | Verified by registry and WFM | Verified by registry and WFM |
| Parametrization | parameter artifact SHOULD be defined | `values.yaml` override | `margo-params.env` file | `margo-params.env` file (shared with Compose) |

## Relationship to Other Proposals

This document is informational only. It does not modify any normative specification file.

The normative changes that apply The Helm Way to Compose components are defined in **WG-PROPOSAL-01** (`WG-PROPOSAL-01_compose-oci-and-structure.md`). The normative changes that apply The Helm Way to Quadlet components are defined in **WG-PROPOSAL-02** (`WG-PROPOSAL-02_quadlet-component-type.md`).

Component type proposals that define a parameter artifact and pointer syntax SHOULD reference the Parametrization element and fill in the corresponding row of the Pattern Compliance Table.

Future component type proposals SHOULD reference this document and fill in the Pattern Compliance Table to demonstrate conformance with The Helm Way.

---

## References

- [OCI Image Specification v1.1.0](https://github.com/opencontainers/image-spec/blob/v1.1.0/manifest.md)
- [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md)
- [Helm OCI Support](https://helm.sh/docs/topics/registries/)
- [ORAS (OCI Registry as Storage)](https://oras.land/)
- [RFC 6838 -- Media Type Specifications and Registration Procedures](https://www.rfc-editor.org/rfc/rfc6838)

---

*This document is part of the Margo WG Compose/Quadlet OCI Publishing proposal package. Prepared by Andrii Melashchenko, 2026-05-04. Subject to the Open Web Foundation Contributor License Agreement governing the Margo specification.*
