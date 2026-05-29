# WG-PROPOSAL-01: Compose OCI Registry Publishing and Archive Structure

| Field | Value |
|---|---|
| Date | 2026-05-04 |
| Category | Cat 2 -- normative enhancement |
| Affects | `system-design/concepts/applications/application-package.md`, `system-design/specification/applications/application-registry.md`, `src/specification/applications/application-description.linkml.yaml` |
| Status | Rev 1 -- ready for public review |

## Motivation

This proposal applies The Helm Way pattern (WG-PROPOSAL-00) to Compose components. The Helm Way establishes that all Margo component types should follow the OCI registry-based publishing model already implemented by Helm: store in an OCI registry, reference via `repository`+`revision`, declare component-type-specific media types, and verify integrity via OCI content-addressable digest. This proposal mandates OCI registry storage for Compose Archives, introduces Margo-specific OCI media types, defines the normative internal archive structure, deprecates the `packageLocation` field, and resolves the open "Investigation Needed" blocks. These changes are inseparable: what you push to the registry is defined by the archive structure, so the OCI mandate and the structure specification belong in a single document.

The current specification describes the Compose Archive as "a tarball file containing the `compose.yaml` file and any additional artifacts referenced by the Compose file." This single-sentence description is insufficient for interoperable implementations. Five specific gaps exist in the Compose packaging model: the OCI registry mandate is absent, the integrity mechanism is unspecified, an `artifactType` string inconsistency exists in the Mermaid diagram, the type discriminator uses a bare string instead of a versioned value, and the archive structure is not normatively defined.

---

## Proposed Changes

### Change 1: `system-design/concepts/applications/application-package.md`

---

#### Change 1a: Replace Compose bullet with OCI-mandated language

This change rewrites the Compose component bullet to mandate OCI registry storage following The Helm Way pattern (WG-PROPOSAL-00). The Helm bullet is also tightened with explicit OCI language. The Quadlet bullet is introduced separately in WG-PROPOSAL-02.

**Before** (lines 46-54)

```markdown
 The components are being deployed as workloads on the edge devices:

- To target devices, which deploy workloads using Kubernetes, components need to be defined as Helm charts using [Helm (version 3)](https://helm.sh/docs/topics/charts/).
- To target devices, which deploy workloads using [Compose](https://www.compose-spec.io/), components need to be packaged as [Compose Archives](../../personas-and-definitions/technical-lexicon.md#compose-archive), i.e., a tarball file containing the `compose.yaml` file and any additional artifacts referenced by the Compose file (e.g., configuration files, environment variable files, etc.). Margo recommends to digitally sign this package and to specify the location of the public key in the `ApplicationDescription` (see `keyLocation` [here](../../specification/applications/application-description.md#componentproperties-attributes)). When digitally signing the package PGP encryption MUST be used.

If either one cannot be implemented it MAY be omitted but Margo RECOMMENDS defining [deployment profiles](../../specification/applications/application-description.md#deploymentprofile-attributes) as both Helm chart **AND** Compose components to strengthen interoperability and applicability.

> **Note**
> A device running the application will only install the application using either the Compose Archives or the Helm Charts, but not both.
```

**After**

```markdown
 The components are being deployed as workloads on the edge devices:

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

**Diff**

```diff
  The components are being deployed as workloads on the edge devices:

