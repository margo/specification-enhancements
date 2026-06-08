# The Helm Way: Canonical OCI Component Publishing Pattern

## Owner

[@javatask](https://github.com/javatask) — Andrii Melashchenko, Belden Inc.

## Summary

The Margo specification already contains a complete OCI-native publishing workflow for Helm-based components. This SUP names that workflow "The Helm Way" and distills it into five reusable elements: **Storage**, **Reference**, **Media Types**, **Integrity**, and **Parametrization**.

This is an informational SUP — it introduces no normative changes and modifies no specification files. Its sole purpose is to define a named, checkable pattern so that future component type SUPs (starting with SUP-01 Compose and SUP-02 Quadlet) can reference a single document rather than re-stating the same five requirements independently.

## Reason for proposal

The Margo specification already mandates OCI registry storage for application packages (established by the completed `sup_app_registry_as_oci` SUP) and already uses `repository` + `revision` fields in `ComponentProperties` to identify Helm charts. However, this OCI publishing contract is not named, not defined as a reusable pattern, and not checked against a compliance table.

Without a named pattern:
- Every new component type SUP must re-specify all five publishing elements from scratch.
- There is no single place a reviewer can check that a new component type (Compose, Quadlet, or any future type) follows the same OCI contract as Helm.
- Divergence between component types becomes hard to detect.

By formally naming what Helm already does, this SUP provides the foundation that SUP-01 and SUP-02 build on. Any future component type can demonstrate conformance by filling in the Pattern Compliance Table in this document.

## Requirements alignment acknowledgement

The Helm Way pattern is grounded entirely in capabilities already present in the Margo pre-draft specification:

- **OCI Application Registry:** The completed SUP [sup_app_registry_as_oci](https://github.com/margo/specification-enhancements/blob/main/completed/sup_app_registry_as_oci.md) established the OCI Distribution Specification as the normative registry interface. The Component Registry that The Helm Way uses is the same OCI model.
- **Feature [margo/specification #89](https://github.com/margo/specification/issues/89):** Closed. Established the OCI registry strategy this pattern names.
- **Feature [margo/specification #91](https://github.com/margo/specification/issues/91):** Closed. Established the standard approach for workload manifests, which this pattern formalises for component packaging.

**No new capabilities are introduced.** All five elements of The Helm Way are already present in the pre-draft specification. This SUP names and documents an existing pattern; it does not introduce any requirements outside Margo's current scope.

**Out of scope:** Artifact signing and supply-chain attestation are explicitly out of scope. The OCI content-addressable digest provides transport integrity. Signing policy belongs in a dedicated Margo security SUP.

## Technical proposal

### The Pattern

"The Helm Way" is a five-element pattern for storing, referencing, identifying, verifying, and parameterizing component artifacts within the Margo ecosystem. Any component type that follows all five elements is said to conform to The Helm Way.

#### 1. Storage

The component artifact MUST be stored in an OCI-compliant [Component Registry](https://specification.margo.org/app-interoperability/application-registry/). The registry MUST support OCI artifacts as defined in the OCI Image Specification v1.1.0. The publishing tool (e.g., `helm push`, `oras push`) MUST produce a valid OCI image manifest.

#### 2. Reference

The ApplicationDescription MUST identify the component using two fields:

- `repository` — an `oci://` URI pointing to the registry and repository path (e.g., `oci://registry.example.com/org/component-name`)
- `revision` — an OCI tag identifying the component version (e.g., `1.0.0`, `2.3.1`) (SemVer 2.0 version without leading `v`)

Together, `repository` + `revision` provide a deterministic, human-readable coordinate for the artifact. No other location field (e.g., a plain-HTTPS URL) is required.

#### 3. Media Types

The OCI image manifest MUST declare a component-type-specific `artifactType` so that registries and tooling can distinguish Margo component artifacts from other OCI content. The layer blob containing the component payload MUST declare a component-type-specific `mediaType` so that consumers can verify they are processing the correct binary format.

Media type strings MUST follow the `application/vnd.org.margo.<hierarchy>` naming convention and MUST be registered in the Margo-Specific Media Types table in `application-registry.md`.

#### 4. Integrity

Integrity verification is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md), which the Margo Application Registry already requires. Implementations MUST verify the digest after pulling and before extracting or processing the artifact. No separate `digest` or hash field is needed in the ApplicationDescription.

#### 5. Parametrization

Each component type SHOULD define how ApplicationDescription `Parameter` values are delivered to the running component at deployment time. The delivery mechanism is component-type-specific and declared in the component type SUP.

Each new component type SUP SHOULD specify:
- The **parameter artifact** — the file or mechanism by which resolved parameter values reach the component (e.g., a values override file, an env file, a configuration file).
- The **`pointer` syntax** — the convention used in `Target.pointer` to identify the parameter's location within the artifact (e.g., dot-notation for a YAML tree, a named environment variable).

A single `Parameter` MAY declare targets with different pointer syntaxes, allowing the same logical value to be delivered to components of different types simultaneously.

---

### Helm as the Reference Implementation

Helm already satisfies all five elements of The Helm Way:

- **Storage**: `helm push` stores Helm charts in any OCI-compliant registry.
- **Reference**: The ApplicationDescription uses `repository: oci://...` and `revision: <tag>` to identify the chart.
- **Media types**: The OCI manifest uses `artifactType: application/vnd.cncf.helm.config.v1+json` and the layer blob uses `mediaType: application/vnd.cncf.helm.chart.content.layer.v1.tar+gzip`, both community-standard CNCF media types already registered.
- **Integrity**: The OCI content-addressable digest on the chart layer blob provides integrity verification per the OCI Distribution Specification v1.1.0.
- **Parametrization**: `helm install` accepts a `values.yaml` override file (via `-f values.yaml`) or individual `--set` flags. The ApplicationDescription `Target.pointer` uses dot-notation to identify the target key within the values tree (e.g., `config.database.host`).

Helm's compliance with this pattern is inherent — no specification changes are required for Helm components. The Helm Way simply names what Helm already does.

---

### Pattern Compliance Table

Any new component type SUP MUST demonstrate compliance with all five elements. The table below shows the reference implementation (Helm) alongside the two component types introduced by SUP-01 and SUP-02.

| Element | Requirement | Helm (reference) | Compose (SUP-01) | Quadlet (SUP-02) |
|---------|------------|-----------------|----------------|----------------|
| Registry | OCI-compliant, MUST | `helm push` | `oras push` | `oras push` |
| `type` value | versioned string, MUST | `helm` | `compose.v1` | `quadlet.v1` |
| `repository` field | `oci://` URI, MUST | `oci://registry/org/chart` | `oci://registry/org/app` | `oci://registry/org/app` |
| `revision` field | OCI tag, MUST | `1.0.0` | `1.0.0` | `1.0.0` |
| `artifactType` | Margo-registered, MUST | `application/vnd.cncf.helm.config.v1+json` | `application/vnd.org.margo.component.compose.v1+json` | `application/vnd.org.margo.component.quadlet.v1+json` |
| Layer `mediaType` | Margo-registered, MUST | `application/vnd.cncf.helm.chart.content.layer.v1.tar+gzip` | `application/vnd.org.margo.component.compose.v1.tar.gzip` | `application/vnd.org.margo.component.quadlet.v1.tar.gzip` |
| Integrity | OCI digest per OCI Distribution Spec v1.1.0 | Verified by registry and Helm CLI | Verified by registry and WFM | Verified by registry and WFM |
| Parameter artifact | SHOULD be defined | `values.yaml` override | `margo-params.env` file | `margo-params.env` file |

---

### Relationship to SUP-01 and SUP-02

This SUP is informational only. It does not modify any normative specification file.

The normative changes that apply The Helm Way to Compose components are defined in **SUP-01** (Compose OCI Registry Publishing and Archive Structure). The normative changes that apply The Helm Way to Quadlet components are defined in **SUP-02** (Quadlet Component Type).

Future component type SUPs SHOULD reference this document and fill in the Pattern Compliance Table to demonstrate conformance with The Helm Way.

---

### References

- [OCI Image Specification v1.1.0](https://github.com/opencontainers/image-spec/blob/v1.1.0/manifest.md)
- [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md)
- [Helm OCI Support](https://helm.sh/docs/topics/registries/)
- [ORAS (OCI Registry as Storage)](https://oras.land/)
- [RFC 6838 — Media Type Specifications and Registration Procedures](https://www.rfc-editor.org/rfc/rfc6838)
- [Completed SUP: Application Registry as OCI Registry](https://github.com/margo/specification-enhancements/blob/main/completed/sup_app_registry_as_oci.md)

## Alternatives considered

**Option A — Embed the pattern definition inside SUP-01 or SUP-02.** Rejected. Embedding the pattern in a normative SUP makes it impossible to reference it independently. If SUP-01 is rejected for Compose-specific reasons, the pattern definition would be lost. A standalone informational document is more durable.

**Option B — Tie the vote on The Helm Way to the vote on SUP-01 or SUP-02.** Rejected. The pattern is useful beyond Compose and Quadlet (any future component type can use it). Tying its acceptance to a specific component type vote introduces an artificial dependency and risks the pattern being rejected for the wrong reasons.

**Option C — No formal pattern definition; rely on informal convention.** Rejected. Without a named pattern, reviewers of future component type SUPs have no single checklist to apply. Divergence between component types becomes difficult to detect and correct.

---

*Prepared by Andrii Melashchenko (Belden Inc.), 2026-06-01. Subject to the Open Web Foundation Contributor License Agreement governing the Margo specification.*
