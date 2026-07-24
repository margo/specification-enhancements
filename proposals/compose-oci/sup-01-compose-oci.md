# Compose OCI Registry Publishing and Archive Structure

## Owner

[@javatask](https://github.com/javatask) — Andrii Melashchenko, Belden Inc.

## Summary

This SUP applies The Helm Way pattern (SUP-00) to Compose components. It mandates OCI registry storage for Compose Archives, defines two Margo-specific OCI media types, specifies the normative internal archive structure (single top-level directory, `compose.yaml` required, security constraints), uses the `compose` deployment profile type, removes the `packageLocation` field (which has never appeared in a released specification), and resolves three open "Investigation Needed" blocks in the current specification.

These changes are inseparable: what you push to the registry is defined by the archive structure, so the OCI mandate and the structure specification belong in a single SUP. The result is a complete, interoperable Compose packaging contract — something the current single-sentence description does not provide.

## Reason for proposal

The current specification describes the Compose Archive as "a tarball file containing the `compose.yaml` file and any additional artifacts referenced by the Compose file." This single-sentence description is insufficient for interoperable implementations. Five specific gaps exist:

1. **No OCI registry mandate** — Compose Archives can be served from any URL; there is no integrity guarantee.
2. **No integrity mechanism** — The spec recommends PGP signing but provides no checkable contract.
3. **`artifactType` inconsistency** — The Mermaid diagram in `application-registry.md` uses a different string than what implementations use.
4. **No `artifactType` or layer `mediaType`** — The spec does not define OCI media types for Compose components, preventing tooling from distinguishing Compose artifacts from other OCI content.
5. **Undefined archive structure** — No normative rules exist for directory layout, required files, or security constraints (path traversal, symlink escapes).

Without resolving these gaps, two implementations of the Margo Compose packaging model can produce non-interoperable results while both claiming compliance.

## Requirements alignment acknowledgement

This SUP addresses the following open and recently-closed specification issues:

- **[margo/specification #168](https://github.com/margo/specification/issues/168)** — "Define improvements to compose manifest workloads when targeting typical targets (Docker/Podman)" (open). This SUP defines the normative archive structure and OCI publishing contract that underpins any compose manifest improvement.
- **[margo/specification #166](https://github.com/margo/specification/issues/166)** — "Define the standard mechanism for compose enabled devices to authenticate and pull Compose Archive" (open). This SUP mandates OCI registry storage, which brings Compose Archives under the same OCI authentication model as application packages (OAuth 2.0 / Bearer token per the existing `application-registry.md` Authentication section).
- **[margo/specification #179](https://github.com/margo/specification/issues/179)** — "Fix/packagefile location for compose applications" (closed). This SUP removes `packageLocation` entirely and replaces it with `repository` + `revision` OCI coordinates. Since the specification is pre-draft, no migration path is needed.

**Out of scope:**
- Artifact signing and supply-chain attestation — deferred to a dedicated Margo security SUP.

## Technical proposal

### Affected files

| File | Change type |
|---|---|
| `system-design/concepts/applications/application-package.md` | Normative update (cross-reference callout) |
| `system-design/specification/applications/application-registry.md` | New media types + Compose Archive Structure section |
| `src/specification/applications/application-description.linkml.yaml` | Schema update (`revision` pattern) |
| `src/specification/margo-management-interface/desired-state.linkml.yaml` | Schema update (`revision` attribute + pattern) |

---

### Change 1: `application-package.md` — OCI mandate and Investigation Needed resolutions

#### 1a: Replace Compose bullet with OCI-mandated language

**Before** (lines 46–54):

```markdown
- To target devices, which deploy workloads using Kubernetes, components need to be defined as Helm charts using [Helm (version 3)](https://helm.sh/docs/topics/charts/).
- To target devices, which deploy workloads using [Compose](https://www.compose-spec.io/), components need to be packaged as [Compose Archives](../../personas-and-definitions/technical-lexicon.md#compose-archive), i.e., a tarball file containing the `compose.yaml` file and any additional artifacts referenced by the Compose file (e.g., configuration files, environment variable files, etc.). Margo recommends to digitally sign this package and to specify the location of the public key in the `ApplicationDescription` (see `keyLocation` [here](../../specification/applications/application-description.md#componentproperties-attributes)). When digitally signing the package PGP encryption MUST be used.

If either one cannot be implemented it MAY be omitted but Margo RECOMMENDS defining [deployment profiles](../../specification/applications/application-description.md#deploymentprofile-attributes) as both Helm chart **AND** Compose components to strengthen interoperability and applicability.

> **Note**
> A device running the application will only install the application using either the Compose Archives or the Helm Charts, but not both.
```

**After**:

```markdown
- To target devices which deploy workloads using Kubernetes, components MUST be defined as Helm charts using [Helm (version 3)](https://helm.sh/docs/topics/charts/). Helm charts MUST be stored in an OCI-compliant [Component Registry](../../personas-and-definitions/technical-lexicon.md#component-registry) and referenced via `repository` (an `oci://` URI) and `revision` (an OCI tag) in the [ApplicationDescription](../../specification/applications/application-description.md#componentproperties-attributes).
- To target devices which deploy workloads using [Compose](https://www.compose-spec.io/), components MUST be packaged as [Compose Archives](../../personas-and-definitions/technical-lexicon.md#compose-archive). Compose Archives MUST be stored in an OCI-compliant [Component Registry](../../personas-and-definitions/technical-lexicon.md#component-registry) and referenced via `repository` (an `oci://` URI) and `revision` (an OCI tag) in the [ApplicationDescription](../../specification/applications/application-description.md#componentproperties-attributes).

> **Normative requirement**: The internal structure of a Compose Archive is
> normatively defined in
> [Compose Archive Structure](../../specification/applications/application-registry.md#compose-archive-structure).
> Implementations MUST conform to that definition.

> **Note on publishing tools**: Use `oras push` (CNCF project) to publish Margo
> Compose Archives to a Component Registry. `docker compose publish`
> (Docker Compose 2.34.0+) is NOT compatible with the Margo Compose Archive
> format: it produces a different OCI artifact structure using
> `artifactType: application/vnd.docker.compose.project` with multiple
> individual layers per file and SHA256-hashed file paths, which cannot be
> consumed by a Margo-compliant WFM or device implementation expecting a single
> `tar+gzip` layer blob with
> `mediaType: application/vnd.org.margo.component.compose.tar+gzip`.

If any one component type cannot be implemented it MAY be omitted, but Margo RECOMMENDS defining [deployment profiles](../../specification/applications/application-description.md#deploymentprofile-attributes) using multiple component types (Helm, Compose, and/or Quadlet) to strengthen interoperability and applicability across heterogeneous edge device fleets.

> **Note**
> A device running the application will only install the application using one component type (Helm Charts, Compose Archives, or Quadlet Archives), never more than one simultaneously.
```

#### 1b: Remove "Investigation Needed" blocks

The three "Investigation Needed" blocks in the current specification MUST be **removed entirely** — not replaced with "Resolved" callouts. The specification text after this SUP is applied simply omits them. The resolutions are recorded here in the SUP for reviewer context only:

- **SHA256 hash / secure registries**: Resolved by OCI registry mandate — integrity is provided by OCI content-addressable digest; authentication follows the mechanisms in the [Authentication, Authorization & Security](../../specification/applications/application-registry.md#authentication-authorization--security) section.
- **3rd-party Helm charts**: Using 3rd-party Helm charts does not affect Margo compliance. The application developer is responsible for ensuring the chart functions on Margo-compliant devices.
- **Application dependencies / infrastructure services**: Out of scope for this SUP. The Investigation Needed block is simply removed.

---

### Change 2: `application-registry.md` — Add Compose media types and archive structure

This change adds two new media type rows to the existing Margo-Specific Media Types table and adds the normative Compose Archive Structure section (moved from `application-package.md`).

**Addition to Margo-Specific Media Types table:**

| Media Type | Description |
|-----------|-------------|
| `application/vnd.org.margo.component.compose+json` | `artifactType` for Compose Archive OCI manifest |
| `application/vnd.org.margo.component.compose.tar+gzip` | `mediaType` for Compose Archive layer blob |

**New section: Compose Archive Structure** (added after existing Helm content):

```markdown
#### Compose Archive Structure

A Compose Archive is a gzip-compressed tar archive (`.tar.gz` or `.tgz`) that packages a Compose application for deployment on edge devices. The archive MUST conform to the following structural requirements.

##### Directory Layout

The archive MUST contain exactly one top-level directory.

The directory name SHOULD match the component `name` as specified in the ApplicationDescription for human readability, but implementations MUST NOT depend on the directory name for discovery.

Discovery algorithm: enter the single top-level directory; locate the file named `compose.yaml`.

The top-level directory MAY contain any number of subdirectories (e.g., `configs/`, `certs/`, `scripts/`). All referenced files MUST resolve within the top-level directory.

The top-level directory MUST contain a file named `compose.yaml`. The Compose file MUST conform to the Compose Specification as currently published.

> **Note:** The Compose file MUST be named `compose.yaml`. The alternative names `compose.yml`, `docker-compose.yaml`, and `docker-compose.yml` are NOT valid within a Margo Compose Archive.

Files referenced by `compose.yaml` via `env_file` entries and `configs` (file source) MUST be included within the archive and MUST be referenced using relative paths that resolve within the top-level directory.

Bind-mount volume paths declared in `volumes` are runtime paths and MUST NOT be included in the archive.

Files for `secrets` (file source) MUST NOT be included in the archive. Secret provisioning is out of scope for this proposal and is the responsibility of the device or WFM implementation at deployment time.

##### Security Constraints

- Symlinks MUST NOT target paths outside the top-level directory.
- Hard links MUST NOT reference paths outside the top-level directory.
- Absolute paths MUST NOT appear in the archive entries.
- File names MUST NOT contain path traversal sequences (`../`).
- Implementations SHOULD normalize file permissions during archive extraction. Implementations MUST NOT preserve setuid, setgid, or sticky bits from archive entries.
- WFM and device implementations MUST validate these constraints before extracting or deploying the archive.

##### Integrity Verification

When stored in an OCI-compliant Component Registry, the Compose Archive tarball is the content of a single layer blob. Integrity verification at the transport layer is provided by the OCI content-addressable digest as mandated by the OCI Distribution Specification v1.1.0. Implementations MUST verify the OCI digest after pulling the blob and before extracting the archive.
```

---

### Change 3: `application-description.linkml.yaml` — Schema updates

#### 3a: `ComponentProperties` schema — `repository`/`revision` required, `packageLocation` removed, `revision` pattern added

```yaml
  ComponentProperties:
    description: Properties dictionary for component deployment details.
    rank: 80
    attributes:
      repository:
        description: >-
          OCI repository URI for the component (e.g., oci://registry.example.com/org/component-name).
          MUST be used for Helm, Compose, and Quadlet components.
        rank: 10
        range: string
        required: true
      revision:
        description: >-
          OCI tag identifying the component version within the repository
          (e.g., "1.0.0", "2.3.1", "1.0.0-rc1", "1.0.0_build.123").
          MUST be a valid SemVer 2.0 string without a leading `v`.
          SemVer build metadata separator `+` MUST be stored as `_`
          because `+` is not a valid OCI tag character.
          Implementations comparing this value against a SemVer string
          MUST convert `_` back to `+` before comparison.
        range: string
        required: true
        rank: 20
        pattern: "^(0|[1-9]\\d*)\\.(0|[1-9]\\d*)\\.(0|[1-9]\\d*)(?:-((?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\\.(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:_([0-9a-zA-Z-]+(?:\\.[0-9a-zA-Z-]+)*))?$"
      wait:
        description: If True, indicates the device waits for the component installation to complete.
        rank: 30
        range: boolean
      timeout:
        description: Time to wait for component installation to complete, formatted as "##m##s".
        rank: 40
        range: string
```

#### 3b: `ComposeDeploymentProfile` keeps existing `"compose"` type value

The existing `ComposeDeploymentProfile` class and its `equals_string: "compose"` are unchanged. The `pre-draft` specification already uses the unversioned `"compose"` value.

```yaml
  ComposeDeploymentProfile:
    is_a: DeploymentProfile
    slot_usage:
      type:
        equals_string: "compose"
        rank: 10
      components:
        range: ComposeComponent
        rank: 20
```

#### 3c: `type` slot — no regex change needed

The existing `type` slot already uses `pattern: ^(helm|compose)$`. No change is required.

#### 3d: `desired-state.linkml.yaml` — Add `revision` attribute to `Component` class

```yaml
  Component:
    description: A class representing a component of a deployment profile.
    rank: 50
    attributes:
      name:
        description: The name of the component.
        required: true
        rank: 10
      revision:
        description: >-
          OCI tag identifying the component version within the repository
          (e.g., "1.0.0", "2.3.1", "1.0.0-rc1", "1.0.0_build.123").
          MUST be a valid SemVer 2.0 string without a leading `v`.
          SemVer build metadata separator `+` MUST be stored as `_`
          because `+` is not a valid OCI tag character.
        range: string
        required: false
        rank: 15
        pattern: "^(0|[1-9]\\d*)\\.(0|[1-9]\\d*)\\.(0|[1-9]\\d*)(?:-((?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\\.(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:_([0-9a-zA-Z-]+(?:\\.[0-9a-zA-Z-]+)*))?$"
      properties:
        description: Properties associated with the component.
        range: Property
        required: true
        multivalued: true
        inlined: true
        inlined_as_list: false
        rank: 20
```

---

### Change 4: Compose Archive Structure normative text in `application-registry.md`

The full normative Compose Archive Structure is defined in Change 2 above (placed in `application-registry.md`). The `application-package.md` file retains only a cross-reference callout pointing to that section.

### Publishing Workflow (INFORMATIVE)

To publish a Compose Archive to an OCI-compliant Component Registry, use `oras push` (CNCF project). This is the RECOMMENDED publishing tool for Margo Compose Archives.

> **Warning**: `docker compose publish` (Docker Compose 2.34.0+) MUST NOT be
> used to publish Margo Compose components. It produces a structurally
> incompatible OCI artifact: `artifactType: application/vnd.docker.compose.project`,
> multiple layers (one per file), and SHA256-hashed file paths. This format
> cannot be consumed by a Margo-compliant WFM or device implementation.

Example:

```bash
oras push registry.example.com/org/myapp:1.0.0 \
  --artifact-type application/vnd.org.margo.component.compose+json \
  myapp-1.0.0-compose.tar.gz:application/vnd.org.margo.component.compose.tar+gzip
```

Reference the artifact in the ApplicationDescription:

```yaml
components:
  - name: myapp
    properties:
      repository: oci://registry.example.com/org/myapp
      revision: "1.0.0"
```

### WFM Reconciliation and `wait` Semantics

If `wait` is set to `true` for a Compose component, the device MUST wait until all containers in the Compose project reach **running** state before reporting the deployment as successful. This is equivalent to `docker compose up` or `podman-compose up` completing synchronously without `--detach`.

If any container exits with a non-zero exit code during startup, the deployment MUST be reported as failed immediately.

If health checks are defined in `compose.yaml`, implementations SHOULD additionally wait for all containers to reach **healthy** state before reporting success.

---

### Conformance impact

| RFC 2119 Keyword | Statement |
|---|---|
| MUST | Compose Archives MUST be stored in an OCI-compliant Component Registry and referenced via `repository` + `revision`. |
| MUST | The `repository` and `revision` fields in `ComponentProperties` MUST be present for all component types. |
| MUST | The OCI image manifest for a Compose component MUST use `artifactType` = `application/vnd.org.margo.component.compose+json`. |
| MUST | The layer blob mediaType for Compose MUST be `application/vnd.org.margo.component.compose.tar+gzip`. |
| MUST | A Compose Archive MUST contain exactly one top-level directory. |
| SHOULD | The directory name SHOULD match the component `name` for human readability. |
| MUST NOT | Implementations MUST NOT depend on the directory name for discovery. |
| MUST | A Compose Archive MUST contain a `compose.yaml` file in the top-level directory. |
| MUST | Files referenced via `env_file` and `configs` (file source) MUST be included within the archive using relative paths. |
| MUST NOT | Bind-mount volume paths MUST NOT be included in the archive. |
| MUST NOT | Files for `secrets` (file source) MUST NOT be included in the archive. |
| MUST NOT | Symlinks MUST NOT target paths outside the top-level directory. |
| MUST NOT | Absolute paths MUST NOT appear in archive entries. |
| MUST NOT | File names MUST NOT contain path traversal sequences. |
| MUST NOT | Implementations MUST NOT preserve setuid, setgid, or sticky bits from archive entries. |
| MUST | WFM and device implementations MUST validate security constraints before extraction. |
| MUST | Implementations MUST verify the OCI digest after pulling and before extracting. |
| MUST | The deployment profile type discriminator for Compose MUST be `compose`. |
| MUST | If `wait` is `true`, the device MUST wait until all containers reach running state. |
| MUST | If any container exits non-zero during startup, the deployment MUST be reported as failed. |
| SHOULD | If health checks are defined, implementations SHOULD wait for healthy state. |
| RECOMMENDED | `oras push` is the RECOMMENDED publishing tool for Margo Compose Archives. |

---

### Backward compatibility

This SUP targets the pre-draft specification. No backward compatibility constraints apply. The `packageLocation` field has never appeared in a released specification and is therefore removed without a deprecation period.

---

### References

- [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md)
- [OCI Image Specification v1.1.0](https://github.com/opencontainers/image-spec/blob/v1.1.0/manifest.md)
- [RFC 2119 — Key words for use in RFCs to Indicate Requirement Levels](https://www.rfc-editor.org/rfc/rfc2119)
- [Compose Specification](https://www.compose-spec.io/)
- [ORAS (OCI Registry as Storage)](https://oras.land/)
- SUP-00: The Helm Way (companion informational document)

## Alternatives considered

**Option A — Use plain-HTTPS URLs (no OCI mandate).** Rejected. Plain-HTTPS URLs provide no integrity guarantee. Any party who can intercept or modify the HTTP response can substitute a different archive, and the WFM has no way to detect the substitution. OCI content-addressable digests make this attack structurally impossible.

**Option B — Require a separate SHA-256 `digest` field in ApplicationDescription.** Rejected. This was the original "Investigation Needed" question. The OCI content-addressable digest on the layer blob makes an explicit `digest` field in the ApplicationDescription redundant. Adding a second integrity mechanism creates a reconciliation problem (which digest wins?) and adds implementation burden with no security benefit.

**Option C — Allow any top-level structure in the Compose Archive (no single-directory requirement).** Rejected. Without a predictable directory structure, WFM and device implementations must implement content discovery heuristics (search for `compose.yaml`, `docker-compose.yaml`, etc.). The single-top-level-directory requirement, combined with the `compose.yaml` naming requirement, eliminates this ambiguity at zero implementation cost to well-structured archives.

---

*Prepared by Andrii Melashchenko (Belden Inc.), 2026-06-01. Subject to the Open Web Foundation Contributor License Agreement governing the Margo specification.*