-- To target devices, which deploy workloads using Kubernetes, components need to be defined as Helm charts using [Helm (version 3)](https://helm.sh/docs/topics/charts/).
-- To target devices, which deploy workloads using [Compose](https://www.compose-spec.io/), components need to be packaged as [Compose Archives](../../personas-and-definitions/technical-lexicon.md#compose-archive), i.e., a tarball file containing the `compose.yaml` file and any additional artifacts referenced by the Compose file (e.g., configuration files, environment variable files, etc.). Margo recommends to digitally sign this package and to specify the location of the public key in the `ApplicationDescription` (see `keyLocation` [here](../../specification/applications/application-description.md#componentproperties-attributes)). When digitally signing the package PGP encryption MUST be used.
+- To target devices which deploy workloads using Kubernetes, components MUST be defined as Helm charts using [Helm (version 3)](https://helm.sh/docs/topics/charts/). Helm charts MUST be stored in an OCI-compliant [Component Registry](../../personas-and-definitions/technical-lexicon.md#component-registry) and referenced via `repository` (an `oci://` URI) and `revision` (an OCI tag) in the [ApplicationDescription](../../specification/applications/application-description.md#componentproperties-attributes).
+- To target devices which deploy workloads using [Compose](https://www.compose-spec.io/), the following requirements apply:
+  1. Components MUST be packaged as [Compose Archives](../../personas-and-definitions/technical-lexicon.md#compose-archive) (see [Compose Archive Structure](#compose-archive-structure) below).
+  2. Compose Archives MUST be stored in an OCI-compliant [Component Registry](../../personas-and-definitions/technical-lexicon.md#component-registry).
+  3. The Compose Archive MUST be pushed to the registry as an OCI artifact (e.g., using `oras push` or equivalent tooling) and referenced via `repository` (an `oci://` URI) and `revision` (an OCI tag) in the [ApplicationDescription](../../specification/applications/application-description.md#componentproperties-attributes).
+  4. The OCI image manifest for the artifact MUST use `application/vnd.org.margo.component.compose.v1+json` as its `artifactType`.
+  5. The layer blob containing the tarball MUST use `application/vnd.org.margo.component.compose.v1.tar.gzip` as its `mediaType`.
+  6. Integrity of the Compose Archive is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md), which the Margo Application Registry already requires.

-If either one cannot be implemented it MAY be omitted but Margo RECOMMENDS defining [deployment profiles](../../specification/applications/application-description.md#deploymentprofile-attributes) as both Helm chart **AND** Compose components to strengthen interoperability and applicability.
+If any one component type cannot be implemented it MAY be omitted, but Margo RECOMMENDS defining [deployment profiles](../../specification/applications/application-description.md#deploymentprofile-attributes) using multiple component types (Helm, Compose, and/or Quadlet) to strengthen interoperability and applicability across heterogeneous edge device fleets.

 > **Note**
-> A device running the application will only install the application using either the Compose Archives or the Helm Charts, but not both.
+> A device running the application will only install the application using one component type (Helm Charts, Compose Archives, or Quadlet Archives), never more than one simultaneously.
```

---

#### Change 1b: Resolve three "Investigation Needed" blocks

**Before** (lines 57-64)

```markdown
Margo will provide more detailed discussion and specification on the following points:

> **Investigation Needed**: Question: do we need to specify the location of a SHA256 hash for the Compose Archive also (similar to the PGP key) in the ApplicationDescription?
> We will also discuss how we should handle secure container registries that require a username and password.
>
> **Investigation Needed**: We need to determine what impact, if any, using 3rd party helm charts has on being Margo compliant.
>
> **Investigation Needed**: Missing in the current specification are ways to define dependencies (e.g., application dependencies) as well as required infrastructure services such as storage, message queues/bus, reverse proxy, or authentication/authorization/accounting.
```

**After**

```markdown
> **Resolved (WG-PROPOSAL-01)**: The integrity question is resolved by mandating OCI registry storage for all component types. Integrity verification is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md), which the Margo Application Registry already requires. No separate `digest` field is needed in the ApplicationDescription. Authentication for secure container registries SHOULD follow the mechanisms defined in the [Authentication, Authorization & Security](../../specification/applications/application-registry.md#authentication-authorization--security) section.

> **Resolved (WG-PROPOSAL-01)**: Using 3rd-party Helm charts as components in an Application Package does not affect Margo compliance of the Application Package itself. Margo compliance is determined by the conformance of the ApplicationDescription and its packaging within the Application Registry. The 3rd-party chart need not be "Margo-aware"; however, the application developer is responsible for ensuring the chart functions correctly on Margo-compliant edge devices. The `pointer` field in Parameter Targets works with any Helm chart's `values.yaml` structure. Application developers SHOULD document known limitations of 3rd-party charts (e.g., charts requiring CRDs, admission webhooks, or cluster-scoped resources) in the deployment profile `description` field.

> **Deferred**: Defining application dependencies and required infrastructure services (storage, message queues, reverse proxy, authentication/authorization/accounting) is deferred to a separate proposal. This topic requires coordination with the Margo Device Interface working group.
```

**Diff**

```diff
-Margo will provide more detailed discussion and specification on the following points:
-
-> **Investigation Needed**: Question: do we need to specify the location of a SHA256 hash for the Compose Archive also (similar to the PGP key) in the ApplicationDescription?
-> We will also discuss how we should handle secure container registries that require a username and password.
->
-> **Investigation Needed**: We need to determine what impact, if any, using 3rd party helm charts has on being Margo compliant.
->
-> **Investigation Needed**: Missing in the current specification are ways to define dependencies (e.g., application dependencies) as well as required infrastructure services such as storage, message queues/bus, reverse proxy, or authentication/authorization/accounting.
+> **Resolved (WG-PROPOSAL-01)**: The integrity question is resolved by mandating OCI registry storage for all component types. Integrity verification is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md), which the Margo Application Registry already requires. No separate `digest` field is needed in the ApplicationDescription. Authentication for secure container registries SHOULD follow the mechanisms defined in the [Authentication, Authorization & Security](../../specification/applications/application-registry.md#authentication-authorization--security) section.
+
+> **Resolved (WG-PROPOSAL-01)**: Using 3rd-party Helm charts as components in an Application Package does not affect Margo compliance of the Application Package itself. Margo compliance is determined by the conformance of the ApplicationDescription and its packaging within the Application Registry. The 3rd-party chart need not be "Margo-aware"; however, the application developer is responsible for ensuring the chart functions correctly on Margo-compliant edge devices. The `pointer` field in Parameter Targets works with any Helm chart's `values.yaml` structure. Application developers SHOULD document known limitations of 3rd-party charts (e.g., charts requiring CRDs, admission webhooks, or cluster-scoped resources) in the deployment profile `description` field.
+
+> **Deferred**: Defining application dependencies and required infrastructure services (storage, message queues, reverse proxy, authentication/authorization/accounting) is deferred to a separate proposal. This topic requires coordination with the Margo Device Interface working group.
```

---

### Change 2: `system-design/specification/applications/application-registry.md`

#### Change 2a: Add Compose media types to Margo-Specific Media Types table

**Before** (lines 222-231)

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

**After**

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

**Diff**

```diff
 #### Margo-Specific Media Types

+##### Application Package Media Types
+
 |Media Type|Description|
 |----------|----------|
 |``application/vnd.org.margo.app.v1+json`` | MUST be used as the **artifactType** to mark the OCI image manifest as the definition of a Margo Application Package |
 |``application/vnd.org.margo.app.description.v1+yaml`` | MUST be used to mark a layer in the OCI image manifest as pointing to the Margo Application Description file |
 |``application/vnd.org.margo.app.icon.v1+{file format}``| MUST be used to mark a layer in the OCI image manifest as pointing to the icon of a Margo Application Package |
 |``application/vnd.org.margo.app.descriptionFile.v1+{file format}``| MUST be used to mark a layer in the OCI image manifest as pointing to description file of a Margo Application Package |
 |``application/vnd.org.margo.app.licenseFile.v1+{file format}``| MUST be used to mark a layer in the OCI image manifest as pointing to the license file of a Margo Application Package|
 |``application/vnd.org.margo.app.releaseNotes.v1+{file format}``| MUST be used to mark a layer in the OCI image manifest as pointing to the release notes file of a Margo Application Package|
+
+##### Component Registry Media Types
+
+|Media Type|Description|
+|----------|----------|
+|``application/vnd.org.margo.component.compose.v1+json``| MUST be used as the **artifactType** to mark an OCI image manifest as a Margo Compose Archive component in a Component Registry |
+|``application/vnd.org.margo.component.compose.v1.tar.gzip``| MUST be used as the **mediaType** for the layer blob containing the Compose Archive tarball (.tar.gz) |
```

> **Note:** Quadlet media types (`application/vnd.org.margo.component.quadlet.v1+json` and `application/vnd.org.margo.component.quadlet.v1.tar.gzip`) are introduced in WG-PROPOSAL-02 and MUST be added to this table when that proposal is applied.

---

### Change 3: `src/specification/applications/application-description.linkml.yaml`

#### Change 3a: Deprecate `packageLocation`, update `repository`/`revision` descriptions

**Before** (lines 385-412)

```yaml
  ComponentProperties:
    description: Properties dictionary for component deployment details.
    rank: 80
    attributes:
      repository:
        description: Repository location for the component.
        rank: 10
        range: string
      revision:
        description: Revision version for the component.
        rank: 20
        range: string
      wait:
        description: If True, indicates the device waits for the component installation to complete.
        rank: 30
        range: boolean
      timeout:
        description: Time to wait for component installation to complete, formatted as "##m##s".
        rank: 40
        range: string
      packageLocation:
        description: URL indicating the Compose package's location.
        rank: 50
        range: string
      keyLocation:
        description: URL for the public key used to validate a digitally signed package.
        rank: 60
        range: string
```

**After**

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
        required: false  # Becomes required: true in Phase 2 (v1-beta1)
      revision:
        description: >-
          OCI tag identifying the component version within the repository (e.g., "1.0.0", "2.3.1").
          SemVer 2.0 version without leading `v`.
          MUST be used for Helm, Compose, and Quadlet components.
        rank: 20
        range: string
        required: false  # Becomes required: true in Phase 2 (v1-beta1)
      wait:
        description: If True, indicates the device waits for the component installation to complete.
        rank: 30
        range: boolean
      timeout:
        description: Time to wait for component installation to complete, formatted as "##m##s".
        rank: 40
        range: string
      packageLocation:
        description: >-
          DEPRECATED. URL indicating the Compose package's location.
          Use `repository` and `revision` instead. This field is retained
          for backward compatibility and MUST NOT be used for new
          ApplicationDescription documents. Implementations SHOULD support
          this field during a transition period and emit a deprecation warning.
          When both `repository` and `packageLocation` are present in a
          `ComponentProperties` element, the WFM MUST use `repository` and
          `revision` and MUST ignore `packageLocation`.
        rank: 50
        range: string
        deprecated: >-
          Deprecated in favor of repository + revision OCI-based publishing.
          Will be removed in a future version of the specification.
      keyLocation:
        description: >-
          URL for the public key used to validate a digitally signed package.
          MAY be used when the component artifact is signed (e.g., using cosign or PGP).
        rank: 60
        range: string
```

**Diff**

```diff
   ComponentProperties:
     description: Properties dictionary for component deployment details.
     rank: 80
     attributes:
       repository:
-        description: Repository location for the component.
+        description: >-
+          OCI repository URI for the component (e.g., oci://registry.example.com/org/component-name).
+          MUST be used for Helm, Compose, and Quadlet components.
         rank: 10
         range: string
+        required: false  # Becomes required: true in Phase 2 (v1-beta1)
       revision:
-        description: Revision version for the component.
+        description: >-
+           OCI tag identifying the component version within the repository (e.g., "1.0.0", "2.3.1").
+           SemVer 2.0 version without leading `v`.
+           MUST be used for Helm, Compose, and Quadlet components.
         rank: 20
         range: string
+        required: false  # Becomes required: true in Phase 2 (v1-beta1)
       wait:
         description: If True, indicates the device waits for the component installation to complete.
         rank: 30
         range: boolean
       timeout:
         description: Time to wait for component installation to complete, formatted as "##m##s".
         rank: 40
         range: string
       packageLocation:
-        description: URL indicating the Compose package's location.
+        description: >-
+          DEPRECATED. URL indicating the Compose package's location.
+          Use `repository` and `revision` instead. This field is retained
+          for backward compatibility and MUST NOT be used for new
+          ApplicationDescription documents. Implementations SHOULD support
+          this field during a transition period and emit a deprecation warning.
+          When both `repository` and `packageLocation` are present in a
+          `ComponentProperties` element, the WFM MUST use `repository` and
+          `revision` and MUST ignore `packageLocation`.
         rank: 50
         range: string
+        deprecated: >-
+          Deprecated in favor of repository + revision OCI-based publishing.
+          Will be removed in a future version of the specification.
       keyLocation:
-        description: URL for the public key used to validate a digitally signed package.
+        description: >-
+          URL for the public key used to validate a digitally signed package.
+          MAY be used when the component artifact is signed (e.g., using cosign or PGP).
         rank: 60
         range: string
```

---

#### Change 3b: Rename `ComposeDeploymentProfile` type value from `"compose"` to `"compose.v1"`

The bare `"compose"` type value is tied to the legacy `packageLocation` plain-HTTPS model. The new OCI-native publishing model introduced by this proposal uses `"compose.v1"` to signal a versioned, OCI-compliant deployment profile. The legacy `"compose"` value is retained in the regex pattern for backward compatibility during the transition period.

**Before**

```yaml
  ComposeDeploymentProfile:
    is_a: DeploymentProfile
    #rank: 66
    slot_usage:
      type:
        equals_string: "compose"
        rank: 10
      components:
        range: ComposeComponent
        rank: 20
```

```yaml
    pattern: ^(helm\.v3|compose)$
```

New ApplicationDescription documents MUST use `compose.v1` as the deployment profile type value. Existing documents using `compose` remain valid during the transition period defined in the Backward Compatibility section.

**After**

```yaml
  ComposeDeploymentProfile:
    is_a: DeploymentProfile
    #rank: 66
    slot_usage:
      type:
        equals_string: "compose"
        rank: 10
      components:
        range: ComposeComponent
        rank: 20
    deprecated: >-
      Deprecated in favor of ComposeV1DeploymentProfile.
      Will be removed in Phase 2 (v1-beta1). Use ComposeV1DeploymentProfile for new documents.

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

```yaml
    pattern: ^(helm\.v3|compose|compose\.v1)$
```

> **Note:** `ComposeDeploymentProfile` (bare `"compose"`) is retained for backward compatibility during Phase 1 but is deprecated. WFM implementations SHOULD treat `"compose"` as equivalent to `"compose.v1"` with a deprecation warning during the transition period. `ComposeDeploymentProfile` will be removed in Phase 2 (v1-beta1).

**Diff**

```diff
   ComposeDeploymentProfile:
     is_a: DeploymentProfile
     #rank: 66
     slot_usage:
       type:
         equals_string: "compose"
         rank: 10
       components:
         range: ComposeComponent
         rank: 20
+    deprecated: >-
+      Deprecated in favor of ComposeV1DeploymentProfile.
+      Will be removed in Phase 2 (v1-beta1). Use ComposeV1DeploymentProfile for new documents.
+
+  ComposeV1DeploymentProfile:
+    is_a: DeploymentProfile
+    #rank: 67
+    slot_usage:
+      type:
+        equals_string: "compose.v1"
+        rank: 10
+      components:
+        range: ComposeComponent
+        rank: 20
```

```diff
-    pattern: ^(helm\.v3|compose)$
+    pattern: ^(helm\.v3|compose|compose\.v1)$
```

---

#### Change 3c: Update `type` slot description to document `compose.v1`

##### Affected file

`src/specification/applications/application-description.linkml.yaml` -- slot `type` description

##### Before

```yaml
  type:
    description: >-
          Defines the type of this deployment configuration for the application. 
          The allowed values are `helm.v3`, to indicate the deployment profile's format is Helm version 3, 
          and `compose` to indicate the deployment profile's format is a Compose file. 
          When installing the application on a device supporting the Kubernetes platform, all `helm.v3` components, 
          and only `helm.v3` components, will be provided to the device in same order they are listed in the application description file. 
          When installing the application on a device supporting Compose, all `compose` components, 
          and only `compose` components, will be provided to the device in the same order they are listed in the application description file. 
          The device will install the components in the same order they are listed in the application description file.
    range: string
    required: true
    pattern: ^(helm\.v3|compose)$
    rank: 10
```

##### After

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

##### Diff

```diff
 slots:
   type:
-    description: >-
-          Defines the type of this deployment configuration for the application. 
-          The allowed values are `helm.v3`, to indicate the deployment profile's format is Helm version 3, 
-          and `compose` to indicate the deployment profile's format is a Compose file. 
-          When installing the application on a device supporting the Kubernetes platform, all `helm.v3` components, 
-          and only `helm.v3` components, will be provided to the device in same order they are listed in the application description file. 
-          When installing the application on a device supporting Compose, all `compose` components, 
-          and only `compose` components, will be provided to the device in the same order they are listed in the application description file. 
-          The device will install the components in the same order they are listed in the application description file.
-    range: string
-    required: true
-    pattern: ^(helm\.v3|compose)$
+    description: >-
+      The deployment profile type discriminator. Allowed values:
+      - `helm.v3`: Helm-based component (see HelmDeploymentProfile).
+      - `compose.v1`: Compose-based component using OCI registry publishing (see ComposeV1DeploymentProfile). Preferred for new documents.
+      - `compose`: Compose-based component (see ComposeDeploymentProfile). Deprecated -- use `compose.v1` for new documents.
+      - `quadlet.v1`: Quadlet-based component (see QuadletDeploymentProfile, introduced by WG-PROPOSAL-02).
     rank: 10
+    range: string
+    required: true
+    pattern: ^(helm\.v3|compose|compose\.v1|quadlet\.v1)$
```

---

### Change 4: Compose Archive Structure (new section in `application-package.md`)

It defines the normative internal structure of Compose Archives.

**New content (addition)**

````markdown
## Compose Archive Structure

A Compose Archive is a gzip-compressed tar archive (`.tar.gz` or `.tgz`) that packages a [Compose](https://www.compose-spec.io/) application for deployment on edge devices. The archive MUST conform to the following structural requirements.

### Directory Layout

The archive MUST contain exactly one top-level directory. The name of this directory MUST match the component `name` as specified in the [ApplicationDescription](../../specification/applications/application-description.md#component-attributes).

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

In this example:

- `myapp` is the component name matching the ApplicationDescription.
- `compose.yaml` is the required Compose file.
- `.env` is an environment variable file referenced by `env_file` in `compose.yaml`.
- `config/app.conf` is a configuration file referenced as a bind mount or config in `compose.yaml`.

### Integrity Verification

When stored in an OCI-compliant Component Registry, the Compose Archive tarball is the content of a single layer blob. Integrity verification at the transport layer is provided by the OCI content-addressable digest as mandated by the [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md), which the Margo Application Registry already requires. No separate `digest` or hash field is needed in the ApplicationDescription.

Implementations MUST verify the OCI digest after pulling the blob and before extracting the archive.

### Publishing Workflow

To publish a Compose Archive to an OCI-compliant Component Registry:

1. Create the `.tar.gz` archive conforming to the directory layout above.
2. Push the archive as an OCI artifact using `oras push` (or equivalent OCI-compliant tooling):

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

> **Resolved**: Integrity verification is provided by the OCI content-addressable digest as mandated by the OCI Distribution Specification v1.1.0, which the Margo Application Registry already requires. No separate hash or digest field is required in the ApplicationDescription.

````

---

## Conformance Impact

This proposal introduces the following normative statements:

| RFC 2119 Keyword | Statement |
|---|---|
| MUST | Compose Archives MUST be stored in an OCI-compliant Component Registry and referenced via `repository` + `revision`. |
| MUST | The OCI image manifest for a Compose component MUST use `artifactType` = `application/vnd.org.margo.component.compose.v1+json`. |
| MUST | The layer blob mediaType for Compose MUST be `application/vnd.org.margo.component.compose.v1.tar.gzip`. |
| MUST | A Compose Archive MUST contain exactly one top-level directory whose name matches the component `name`. |
| MUST | A Compose Archive MUST contain a `compose.yaml` file in the top-level directory. |
| MUST | The `compose.yaml` file MUST conform to the Compose Specification as currently published. |
| SHOULD | The TWG SHOULD establish a minimum Compose Specification compatibility baseline in a follow-up proposal. |
| MUST | All files referenced by `compose.yaml` MUST be included within the archive using relative paths. |
| MUST NOT | Symlinks MUST NOT target paths outside the top-level directory. |
| MUST NOT | Hard links MUST NOT reference paths outside the top-level directory. |
| MUST NOT | Absolute paths MUST NOT appear in archive entries. |
| MUST NOT | File names MUST NOT contain path traversal sequences. |
| MUST NOT | Implementations MUST NOT preserve setuid, setgid, or sticky bits from archive entries. |
| MUST | WFM and device implementations MUST validate security constraints before extraction. |
| MUST | Implementations MUST verify the OCI digest after pulling and before extracting. |
| MUST NOT | `packageLocation` MUST NOT be used for new ApplicationDescription documents. |
| MUST | When both `repository` and `packageLocation` are present, the WFM MUST use `repository`/`revision` and MUST ignore `packageLocation`. |
| SHOULD | Implementations SHOULD support `packageLocation` during a transition period with a deprecation warning. |
| SHOULD | Implementations SHOULD normalize file permissions during archive extraction. |
| SHOULD | Authentication for secure registries SHOULD follow the existing Authentication, Authorization & Security section. |
| MAY | A deployment profile type MAY be omitted if it cannot be implemented. |

---

## Backward Compatibility

### Breaking Changes

1. **`packageLocation` deprecation**: Existing ApplicationDescription documents that use `packageLocation` for Compose components will trigger a deprecation warning. These documents remain valid during the transition period but MUST be migrated to `repository` + `revision` before `packageLocation` is removed in a future spec version.

2. **`repository` and `revision` transition to required**: In Phase 1, both fields are marked `required: false` in the schema to preserve validation compatibility. Existing Compose ApplicationDescription documents that only use `packageLocation` (without `repository`/`revision`) remain schema-valid during Phase 1. In Phase 2 (v1-beta1), both fields become `required: true`, at which point documents lacking these fields will fail schema validation.

3. **Compose Archive structure constraints**: Existing Compose Archives that do not follow the single-top-level-directory convention or that use `docker-compose.yaml` instead of `compose.yaml` will become non-compliant and must be restructured.

### Migration Path

1. Application developers MUST update Compose-based ApplicationDescription documents to use `repository` + `revision` pointing to an OCI registry.
2. Application developers MUST push their Compose Archives to an OCI registry using `oras push` (or equivalent) with the correct media types.
3. Application developers MUST restructure their Compose Archives to use a single top-level directory matching the component name, with `compose.yaml` as the entry point.
4. The `packageLocation` field MAY be retained alongside `repository`/`revision` during the transition period for backward compatibility with older WFM implementations.
5. WFM implementations SHOULD be updated to resolve components via OCI pull (using `repository` + `revision`) and fall back to `packageLocation` only when `repository` is absent.

### Version Transition Strategy

This proposal adopts a two-phase transition to manage the breaking change introduced by making `repository` and `revision` required.

**Phase 1 (this spec version, pre-draft):** `repository` and `revision` are **SHOULD** for Compose components; `packageLocation` remains valid. The schema sets `required: false` on `repository` and `revision` to preserve validation compatibility with existing documents that use only `packageLocation`. The schema version bumps from v1-alpha1 to v1-alpha2 to signal the change without breaking existing validators. Phase 1 also introduces `ComposeV1DeploymentProfile` (with `equals_string: "compose.v1"`) alongside the existing `ComposeDeploymentProfile` (with `equals_string: "compose"`); both are valid during this phase.

**Phase 2 (v1-beta1 or later):** `repository` and `revision` become **REQUIRED** (`required: true` is set); `packageLocation` is removed from the schema entirely. `ComposeDeploymentProfile` (bare `"compose"`) is removed; only `ComposeV1DeploymentProfile` remains.

> **Note:** Implementations relying solely on `packageLocation` SHOULD migrate during Phase 1. No production backward-compatibility guarantee exists for pre-draft schema versions.

### Version Impact

- The `packageLocation` deprecation, the addition of `ComposeV1DeploymentProfile`, and the Phase 2 promotion of `repository`/`revision` to `required: true` constitute breaking schema changes.
- The schema version MUST be bumped to reflect the breaking changes introduced by this proposal. The LinkML schema `version` MUST be updated from `1.0` to `1.1` and the API version label from v1-alpha1 to v1-alpha2 accordingly. The Phase 2 promotion to `required: true` will be accompanied by a further version increment to v1-beta1.

---

## References

- [OCI Distribution Specification v1.1.0](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md)
- [OCI Image Specification v1.1.0](https://github.com/opencontainers/image-spec/blob/v1.1.0/manifest.md)
- [RFC 2119 -- Key words for use in RFCs to Indicate Requirement Levels](https://www.rfc-editor.org/rfc/rfc2119)
- [RFC 6838 -- Media Type Specifications and Registration Procedures](https://www.rfc-editor.org/rfc/rfc6838)
- [Compose Specification](https://www.compose-spec.io/)
- [ORAS (OCI Registry as Storage)](https://oras.land/)
- WG-PROPOSAL-00: The Helm Way (companion informational document)

---

*This document is part of the Margo WG Compose/Quadlet OCI Publishing proposal package. Prepared by the Andrii Melashchenko (Belden Inc.), 2026-05-04. Subject to the Open Web Foundation Contributor License Agreement governing the Margo specification.*
