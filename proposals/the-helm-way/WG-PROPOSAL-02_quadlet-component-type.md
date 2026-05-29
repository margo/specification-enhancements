# WG-PROPOSAL-02: Quadlet Component Type

| Field | Value |
|---|---|
| Date | 2026-05-04 |
| Category | Cat 2 -- normative enhancement |
| Affects | `system-design/concepts/applications/application-package.md`, `system-design/specification/applications/application-registry.md`, `src/specification/applications/application-description.linkml.yaml` |
| Status | Rev 1 -- ready for public review |
| Depends on | WG-PROPOSAL-00 (informational), WG-PROPOSAL-01 (normative, MUST be applied first) |

---

## Motivation

Quadlet is a **key deliverable** for the Margo edge workload model. It provides native systemd integration for container workloads without requiring a full Kubernetes control plane — exactly the lightweight, daemon-free deployment model needed on constrained OT edge devices. Quadlet (Podman 5.0+) is production-ready and already deployed in industrial environments.

This proposal applies The Helm Way pattern (WG-PROPOSAL-00) to Quadlet components. It MUST be applied after WG-PROPOSAL-01, which establishes the OCI registry mandate and Compose support that this proposal extends to Quadlet.

Quadlet -- Podman's native systemd integration format -- is not addressed by the current Margo specification, despite growing adoption in lightweight edge deployments where a full Kubernetes control plane is unnecessary. Quadlet allows containers to be managed as native systemd services, providing:

- Automatic container restart and dependency management via systemd
- Integration with existing systemd-based monitoring and logging infrastructure
- Rootless container operation without a daemon process
- Lightweight footprint suitable for constrained edge devices

This proposal adds Quadlet as a third component type alongside Helm and Compose, following the same OCI-based publishing pattern.

---

## Proposed Changes

### Change 1: `system-design/concepts/applications/application-package.md`

#### Change 1a: Add Quadlet bullet to component type list

This change adds the Quadlet component bullet to the existing component type list. It MUST be applied after WG-PROPOSAL-01's Change 1b, which rewrites the Helm and Compose bullets.

**New bullet (inserted after Compose bullet)**

