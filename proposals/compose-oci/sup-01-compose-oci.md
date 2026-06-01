# Compose OCI Registry Publishing and Archive Structure

## Owner

[@javatask](https://github.com/javatask) — Andrii Melashchenko, Belden Inc.

## Summary

This SUP applies The Helm Way pattern (SUP-00) to Compose components. It mandates OCI registry storage for Compose Archives, defines two Margo-specific OCI media types, specifies the normative internal archive structure (single top-level directory, `compose.yaml` required, security constraints), introduces the versioned `compose.v1` deployment profile type, removes the `packageLocation` field (which has never appeared in a released specification), and resolves three open "Investigation Needed" blocks in the current specification.

These changes are inseparable: what you push to the registry is defined by the archive structure, so the OCI mandate and the structure specification belong in a single SUP. The result is a complete, interoperable Compose packaging contract — something the current single-sentence description does not provide.

## Reason for proposal

The current specification describes the Compose Archive as "a tarball file containing the `compose.yaml` file and any additional artifacts referenced by the Compose file." This single-sentence description is insufficient for interoperable implementations. Five specific gaps exist:

1. **No OCI registry mandate** — Compose Archives can be served from any URL; there is no integrity guarantee.
2. **No integrity mechanism** — The spec recommends PGP signing but provides no checkable contract.
3. **`artifactType` inconsistency** — The Mermaid diagram in `application-registry.md` uses a different string than what implementations use.
4. **Bare type discriminator** — `"compose"` is not versioned, making future evolution impossible without a flag day.
5. **Undefined archive structure** — No normative rules exist for directory layout, required files, or security constraints (path traversal, symlink escapes).

Without resolving these gaps, two implementations of the Margo Compose packaging model can produce non-interoperable results while both claiming compliance.

## Requirements alignment acknowledgement

This SUP addresses the following open and recently-closed specification issues:

- **[margo/specification #168](https://github.com/margo/specification/issues/168)** — "Define improvements to compose manifest workloads when targeting typical targets (Docker/Podman)" (open). This SUP defines the normative archive structure and OCI publishing contract that underpins any compose manifest improvement.
- **[margo/specification #166](https://github.com/margo/specification/issues/166)** — "Define the standard mechanism for compose enabled devices to authenticate and pull Compose Archive" (open). This SUP mandates OCI registry storage, which brings Compose Archives under the same OCI authentication model as application packages (OAuth 2.0 / Bearer token per the existing `application-registry.md` Authentication section).
- **[margo/specification #179](https://github.com/margo/specification/issues/179)** — "Fix/packagefile location for compose applications" (closed). This SUP removes `packageLocation` entirely and replaces it with `repository` + `revision` OCI coordinates. Since the specification is pre-draft, no migration path is needed.

**Out of scope (explicitly deferred):**
- Artifact signing and supply-chain attestation — deferred to a dedicated Margo security SUP.
- Application dependencies and required infrastructure services (storage, message queues, reverse proxy) — deferred; requires coordination with the Margo Device Interface Working Group.

## Technical proposal

### Affected files

| File | Change type |
|---|---|
| `system-design/concepts/applications/application-package.md` | Normative update + new section |
| `system-design/specification/applications/application-registry.md` | Table addition |
| `src/specification/applications/application-description.linkml.yaml` | Schema update |

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
- To target devices which deploy workloads using [Compose](https://www.compose-spec.io/), the following requirements apply:
  1. Components MUST be packaged as [Compose Archives](../../personas-and-definitions/technical-lexicon.md#compose-archive) (see [Compose Archive Structure](#compose-archive-structure) below).
  2. Compose Archives MUST be stored in an OCI-compliant [Component Registry](../../personas-and-definitions/technical-lexicon.md#component-registry).
  3. The Compose Archive MUST be pushed to the registry as an OCI artifact (e.g., using `oras push` or equivalent tooling) and referenced via `repository` (an `oci://` URI) and `revision` (an OCI tag) in the [ApplicationDescription](../../specification/applications/application-description.md#componentproperties-attributes).
  4. The OCI image manifest for the artifact MUST use `application/vnd.org.margo.component.compose.v1+json` as its `artifactType`.
  5. The layer blob containing the tarball MUST use `application/vnd.org.margo.component.compose.v1.tar.gzip` as its `mediaType`.
  6. Integrity of the Compose Archive is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md), which the Margo Application Registry already requires.

If any one component type cannot be implemented it MAY be omitted, but Margo RECOMMENDS defining [deployment profiles](../../specification/applications/application-description.md#deploymentprofile-attributes) using multiple component types (Helm, Compose, and/or Quadlet) to strengthen interoperability and applicability across heterogeneous edge device fleets.

> **Note**
> A device running the application will only install the application using one component type (Helm Charts, Compose Archives, or Quadlet Archives), never more than one simultaneously.
```

#### 1b: Resolve three "Investigation Needed" blocks

**Before**:

```markdown
Margo will provide more detailed discussion and specification on the following points:

> **Investigation Needed**: Question: do we need to specify the location of a SHA256 hash for the Compose Archive also (similar to the PGP key) in the ApplicationDescription?
> We will also discuss how we should handle secure container registries that require a username and password.
>
> **Investigation Needed**: We need to determine what impact, if any, using 3rd party helm charts has on being Margo compliant.
>
> **Investigation Needed**: Missing in the current specification are ways to define dependencies (e.g., application dependencies) as well as required infrastructure services such as storage, message queues/bus, reverse proxy, or authentication/authorization/accounting.
```

**After**:

```markdown
> **Resolved (SUP-01)**: The integrity question is resolved by mandating OCI registry storage for all component types. Integrity verification is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md), which the Margo Application Registry already requires. No separate `digest` field is needed in the ApplicationDescription. Authentication for secure container registries SHOULD follow the mechanisms defined in the [Authentication, Authorization & Security](../../specification/applications/application-registry.md#authentication-authorization--security) section.

> **Resolved (SUP-01)**: Using 3rd-party Helm charts as components in an Application Package does not affect Margo compliance of the Application Package itself. Margo compliance is determined by the conformance of the ApplicationDescription and its packaging within the Application Registry. The 3rd-party chart need not be "Margo-aware"; however, the application developer is responsible for ensuring the chart functions correctly on Margo-compliant edge devices. The `pointer` field in Parameter Targets works with any Helm chart's `values.yaml` structure. Application developers SHOULD document known limitations of 3rd-party charts (e.g., charts requiring CRDs, admission webhooks, or cluster-scoped resources) in the deployment profile `description` field.

> **Deferred**: Defining application dependencies and required infrastructure services (storage, message queues, reverse proxy, authentication/authorization/accounting) is deferred to a separate SUP. This topic requires coordination with the Margo Device Interface working group.
```

---

### Change 2: `application-registry.md` — Add Compose media types

**Before** (lines 222–231):

```markdown
#### Margo-Specific Media Types

|Media Type|Description|
|----------|----------|
|``application/vnd.org.margo.app.v1+json`` | MUST be used as the **artifactType** to mark the OCI image manifest as the definition of a Margo Application Package |
|``application/vnd.org.margo.app.description.v1+yaml`` | MUST be used to mark a layer in the OCI image manifest as pointing to the Margo Application Description file |
|``application/vnd.org.margo.app.icon.v1+{file format}``| MUST be used to mark a layer in the OCI image manifest as pointing to the icon of a Margo Application Package |
|``application/vnd.org.margo.app.descriptionFile.v1+{file format}``| MUST be used to mark a layer in the OCI image manifest as pointing to description file of a Margo Application Package |
|``application/vnd.org.margo.app.licenseFile.v1+{file format}``| MUST be used to mark a layer in the OCI image manifest as pointing to the license file of a Margo Application Package|
|``application/vnd.org.margo.app.releaseNotes.v1+{file format}``| MUST be used to mark a layer in the OCI image manifest as pointing to the release notes file of a Margo Application Package|
```

**After**:

```markdown
#### Margo-Specific Media Types

##### Application Package Media Types

|Media Type|Description|
|----------|----------|
|``application/vnd.org.margo.app.v1+json`` | MUST be used as the **artifactType** to mark the OCI image manifest as the definition of a Margo Application Package |
|``application/vnd.org.margo.app.description.v1+yaml`` | MUST be used to mark a layer in the OCI image manifest as pointing to the Margo Application Description file |
|``application/vnd.org.margo.app.icon.v1+{file format}``| MUST be used to mark a layer in the OCI image manifest as pointing to the icon of a Margo Application Package |
|``application/vnd.org.margo.app.descriptionFile.v1+{file format}``| MUST be used to mark a layer in the OCI image manifest as pointing to description file of a Margo Application Package |
|``application/vnd.org.margo.app.licenseFile.v1+{file format}``| MUST be used to mark a layer in the OCI image manifest as pointing to the license file of a Margo Application Package|
|``application/vnd.org.margo.app.releaseNotes.v1+{file format}``| MUST be used to mark a layer in the OCI image manifest as pointing to the release notes file of a Margo Application Package|

##### Component Registry Media Types

|Media Type|Description|
|----------|----------|
|``application/vnd.org.margo.component.compose.v1+json``| MUST be used as the **artifactType** to mark an OCI image manifest as a Margo Compose Archive component in a Component Registry |
|``application/vnd.org.margo.component.compose.v1.tar.gzip``| MUST be used as the **mediaType** for the layer blob containing the Compose Archive tarball (.tar.gz) |
```

> **Note:** Quadlet media types (`application/vnd.org.margo.component.quadlet.v1+json` and `application/vnd.org.margo.component.quadlet.v1.tar.gzip`) are introduced in SUP-02 and MUST be added to this table when that SUP is applied.

---

### Change 3: `application-description.linkml.yaml` — Schema updates

#### 3a: `ComponentProperties` schema — `repository`/`revision` required, `packageLocation` removed

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
          OCI tag identifying the component version within the repository (e.g., "1.0.0", "2.3.1").
          SemVer 2.0 version without leading `v`.
          MUST be used for Helm, Compose, and Quadlet components.
        rank: 20
        range: string
        required: true
      wait:
        description: If True, indicates the device waits for the component installation to complete.
        rank: 30
        range: boolean
      timeout:
        description: Time to wait for component installation to complete, formatted as "##m##s".
        rank: 40
        range: string
```

#### 3b: Replace `ComposeDeploymentProfile` with `ComposeV1DeploymentProfile`

```yaml
  ComposeV1DeploymentProfile:
    is_a: DeploymentProfile
    #rank: 67
    slot_usage:
      type:
        equals_string: "compose.v1"
        rank: 10
      components:
        range: ComposeComponent
        rank: 20
```

> **Note:** The bare `"compose"` type value and its corresponding `ComposeDeploymentProfile` class are removed. Since the specification is pre-draft, no backward-compatible alias is needed. All Compose deployment profiles MUST use the versioned `"compose.v1"` type discriminator.

#### 3c: Update `type` slot description and regex

```yaml
  type:
    description: >-
      The deployment profile type discriminator. Allowed values:
      - `helm.v3`: Helm-based component (see HelmDeploymentProfile).
      - `compose.v1`: Compose-based component using OCI registry publishing (see ComposeV1DeploymentProfile).
      - `quadlet.v1`: Quadlet-based component (see QuadletDeploymentProfile, introduced by SUP-02).
    rank: 10
    range: string
    required: true
    pattern: ^(helm\.v3|compose\.v1|quadlet\.v1)$
```

---

### Change 4: New section `## Compose Archive Structure` in `application-package.md`

Add after the existing component type list:

````markdown
## Compose Archive Structure

A Compose Archive is a gzip-compressed tar archive (`.tar.gz` or `.tgz`) that packages a [Compose](https://www.compose-spec.io/) application for deployment on edge devices. The archive MUST conform to the following structural requirements.

### Directory Layout

The archive MUST contain exactly one top-level directory. The name of this directory MUST match the component `name` as specified in the ApplicationDescription.

The top-level directory MUST contain a file named `compose.yaml`. The Compose file MUST conform to the [Compose Specification](https://www.compose-spec.io/) as currently published.

> **Note:** The Compose file MUST be named `compose.yaml`. The alternative names `compose.yml`, `docker-compose.yaml`, and `docker-compose.yml` are NOT valid within a Margo Compose Archive. This restriction ensures predictable file discovery by WFM and device implementations.

All files referenced by `compose.yaml` (including but not limited to `env_file` entries, `volumes` bind-mount sources, `configs` file sources, and `secrets` file sources) MUST be included within the archive and MUST be referenced using relative paths that resolve within the top-level directory.

### Security Constraints

- Symlinks MUST NOT target paths outside the top-level directory.
- Hard links MUST NOT reference paths outside the top-level directory.
- Absolute paths MUST NOT appear in the archive entries.
- File names MUST NOT contain path traversal sequences (`../`).
- Implementations SHOULD normalize file permissions during archive extraction. Implementations MUST NOT preserve setuid, setgid, or sticky bits from archive entries.
- WFM and device implementations MUST validate these constraints before extracting or deploying the archive.

### Example

```tree
myapp-1.0.0-compose.tgz
+-- myapp/
    +-- compose.yaml
    +-- .env
    +-- config/
        +-- app.conf
```

- `myapp` is the component name matching the ApplicationDescription.
- `compose.yaml` is the required Compose file.
- `.env` is an environment variable file referenced by `env_file` in `compose.yaml`.
- `config/app.conf` is a configuration file referenced as a bind mount or config in `compose.yaml`.

### Integrity Verification

When stored in an OCI-compliant Component Registry, the Compose Archive tarball is the content of a single layer blob. Integrity verification at the transport layer is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md). Implementations MUST verify the OCI digest after pulling the blob and before extracting the archive.

### Publishing Workflow

1. Create the `.tar.gz` archive conforming to the directory layout above.
2. Push the archive as an OCI artifact:

   ```bash
   oras push registry.example.com/org/myapp:1.0.0 \
     --artifact-type application/vnd.org.margo.component.compose.v1+json \
     myapp-1.0.0-compose.tar.gz:application/vnd.org.margo.component.compose.v1.tar.gzip
   ```

3. Reference the artifact in the ApplicationDescription:

   ```yaml
   components:
     - name: myapp
       properties:
         repository: oci://registry.example.com/org/myapp
         revision: "1.0.0"
   ```
````

---

### Conformance impact

| RFC 2119 Keyword | Statement |
|---|---|
| MUST | Compose Archives MUST be stored in an OCI-compliant Component Registry and referenced via `repository` + `revision`. |
| MUST | The `repository` and `revision` fields in `ComponentProperties` MUST be present for all component types. |
| MUST | The OCI image manifest for a Compose component MUST use `artifactType` = `application/vnd.org.margo.component.compose.v1+json`. |
| MUST | The layer blob mediaType for Compose MUST be `application/vnd.org.margo.component.compose.v1.tar.gzip`. |
| MUST | A Compose Archive MUST contain exactly one top-level directory whose name matches the component `name`. |
| MUST | A Compose Archive MUST contain a `compose.yaml` file in the top-level directory. |
| MUST | All files referenced by `compose.yaml` MUST be included within the archive using relative paths. |
| MUST NOT | Symlinks MUST NOT target paths outside the top-level directory. |
| MUST NOT | Absolute paths MUST NOT appear in archive entries. |
| MUST NOT | File names MUST NOT contain path traversal sequences. |
| MUST NOT | Implementations MUST NOT preserve setuid, setgid, or sticky bits from archive entries. |
| MUST | WFM and device implementations MUST validate security constraints before extraction. |
| MUST | Implementations MUST verify the OCI digest after pulling and before extracting. |
| MUST | The deployment profile type discriminator for Compose MUST be `compose.v1`. |

---

### Backward compatibility

This SUP targets the pre-draft specification. No backward compatibility constraints apply. The `packageLocation` field and the bare `"compose"` type value have never appeared in a released specification and are therefore removed without a deprecation period.

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
