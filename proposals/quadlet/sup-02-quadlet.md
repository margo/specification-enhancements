# Quadlet Component Type (systemd-integrated container workloads)

## Owner

[@javatask](https://github.com/javatask) — Andrii Melashchenko, Belden Inc.

## Summary

This SUP adds Quadlet as a third Margo component type alongside Helm and Compose. Quadlet is a systemd-native unit file format for declarative container workloads — it allows containers to be managed as native systemd services without requiring a Kubernetes control plane. This is the lightweight, daemon-free deployment model required on constrained OT edge devices.

The SUP applies The Helm Way pattern (SUP-00) to Quadlet, following the same OCI registry publishing model established by SUP-01 for Compose. It defines the `quadlet` deployment profile type, two Margo-registered OCI media types, `QuadletDeploymentProfile` and `QuadletComponent` schema classes, the normative Quadlet Archive Structure, and a companion patch for the desired-state schema. This SUP is **additive only** — no existing Helm or Compose functionality is changed.

**Prerequisite:** This SUP MUST be applied after SUP-01 (Compose OCI Registry Publishing), which establishes the OCI mandate and Component Registry media type table that this SUP extends.

## Reason for proposal

[Quadlet](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html) provides:
- Automatic container restart and dependency management via systemd
- Integration with existing systemd-based monitoring and logging infrastructure (journald)
- Rootless container operation without a daemon process
- Lightweight footprint suitable for constrained OT edge devices

The current Margo specification has no path for systemd-managed container workloads. Edge devices capable of running Quadlet unit files cannot be managed by a Margo-compliant WFM today. This gap is significant: many industrial OT edge devices run a full Linux distribution with systemd but cannot justify the resource overhead of a Kubernetes control plane.

Quadlet is production-ready and already deployed in industrial environments. Adding it as a first-class Margo component type closes the gap between the Margo application model and the reality of OT edge device deployments.

## Requirements alignment acknowledgement

> **Note:** A TWG Feature Project feature for Quadlet deployment type support must be created before this SUP advances to Phase 3 approval vote. The SUP owner will coordinate with the TWG Chair/Co-Chair to create this feature.

**Out of scope (explicitly stated):**
- Quadlet unit file *content* (systemd unit semantics, network/volume definitions, socket activation) is governed by the [Quadlet unit file specification](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html) and is outside Margo's normative scope. Margo specifies only the packaging and registry contract.
- Device provisioning of a Quadlet-capable runtime is outside Margo's scope. The device MUST be capable of processing Quadlet unit files; this is a deployment prerequisite, not a WFM responsibility.
- Signing and supply-chain attestation are deferred to a dedicated Margo security SUP.
- Namespacing and collision avoidance for Quadlet unit files is out of scope. This is a general Margo gap: Helm lacks normative namespace/release-name rules ([specification#183](https://github.com/margo/specification/issues/183)), and Compose lacks project-name rules ([specification#142](https://github.com/margo/specification/issues/142)).
- Device capabilities integration: for the sake of isolation between SUPs, this proposal does not prescribe changes to the device capabilities manifest. By default, standalone devices use Docker Compose; operators can manually select the Quadlet deployment profile when deploying to Quadlet-capable devices. Automated WFM targeting via `supportedDeploymentTypes` is a follow-up coordination item.

**Dependency:** This SUP depends on SUP-01 being accepted and applied. The Component Registry Media Types table introduced by SUP-01 is extended here. The `type` slot regex introduced by SUP-01 is expanded here.

## Technical proposal

### Affected files

| File | Change type |
|---|---|
| `system-design/concepts/applications/application-package.md` | Non-normative cross-reference |
| `system-design/specification/applications/application-registry.md` | Table addition + Quadlet Archive Structure section |
| `src/specification/applications/application-description.linkml.yaml` | Schema addition |
| `src/specification/margo-management-interface/desired-state.linkml.yaml` | Schema addition (companion patch) |

---

### Change 1: `application-package.md` — Add Quadlet cross-reference bullet

Insert after the Compose bullet (added by SUP-01). This bullet is non-normative — all specification language is in `application-registry.md`:

```markdown
- To target devices which deploy workloads using [Quadlet](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html), components are packaged as Quadlet Archives and stored in an OCI-compliant [Component Registry](../../personas-and-definitions/technical-lexicon.md#component-registry). See [Quadlet Archive Structure](../../specification/applications/application-registry.md#quadlet-archive-structure) for the normative definition. The target device must be capable of processing Quadlet unit files (e.g., via Podman with systemd integration).
```

---

### Change 2: `application-registry.md` — Add Quadlet media types and archive structure

#### 2a: Media types table addition

Append to the Component Registry Media Types table introduced by SUP-01:

| Media Type | Description |
|-----------|-------------|
| `application/vnd.org.margo.component.quadlet+json` | `artifactType` for Quadlet Archive OCI manifest |
| `application/vnd.org.margo.component.quadlet.tar+gzip` | `mediaType` for Quadlet Archive layer blob |

#### 2b: New section — Quadlet Archive Structure

Add after the Compose Archive Structure section (added by SUP-01):

```markdown
#### Quadlet Archive Structure

A Quadlet Archive is a gzip-compressed tar archive (`.tar.gz` or `.tgz`) that packages a set of [Quadlet unit files](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html) for deployment on edge devices capable of processing Quadlet units via systemd integration. The archive MUST conform to the following structural requirements.

##### Directory Layout

The archive MUST contain exactly one top-level directory.

The directory name SHOULD match the component `name` as specified in the ApplicationDescription for human readability, but implementations MUST NOT depend on the directory name for discovery.

Discovery algorithm: enter the single top-level directory; locate Quadlet unit files by extension (`.container`, `.network`, `.volume`, `.image`, `.build`, `.pod`).

The top-level directory MAY contain Quadlet unit files according to the [Quadlet unit file specification](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html). The archive MUST contain at least one `.container` file.

The top-level directory MAY contain subdirectories for supporting files (e.g., environment files, configuration).

> **RECOMMENDED:** To reduce collision risk when multiple Quadlet components are deployed to the same device, implementations SHOULD prefix unit file names with the application ID or component name (e.g., `myapp-frontend.container` rather than `frontend.container`).

##### Security Constraints

- Symlinks MUST NOT target paths outside the top-level directory.
- Hard links MUST NOT reference paths outside the top-level directory.
- Absolute paths MUST NOT appear in the archive entries.
- File names MUST NOT contain path traversal sequences (`../`).
- Implementations SHOULD normalize file permissions during archive extraction. Implementations MUST NOT preserve setuid, setgid, or sticky bits from archive entries.
- WFM and device implementations MUST validate these constraints before extracting or deploying the archive.

##### Single-Container Example

```
myapp-1.0.0-quadlet.tgz
└── myapp/
    ├── myapp.container
    ├── myapp.network
    └── myapp.volume
```

##### Multi-Container Example

```
my-platform-1.0.0-quadlet.tgz
└── my-platform/
    ├── frontend.container
    ├── backend.container
    ├── database.container
    ├── app.network
    ├── db-data.volume
    └── config/
        └── backend.env
```

##### Integrity Verification

When stored in an OCI-compliant Component Registry, the Quadlet Archive tarball is the content of a single layer blob. Integrity verification at the transport layer is provided by the OCI content-addressable digest as mandated by the OCI Distribution Specification v1.1.0. Implementations MUST verify the OCI digest after pulling the blob and before extracting the archive.
```

---

### Change 3: `application-description.linkml.yaml` — Add schema classes

#### 3a: Add `QuadletDeploymentProfile` and `QuadletComponent`

After `ComposeDeploymentProfile`:

```yaml
  QuadletDeploymentProfile:
    is_a: DeploymentProfile
    description: >-
      Deployment profile for Quadlet-based workloads targeting devices capable
      of processing Quadlet unit files via systemd integration.
    rank: 45
    slot_usage:
      type:
        equals_string: "quadlet"
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
      capable of processing Quadlet unit files via systemd integration.
      The archive is stored in an OCI-compliant Component Registry and
      referenced via repository and revision. The OCI image manifest for
      this component type uses artifactType
      application/vnd.org.margo.component.quadlet+json
      (see application-registry.md).
```

#### 3b: Expand `type` slot regex

The current `type` slot pattern in `application-description.linkml.yaml` (after the Helm SUP) is:
```yaml
    pattern: ^(helm|compose)$
```

Update to:
```yaml
    pattern: ^(helm|compose|quadlet)$
```

> **Note:** This pattern constraint exists in `application-description.linkml.yaml` only. The `desired-state.linkml.yaml` schema relies on per-class `equals_string` constraints (see Change 5 / companion patch) rather than a top-level regex.

---

### Change 4: WFM Reconciliation and `wait`/`timeout` Semantics for Quadlet

If `wait` is set to `true` for a Quadlet component, the device MUST wait until all systemd units generated from the Quadlet archive reach `active (running)` state as reported by systemd before reporting the deployment as successful.

If `timeout` is specified and any unit has not reached `active (running)` state within the specified duration, the deployment MUST be reported as failed.

If any unit enters `failed` state during activation, the deployment MUST be reported as failed immediately regardless of timeout.

If health checks are defined via systemd mechanisms (e.g., `Type=notify` readiness notification, `ExecStartPost=` probes), implementations SHOULD additionally wait for readiness confirmation before reporting success.

---

### Change 5: Companion patch — `desired-state.linkml.yaml`

A companion patch file (`PATCH_desired-state-quadlet.diff`) is provided alongside this SUP. It MUST be applied together with this SUP's changes to `application-description.linkml.yaml`.

The patch adds `QuadletDeploymentProfile` and `QuadletComponent` to `desired-state.linkml.yaml` and updates the `type` slot description. Without this patch, WFMs cannot generate schema-valid `ApplicationDeployment` desired-state documents for Quadlet workloads.

---

### Publishing Workflow (INFORMATIVE)

To publish a Quadlet Archive to an OCI-compliant Component Registry:

1. Create the `.tar.gz` archive conforming to the directory layout above.
2. Push the archive as an OCI artifact:

   ```bash
   oras push registry.example.com/org/myapp:1.0.0 \
     --artifact-type application/vnd.org.margo.component.quadlet+json \
     myapp-1.0.0-quadlet.tar.gz:application/vnd.org.margo.component.quadlet.tar+gzip
   ```

3. Reference the artifact in the ApplicationDescription:

   ```yaml
   components:
     - name: myapp
       properties:
         repository: oci://registry.example.com/org/myapp
         revision: "1.0.0"
   ```

---

### Conformance impact

| RFC 2119 Keyword | Statement |
|---|---|
| MUST | Quadlet Archives MUST be stored in an OCI-compliant Component Registry and referenced via `repository` + `revision`. |
| MUST | The OCI image manifest for a Quadlet component MUST use `artifactType` = `application/vnd.org.margo.component.quadlet+json`. |
| MUST | The layer blob mediaType for Quadlet MUST be `application/vnd.org.margo.component.quadlet.tar+gzip`. |
| MUST | A Quadlet Archive MUST contain exactly one top-level directory. |
| SHOULD | The directory name SHOULD match the component `name` for human readability. |
| MUST NOT | Implementations MUST NOT depend on the directory name for discovery. |
| MUST | A Quadlet Archive MUST contain at least one `.container` file. |
| MUST | The target device MUST be capable of processing Quadlet unit files. |
| MUST | The deployment profile type discriminator for Quadlet MUST be `quadlet`. |
| MUST | If `wait` is `true`, the device MUST wait until all Quadlet units reach `active (running)` state. |
| MUST | If any unit enters `failed` state, the deployment MUST be reported as failed immediately. |
| MUST | If `timeout` expires before units are active, the deployment MUST be reported as failed. |
| SHOULD | If systemd readiness mechanisms are defined, implementations SHOULD wait for readiness. |
| RECOMMENDED | Implementations SHOULD prefix unit file names with application/component name to reduce collision risk. |
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

- The `type` regex expansion from `^(helm|compose)$` to `^(helm|compose|quadlet)$` is additive — existing documents continue to validate.
- `QuadletDeploymentProfile` and `QuadletComponent` are new schema classes — no existing class is modified.
- Existing WFM implementations that do not support Quadlet will not receive Quadlet deployment profiles. The WFM selects deployment profiles based on the `type` value matching supported types.
- Edge devices not capable of processing Quadlet unit files will not receive Quadlet deployment profiles.

---

### References

- [Quadlet Unit File Specification (Podman)](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html)
- [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md)
- [OCI Image Specification v1.1.0](https://github.com/opencontainers/image-spec/blob/v1.1.0/manifest.md)
- [RFC 2119 — Key words for use in RFCs to Indicate Requirement Levels](https://www.rfc-editor.org/rfc/rfc2119)
- [ORAS (OCI Registry as Storage)](https://oras.land/)
- SUP-00: The Helm Way (companion informational document)
- SUP-01: Compose OCI Registry Publishing and Archive Structure (prerequisite)
- `PATCH_desired-state-quadlet.diff` (companion schema patch, included in this PR)

## Alternatives considered

**Option A — Use plain Compose (with Podman as the runtime) instead of Quadlet.** Rejected. Compose on Podman provides container execution but not systemd lifecycle integration. Quadlet provides automatic restart on failure, dependency ordering between services (e.g., start database before application), journald log integration, and socket-activated services — all without a daemon process. These are essential properties for unattended OT edge deployments where human operators are not available to restart failed containers.

**Option B — Require Kubernetes (K3s or MicroK8s) on all OT edge devices.** Rejected. A minimal K3s installation requires ~500 MB RAM and a persistent control-plane process. Many constrained OT edge devices (industrial PLCs, embedded Linux controllers) have 256–512 MB RAM total and cannot run a Kubernetes control plane. Quadlet requires only a Quadlet-capable container runtime and systemd, which are already present on modern Linux distributions used in industrial settings.

**Option C — Define Quadlet support as a device capability flag, without a new component type.** Rejected. A capability flag does not provide a checkable packaging contract. Application developers need to know exactly what archive structure, media types, and OCI publishing workflow to use. Without a dedicated component type, Quadlet workloads would be packaged as Compose Archives and rely on runtime detection — an unreliable approach that defeats the purpose of typed deployment profiles.

---

*Prepared by Andrii Melashchenko (Belden Inc.), 2026-06-01. Updated 2026-06-24 addressing TWG review comments. Subject to the Open Web Foundation Contributor License Agreement governing the Margo specification.*
