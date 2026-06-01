# Quadlet Component Type (Podman 5.0 / systemd)

## Owner

[@javatask](https://github.com/javatask) — Andrii Melashchenko, Belden Inc.

## Summary

This SUP adds Quadlet as a third Margo component type alongside Helm and Compose. Quadlet is Podman's native systemd integration format for container workloads — it allows containers to be managed as native systemd services without requiring a Kubernetes control plane. This is the lightweight, daemon-free deployment model required on constrained OT edge devices.

The SUP applies The Helm Way pattern (SUP-00) to Quadlet, following the same OCI registry publishing model established by SUP-01 for Compose. It defines the `quadlet.v1` deployment profile type, two Margo-registered OCI media types, `QuadletDeploymentProfile` and `QuadletComponent` schema classes, the normative Quadlet Archive Structure, and a companion patch for the desired-state schema. This SUP is **additive only** — no existing Helm or Compose functionality is changed.

**Prerequisite:** This SUP MUST be applied after SUP-01 (Compose OCI Registry Publishing), which establishes the OCI mandate and Component Registry media type table that this SUP extends.

## Reason for proposal

Quadlet (Podman 5.0+) provides:
- Automatic container restart and dependency management via systemd
- Integration with existing systemd-based monitoring and logging infrastructure (journald)
- Rootless container operation without a daemon process
- Lightweight footprint suitable for constrained OT edge devices

The current Margo specification has no path for systemd-managed container workloads. Edge devices running Podman 5.0+ with systemd cannot be managed by a Margo-compliant WFM today. This gap is significant: many industrial OT edge devices run a full Linux distribution with systemd but cannot justify the resource overhead of a Kubernetes control plane.

Quadlet is production-ready and already deployed in industrial environments. Adding it as a first-class Margo component type closes the gap between the Margo application model and the reality of OT edge device deployments.

## Requirements alignment acknowledgement

This SUP is grounded in the following in-scope features and issues:

- **[margo/specification-enhancements #60](https://github.com/margo/specification-enhancements/issues/60)** — "Mandate daemonless container runtime as the default single edge container runtime for Margo-compliant edge devices" (open). Quadlet/Podman is the reference daemonless container runtime. This SUP provides the Margo packaging contract for Quadlet workloads on such devices.
- **[margo/specification #168](https://github.com/margo/specification/issues/168)** — "Define improvements to compose manifest workloads when targeting typical targets (Docker/Podman)" (open). Quadlet complements Compose for Podman-based targets: Compose targets Docker-compatible runtimes; Quadlet targets systemd-integrated Podman deployments where lifecycle management (restart, dependency ordering, journald logging) is required.

**Out of scope (explicitly stated):**
- Quadlet unit file *content* (systemd unit semantics, network/volume definitions, socket activation) is governed by Podman documentation and is outside Margo's normative scope. Margo specifies only the packaging and registry contract.
- Device provisioning of Podman 5.0+ is outside Margo's scope. The device MUST have Podman 5.0+ installed; this is a deployment prerequisite, not a WFM responsibility.
- Signing and supply-chain attestation are deferred to a dedicated Margo security SUP.

**Dependency:** This SUP depends on SUP-01 being accepted and applied. The Component Registry Media Types table introduced by SUP-01 is extended here. The `type` slot regex introduced by SUP-01 is expanded here.

## Technical proposal

### Affected files

| File | Change type |
|---|---|
| `system-design/concepts/applications/application-package.md` | Normative update + new section |
| `system-design/specification/applications/application-registry.md` | Table addition (extends SUP-01 table) |
| `src/specification/applications/application-description.linkml.yaml` | Schema addition |
| `src/specification/margo-management-interface/desired-state.linkml.yaml` | Schema addition (companion patch) |

---

### Change 1: `application-package.md` — Add Quadlet bullet

Insert after the Compose bullet (added by SUP-01):

```markdown
- To target devices which deploy workloads using [Quadlet](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html), components MUST be packaged as Quadlet Archives (see [Quadlet Archive Structure](#quadlet-archive-structure) below) and stored in an OCI-compliant [Component Registry](../../personas-and-definitions/technical-lexicon.md#component-registry). The Quadlet Archive MUST be pushed to the registry as an OCI artifact and referenced via `repository` (an `oci://` URI) and `revision` (an OCI tag) in the [ApplicationDescription](../../specification/applications/application-description.md#componentproperties-attributes). The OCI image manifest for the artifact MUST use `application/vnd.org.margo.component.quadlet.v1+json` as its `artifactType`, and the layer blob containing the tarball MUST use `application/vnd.org.margo.component.quadlet.v1.tar.gzip` as its `mediaType`. Integrity is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md), which the Margo Application Registry already requires. The target device MUST have Podman 5.0 or later installed to support Quadlet deployment.
```

---

### Change 2: `application-registry.md` — Add Quadlet media types

Append to the Component Registry Media Types table introduced by SUP-01:

```markdown
|``application/vnd.org.margo.component.quadlet.v1+json``| MUST be used as the **artifactType** to mark an OCI image manifest as a Margo Quadlet Archive component in a Component Registry |
|``application/vnd.org.margo.component.quadlet.v1.tar.gzip``| MUST be used as the **mediaType** for the layer blob containing the Quadlet Archive tarball (.tar.gz) |
```

---

### Change 3: `application-description.linkml.yaml` — Add schema classes

#### 3a: Add `QuadletDeploymentProfile` and `QuadletComponent`

After `ComposeV1DeploymentProfile` (added by SUP-01):

```yaml
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
```

After `ComposeComponent`:

```yaml
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

#### 3b: Expand `type` slot regex

The `type` slot pattern introduced by SUP-01 is:
```yaml
    pattern: ^(helm\.v3|compose|compose\.v1)$
```

Update to:
```yaml
    pattern: ^(helm\.v3|compose|compose\.v1|quadlet\.v1)$
```

> **Note:** SUP-01 Change 3c already includes `quadlet.v1` in the `type` slot description text. Only the regex requires expansion here.

---

### Change 4: New section `## Quadlet Archive Structure` in `application-package.md`

Add after the Compose Archive Structure section (added by SUP-01):

````markdown
## Quadlet Archive Structure

A Quadlet Archive is a gzip-compressed tar archive (`.tar.gz` or `.tgz`) that packages a set of [Podman Quadlet](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html) unit files for deployment on edge devices running Podman 5.0 or later with systemd integration. The archive MUST conform to the following structural requirements.

### Directory Layout

The archive MUST contain exactly one top-level directory. The name of this directory MUST match the component `name` as specified in the ApplicationDescription.

The top-level directory MAY contain Quadlet unit files according to the [Podman Quadlet specification](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html). Supported unit file extensions include `.container`, `.network`, `.volume`, `.image`, `.build`, and `.pod`.

### Security Constraints

- Symlinks MUST NOT target paths outside the top-level directory.
- Hard links MUST NOT reference paths outside the top-level directory.
- Absolute paths MUST NOT appear in the archive entries.
- File names MUST NOT contain path traversal sequences (`../`).
- Implementations SHOULD normalize file permissions during archive extraction. Implementations MUST NOT preserve setuid, setgid, or sticky bits from archive entries.
- WFM and device implementations MUST validate these constraints before extracting or deploying the archive.

### Single-Container Example

```tree
myapp-1.0.0-quadlet.tgz
+-- myapp/
    +-- myapp.container
    +-- myapp.network
    +-- myapp.volume
```

- `myapp` is the component name matching the ApplicationDescription.
- `myapp.container` is the primary Quadlet container unit defining the container image, environment, and mounts.
- `myapp.network` defines a Podman network for the container.
- `myapp.volume` defines a Podman volume for persistent storage.

### Multi-Container Example

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

When stored in an OCI-compliant Component Registry, the Quadlet Archive tarball is the content of a single layer blob. Integrity verification at the transport layer is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md). Implementations MUST verify the OCI digest after pulling the blob and before extracting the archive.

### Publishing Workflow

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

### Change 5: Companion patch — `desired-state.linkml.yaml`

A companion patch file (`PATCH_desired-state-quadlet.diff`) is provided alongside this SUP. It MUST be applied together with this SUP's changes to `application-description.linkml.yaml`.

The patch adds `QuadletDeploymentProfile` and `QuadletComponent` to `desired-state.linkml.yaml` and updates the `type` slot description. Without this patch, WFMs cannot generate schema-valid `ApplicationDeployment` desired-state documents for Quadlet workloads.

```diff
--- a/src/specification/margo-management-interface/desired-state.linkml.yaml
+++ b/src/specification/margo-management-interface/desired-state.linkml.yaml
@@ -130,6 +130,19 @@ classes:
   ComposeDeploymentProfile:
     is_a: DeploymentProfile
     rank: 40
     slot_usage:
       type:
         equals_string: "compose"
         rank: 10
       components:
         range: ComposeComponent
         rank: 20

+  QuadletDeploymentProfile:
+    is_a: DeploymentProfile
+    description: >-
+      Deployment profile for Quadlet-based workloads targeting devices running
+      Podman 5.0 or later with systemd integration.
+    rank: 40
+    slot_usage:
+      type:
+        equals_string: "quadlet"
+        rank: 10
+      components:
+        range: QuadletComponent
+        rank: 20
+
   Component:
     ...

   ComposeComponent:
     is_a: Component
     rank: 50

+  QuadletComponent:
+    is_a: Component
+    rank: 50
+    description: >-
+      A component packaged as a Quadlet Archive for deployment on devices using
+      Podman with systemd integration.
+

   type:
-    description: The type of deployment profile (e.g., helm.v3, compose).
+    description: >-
+      The type of deployment profile. Allowed values are: helm.v3 (Kubernetes/Helm
+      chart deployment), compose (Compose file deployment via Podman or Docker
+      Compose), and quadlet (Podman systemd Quadlet unit file deployment). The
+      value MUST match the type field in the corresponding ApplicationDescription
+      deploymentProfile element.
     range: string
     required: true
     rank: 10
```

---

### Conformance impact

| RFC 2119 Keyword | Statement |
|---|---|
| MUST | Quadlet Archives MUST be stored in an OCI-compliant Component Registry and referenced via `repository` + `revision`. |
| MUST | The OCI image manifest for a Quadlet component MUST use `artifactType` = `application/vnd.org.margo.component.quadlet.v1+json`. |
| MUST | The layer blob mediaType for Quadlet MUST be `application/vnd.org.margo.component.quadlet.v1.tar.gzip`. |
| MUST | A Quadlet Archive MUST contain exactly one top-level directory whose name matches the component `name`. |
| MUST | The target device MUST have Podman 5.0 or later installed. |
| MUST NOT | Symlinks MUST NOT target paths outside the top-level directory. |
| MUST NOT | Absolute paths MUST NOT appear in archive entries. |
| MUST NOT | File names MUST NOT contain path traversal sequences. |
| MUST NOT | Implementations MUST NOT preserve setuid, setgid, or sticky bits from archive entries. |
| MUST | WFM and device implementations MUST validate security constraints before extraction. |
| MUST | Implementations MUST verify the OCI digest after pulling and before extracting. |
| SHOULD | Implementations SHOULD normalize file permissions during archive extraction. |

---

### Backward compatibility

This SUP is **additive only**.

- The `type` regex expansion from `^(helm\.v3|compose|compose\.v1)$` to `^(helm\.v3|compose|compose\.v1|quadlet\.v1)$` is additive — existing documents continue to validate.
- `QuadletDeploymentProfile` and `QuadletComponent` are new schema classes — no existing class is modified.
- Existing WFM implementations that do not support Quadlet will ignore `type: quadlet.v1` deployment profiles per existing spec behavior.
- Edge devices without Podman 5.0+ will not receive Quadlet deployment profiles.

---

### References

- [Podman Quadlet Documentation](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html)
- [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md)
- [OCI Image Specification v1.1.0](https://github.com/opencontainers/image-spec/blob/v1.1.0/manifest.md)
- [RFC 2119 — Key words for use in RFCs to Indicate Requirement Levels](https://www.rfc-editor.org/rfc/rfc2119)
- [ORAS (OCI Registry as Storage)](https://oras.land/)
- SUP-00: The Helm Way (companion informational document)
- SUP-01: Compose OCI Registry Publishing and Archive Structure (prerequisite)
- `PATCH_desired-state-quadlet.diff` (companion schema patch, included in this PR)

## Alternatives considered

**Option A — Use plain Compose (with Podman as the runtime) instead of Quadlet.** Rejected. Compose on Podman provides container execution but not systemd lifecycle integration. Quadlet provides automatic restart on failure, dependency ordering between services (e.g., start database before application), journald log integration, and socket-activated services — all without a daemon process. These are essential properties for unattended OT edge deployments where human operators are not available to restart failed containers.

**Option B — Require Kubernetes (K3s or MicroK8s) on all OT edge devices.** Rejected. A minimal K3s installation requires ~500 MB RAM and a persistent control-plane process. Many constrained OT edge devices (industrial PLCs, embedded Linux controllers) have 256–512 MB RAM total and cannot run a Kubernetes control plane. Quadlet + Podman requires only Podman itself and systemd, which are already present on any modern Linux distribution used in industrial settings.

**Option C — Define Quadlet support as a device capability flag, without a new component type.** Rejected. A capability flag does not provide a checkable packaging contract. Application developers need to know exactly what archive structure, media types, and OCI publishing workflow to use. Without a dedicated component type, Quadlet workloads would be packaged as Compose Archives and rely on runtime detection — an unreliable approach that defeats the purpose of typed deployment profiles.

---

*Prepared by Andrii Melashchenko (Belden Inc.), 2026-06-01. Subject to the Open Web Foundation Contributor License Agreement governing the Margo specification.*