```markdown
- To target devices which deploy workloads using [Quadlet](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html), components MUST be packaged as Quadlet Archives (see [Quadlet Archive Structure](#quadlet-archive-structure) below) and stored in an OCI-compliant [Component Registry](../../personas-and-definitions/technical-lexicon.md#component-registry). The Quadlet Archive MUST be pushed to the registry as an OCI artifact and referenced via `repository` (an `oci://` URI) and `revision` (an OCI tag) in the [ApplicationDescription](../../specification/applications/application-description.md#componentproperties-attributes). The OCI image manifest for the artifact MUST use `application/vnd.org.margo.component.quadlet.v1+json` as its `artifactType`, and the layer blob containing the tarball MUST use `application/vnd.org.margo.component.quadlet.v1.tar.gzip` as its `mediaType`. Integrity is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md), which the Margo Application Registry already requires. The target device MUST have Podman 5.0 or later installed to support Quadlet deployment.
```

**Diff** (applied after WG-PROPOSAL-01's Change 1b)

```diff
 - To target devices which deploy workloads using [Compose](https://www.compose-spec.io/), components MUST be packaged as [Compose Archives](...) ...
+- To target devices which deploy workloads using [Quadlet](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html), components MUST be packaged as Quadlet Archives (see [Quadlet Archive Structure](#quadlet-archive-structure) below) and stored in an OCI-compliant [Component Registry](../../personas-and-definitions/technical-lexicon.md#component-registry). The Quadlet Archive MUST be pushed to the registry as an OCI artifact and referenced via `repository` (an `oci://` URI) and `revision` (an OCI tag) in the [ApplicationDescription](../../specification/applications/application-description.md#componentproperties-attributes). The OCI image manifest for the artifact MUST use `application/vnd.org.margo.component.quadlet.v1+json` as its `artifactType`, and the layer blob containing the tarball MUST use `application/vnd.org.margo.component.quadlet.v1.tar.gzip` as its `mediaType`. Integrity is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md), which the Margo Application Registry already requires. The target device MUST have Podman 5.0 or later installed to support Quadlet deployment.
```

---

### Change 2: `system-design/specification/applications/application-registry.md`

#### Change 2a: Add Quadlet media types to Component Registry Media Types table

These two rows MUST be added to the Component Registry Media Types subtable introduced by WG-PROPOSAL-01 Change 2a.

**New rows**

```markdown
|``application/vnd.org.margo.component.quadlet.v1+json``| MUST be used as the **artifactType** to mark an OCI image manifest as a Margo Quadlet Archive component in a Component Registry |
|``application/vnd.org.margo.component.quadlet.v1.tar.gzip``| MUST be used as the **mediaType** for the layer blob containing the Quadlet Archive tarball (.tar.gz) |
```

**Diff** (appended to the Component Registry Media Types table from WG-PROPOSAL-01)

```diff
 |``application/vnd.org.margo.component.compose.v1+json``| MUST be used as the **artifactType** to mark an OCI image manifest as a Margo Compose Archive component in a Component Registry |
 |``application/vnd.org.margo.component.compose.v1.tar.gzip``| MUST be used as the **mediaType** for the layer blob containing the Compose Archive tarball (.tar.gz) |
+|``application/vnd.org.margo.component.quadlet.v1+json``| MUST be used as the **artifactType** to mark an OCI image manifest as a Margo Quadlet Archive component in a Component Registry |
+|``application/vnd.org.margo.component.quadlet.v1.tar.gzip``| MUST be used as the **mediaType** for the layer blob containing the Quadlet Archive tarball (.tar.gz) |
```

---

### Change 3: `src/specification/applications/application-description.linkml.yaml`

#### Change 3a: Add `QuadletDeploymentProfile` and `QuadletComponent` classes

**Before** (after `ComposeDeploymentProfile` and `ComposeComponent`)

```yaml
  ComposeDeploymentProfile:
    is_a: DeploymentProfile
    #rank: 66
    slot_usage:
      type:
        equals_string: "compose.v1"
        rank: 10
      components:
        range: ComposeComponent
        rank: 20

  # ... Component class ...

  ComposeComponent:
    is_a: Component
    #rank: 76
```

> **Note:** The Before block reflects the state after WG-PROPOSAL-01 has been applied, which renames the `type` value from `"compose"` (legacy) to `"compose.v1"`.

**After**

```yaml
  ComposeDeploymentProfile:
    is_a: DeploymentProfile
    #rank: 66
    slot_usage:
      type:
        equals_string: "compose.v1"
        rank: 10
      components:
        range: ComposeComponent
        rank: 20

  QuadletDeploymentProfile:
    is_a: DeploymentProfile
    description: >-
      Deployment profile for Quadlet-based workloads targeting devices running
      Podman 5.0 or later with systemd integration.
    #rank: 67
    slot_usage:
      type:
        equals_string: "quadlet.v1"
        rank: 10
      components:
        range: QuadletComponent
        rank: 20

  # ... Component class ...

  ComposeComponent:
    is_a: Component
    #rank: 76

  QuadletComponent:
    is_a: Component
    description: >-
      A component packaged as a Quadlet Archive for deployment on devices
      using Podman with systemd integration. The archive is stored in an
      OCI-compliant Component Registry and referenced via repository and
      revision. The OCI image manifest for this component type uses
      artifactType application/vnd.org.margo.component.quadlet.v1+json
      (see application-registry.md).
    #rank: 77
```

**Diff**

```diff
   ComposeDeploymentProfile:
     is_a: DeploymentProfile
     #rank: 66
     slot_usage:
       type:
         equals_string: "compose.v1"
         rank: 10
       components:
         range: ComposeComponent
         rank: 20

+  QuadletDeploymentProfile:
+    is_a: DeploymentProfile
+    description: >-
+      Deployment profile for Quadlet-based workloads targeting devices running
+      Podman 5.0 or later with systemd integration.
+    #rank: 67
+    slot_usage:
+      type:
+        equals_string: "quadlet.v1"
+        rank: 10
+      components:
+        range: QuadletComponent
+        rank: 20
+
   Component:
     description: A class representing a component of a deployment profile.
     ...

   ComposeComponent:
     is_a: Component
     #rank: 76
-  
+
+  QuadletComponent:
+    is_a: Component
+    description: >-
+      A component packaged as a Quadlet Archive for deployment on devices
+      using Podman with systemd integration. The archive is stored in an
+      OCI-compliant Component Registry and referenced via repository and
+      revision. The OCI image manifest for this component type uses
+      artifactType application/vnd.org.margo.component.quadlet.v1+json
+      (see application-registry.md).
+    #rank: 77
```

---

#### Change 3b: Add `quadlet.v1` to `type` slot pattern

> **Note:** WG-PROPOSAL-01 Change 3c replaces the `type` slot description with a bullet-list format and already includes `quadlet.v1`. Therefore, this change only updates the regex pattern. No description change is needed here; P-01 Change 3c is sufficient.

**Before** (after WG-PROPOSAL-01 Change 3c has been applied)

```yaml
  type:
    description: >-
      The deployment profile type discriminator. Allowed values:
      - `helm.v3`: Helm-based component (see HelmDeploymentProfile).
      - `compose.v1`: Compose-based component using OCI registry publishing (see ComposeV1DeploymentProfile). Preferred for new documents.
      - `compose`: Compose-based component (see ComposeDeploymentProfile). Deprecated -- use `compose.v1` for new documents.
      - `quadlet.v1`: Quadlet-based component (see QuadletDeploymentProfile, introduced by WG-PROPOSAL-02).
    rank: 10
    range: string
    required: true
    pattern: ^(helm\.v3|compose|compose\.v1)$
```

**After**

```yaml
  type:
    description: >-
      The deployment profile type discriminator. Allowed values:
      - `helm.v3`: Helm-based component (see HelmDeploymentProfile).
      - `compose.v1`: Compose-based component using OCI registry publishing (see ComposeV1DeploymentProfile). Preferred for new documents.
      - `compose`: Compose-based component (see ComposeDeploymentProfile). Deprecated -- use `compose.v1` for new documents.
      - `quadlet.v1`: Quadlet-based component (see QuadletDeploymentProfile, introduced by WG-PROPOSAL-02).
    rank: 10
    range: string
    required: true
    pattern: ^(helm\.v3|compose|compose\.v1|quadlet\.v1)$
```

**Diff**

```diff
   type:
     description: >-
       The deployment profile type discriminator. Allowed values:
       ...
       - `quadlet.v1`: Quadlet-based component (see QuadletDeploymentProfile, introduced by WG-PROPOSAL-02).
     rank: 10
     range: string
     required: true
-    pattern: ^(helm\.v3|compose|compose\.v1)$
+    pattern: ^(helm\.v3|compose|compose\.v1|quadlet\.v1)$
```

---

### Change 4: Quadlet Archive Structure (new section in `application-package.md`)

This section defines the normative archive structure for `quadlet.v1` components, complementing the Compose Archive Structure defined in WG-PROPOSAL-01. It defines the normative internal structure of Quadlet Archives.

**New content (addition)**

````markdown
## Quadlet Archive Structure

A Quadlet Archive is a gzip-compressed tar archive (`.tar.gz` or `.tgz`) that packages a set of [Podman Quadlet](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html) unit files for deployment on edge devices running Podman 5.0 or later with systemd integration. The archive MUST conform to the following structural requirements.

### Directory Layout

The archive MUST contain exactly one top-level directory. The name of this directory MUST match the component `name` as specified in the [ApplicationDescription](../../specification/applications/application-description.md#component-attributes).

The top-level directory MAY contain Quadlet unit files according to [Podman Quadlet](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html).

### Security Constraints

- Symlinks MUST NOT target paths outside the top-level directory.
- Hard links MUST NOT reference paths outside the top-level directory.
- Absolute paths MUST NOT appear in the archive entries.
- File names MUST NOT contain path traversal sequences (`../`).
- Implementations SHOULD normalize file permissions during archive extraction. Implementations MUST NOT preserve setuid, setgid, or sticky bits from archive entries.
- WFM and device implementations MUST validate these constraints before extracting or deploying the archive.

### Example

```tree

myapp-1.0.0-quadlet.tgz
+-- myapp/
    +-- myapp.container
    +-- myapp.network
    +-- myapp.volume

```

In this example:

- `myapp` is the component name matching the ApplicationDescription.
- `myapp.container` is the primary Quadlet container unit.
- `myapp.network` defines a Podman network for the container.
- `myapp.volume` defines a Podman volume for persistent storage.

### Multi-Container Quadlet Example

For applications requiring multiple containers:

```tree

my-platform-1.0.0-quadlet.tgz
+-- my-platform/
    +-- frontend.container
    +-- backend.container
    +-- database.container
    +-- app.network
    +-- db-data.volume
    +-- config/
        +-- backend.env

```

### Integrity Verification

When stored in an OCI-compliant Component Registry (as mandated by this specification), the Quadlet Archive tarball is the content of a single layer blob. Integrity verification at the transport layer is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md), which the Margo Application Registry already requires.

Implementations MUST verify the OCI digest after pulling the blob and before extracting the archive.

### Publishing Workflow

To publish a Quadlet Archive to an OCI-compliant Component Registry:

1. Create the `.tar.gz` archive conforming to the directory layout above.
2. Push the archive as an OCI artifact:

   ```bash
   oras push registry.example.com/org/myapp:1.0.0 \
     --artifact-type application/vnd.org.margo.component.quadlet.v1+json \
     myapp-1.0.0-quadlet.tar.gz:application/vnd.org.margo.component.quadlet.v1.tar.gzip
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

## Companion Artifact

A companion patch (`PATCH_desired-state-quadlet.diff`) is provided with this proposal package to apply the required desired-state schema changes. That patch adds `QuadletDeploymentProfile` and `QuadletComponent` to `desired-state.linkml.yaml` and MUST be applied before Quadlet workloads can be deployed via the Margo Management Interface.

---

## Conformance Impact

This proposal introduces the following normative statements:

| RFC 2119 Keyword | Statement |
|---|---|
| MUST | Quadlet Archives MUST be stored in an OCI-compliant Component Registry and referenced via `repository` + `revision`. |
| MUST | The OCI image manifest for a Quadlet component MUST use `artifactType` = `application/vnd.org.margo.component.quadlet.v1+json`. |
| MUST | The layer blob mediaType for Quadlet MUST be `application/vnd.org.margo.component.quadlet.v1.tar.gzip`. |
| MUST | A Quadlet Archive MUST contain exactly one top-level directory whose name matches the component `name`. |
| MUST | The target device MUST have Podman 5.0 or later installed to support Quadlet deployment. |
| MUST NOT | Symlinks MUST NOT target paths outside the top-level directory. |
| MUST NOT | Hard links MUST NOT reference paths outside the top-level directory. |
| MUST NOT | Absolute paths MUST NOT appear in archive entries. |
| MUST NOT | File names MUST NOT contain path traversal sequences. |
| MUST NOT | Implementations MUST NOT preserve setuid, setgid, or sticky bits from archive entries. |
| MUST | WFM and device implementations MUST validate security constraints before extraction. |
| MUST | Implementations MUST verify the OCI digest after pulling and before extracting. |
| SHOULD | Implementations SHOULD normalize file permissions during archive extraction. |

---

## Backward Compatibility

### Breaking Changes

This proposal is **additive only**. It does not modify any existing normative text or schema constraint that would break existing implementations.

- The `type` regex expansion from `^(helm\.v3|compose|compose\.v1)$` to `^(helm\.v3|compose|compose\.v1|quadlet\.v1)$` is additive -- existing documents with `type: helm.v3`, `type: compose`, or `type: compose.v1` continue to validate.
- The new `QuadletDeploymentProfile` and `QuadletComponent` classes are additions to the schema class hierarchy -- no existing class is modified.
- The new Quadlet media types are additions to the media type table -- no existing media type is changed.
- The Quadlet Archive Structure section is entirely new normative text.

**Breaking change: NO** (when considered independently of WG-PROPOSAL-01).

### Migration Path

No migration is needed for existing implementations. Quadlet is a greenfield addition:

- Existing Helm and Compose ApplicationDescription documents are unaffected.
- Existing WFM implementations that do not support Quadlet will ignore `type: quadlet.v1` deployment profiles per the existing spec behavior (unknown types are skipped).
- Edge devices without Podman 5.0+ will not receive Quadlet deployment profiles.

---

## References

- [Podman Quadlet Documentation](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html)
- [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md)
- [OCI Image Specification v1.1.0](https://github.com/opencontainers/image-spec/blob/v1.1.0/manifest.md)
- [RFC 2119 -- Key words for use in RFCs to Indicate Requirement Levels](https://www.rfc-editor.org/rfc/rfc2119)
- [ORAS (OCI Registry as Storage)](https://oras.land/)
- WG-PROPOSAL-00: The Helm Way (companion informational document)
- WG-PROPOSAL-01: Compose OCI Registry Publishing and Archive Structure (prerequisite normative document)
- PATCH_desired-state-quadlet.diff (companion schema patch)

---

*This document is part of the Margo WG Compose/Quadlet OCI Publishing proposal package. Prepared by the Andrii Melashchenko, 2026-05-04. Subject to the Open Web Foundation Contributor License Agreement governing the Margo specification.*
