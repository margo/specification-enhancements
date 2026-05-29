## SUP: Address Problems of Conflicting Resource Allocation to Workloads

## Owner

[@singhmj-1](https://github.com/singhmj-1)

## Summary

This proposal introduces a platform-managed infrastructure binding mechanism to resolve conflicts that arise when applications expose endpoints, consume storage, or access hardware on Margo-managed devices. It defines a set of standardized and extensible provider capability types _(capability.margo.org/network/port, capability.margo.org/network/ingress, capability.margo.org/storage/volume, capability.margo.org/camera)_ with formal input/output schemas, and establishes that infrastructure bindings are resolved at deploy time within the Margo Deployment Manifest.

The ApplicationDescription remains a portable, environment-agnostic vendor artifact that declares only symbolic parameter names and their targets within deployment components. The Deployment Manifest, constructed by the Workload Fleet Manager (WFM) or operator, binds those parameters to platform-assigned resources for a specific target device. The Device Agent enforces binding constraints at deployment time, failing with RESOURCE_CONSTRAINTS_CANNOT_BE_MET when constraints cannot be satisfied. The WFM MAY proactively prevent deployment when it maintains a resource registry for the target device.

### Problem Statement

Application vendors deploying to Margo-managed devices have no prior knowledge of the target environment — what ports are in use, what ingress controller is installed, or what other applications are already deployed. This creates conflicts when exposing ports, routes, hostnames, and hardware resources across Docker Compose, Kubernetes (Ingress, Service, Gateway API), and other runtimes.

## Reason for Proposal

Application vendors have no prior knowledge of the devices their applications will be deployed to. This constraint creates a class of unresolved conflicts that currently have no standardized solution within the Margo specification:

- Port conflicts arise in Docker Compose and other deployments when multiple applications attempt to publish the same host port, or when an application targets a port already in use by a non-Margo process or a reserved system port.
- Ingress and routing conflicts arise in Kubernetes deployments when multiple applications claim the same hostname, subdomain, or URL route, or when an application's ingress configuration is incompatible with the ingress controller installed on the device.
- Hardware resource conflicts arise when multiple applications attempt to claim exclusive access to the same physical device, such as a USB camera.
- Storage conflicts arise when volume naming, access modes, or persistence policies are not coordinated across applications sharing a device.

Without a standardized mechanism, application vendors are forced to either hard-code environment-specific values — breaking portability — or leave conflicts unresolved, leading to unpredictable runtime failures.

## Requirements alignment acknowledgement

This proposal addresses [this issue #75](https://github.com/margo/specification/issues/75).

### Proposed Solution: Infrastructure Bindings in the Margo Deployment Manifest

**Key architectural decision:** The `ApplicationDescription` remains a portable artifact authored by the vendor. Infrastructure bindings are resolved at deploy time and live in the **Margo Deployment Manifest**, which is constructed by the Workload Fleet Manager (WFM) or operator when targeting a specific device.

This separation means:
- App vendors declare *what* their app needs (parameters with symbolic names)
- The platform resolves *how* those needs are met on a specific device

---

### Artifact Separation

```
ApplicationDescription.yaml        (authored by vendor, environment-agnostic)
        │
        │  declares: named parameters with no binding
        ▼
ApplicationDeploymentManifest.yaml            (constructed by WFM/operator at deploy time)
        │
        │  resolves: infrastructureBindings → parameter values
        ▼
Device Agent                       (applies resolved values to deployment components)
```

---

### Predefined Provider Types and their Formal Schemas

#### `capability.margo.org/network/port`

```yaml
providerType: capability.margo.org/network/port
schema:
  input:
    preferredPort:
      type: integer
      minimum: 1
      maximum: 65535
      description: Preferred port number; treated as advisory unless strategy is strict.
    protocol:
      type: enum
      values: [TCP, UDP]
      required: true
    strategy:
      type: enum
      values: [random, strict]
      default: random
      description: |
        random: Platform assigns any available port. preferredPort is a hint.
        strict: Platform MUST assign preferredPort exactly. If unavailable,
                deployment fails with RESOURCE_CONSTRAINTS_CANNOT_BE_MET.
                WFM MAY proactively block deployment before reaching the device
                if it maintains a port registry for the target device.
  output:
    assignedPort:
      type: integer
      description: The port number assigned by the platform.
```

#### `capability.margo.org/network/ingress`

```yaml
providerType: capability.margo.org/network/ingress
schema:
  input:
    preferredRoute:
      type: string
      pattern: "^/.*"
      description: Preferred URL path prefix. Advisory unless strategy is strict.
    preferredHostname:
      type: string
      description: Preferred hostname/subdomain. Advisory unless strategy is strict.
    protocol:
      type: enum
      values: [HTTP, HTTPS]
      default: HTTP
    tlsTermination:
      type: enum
      values: [edge, passthrough, reencrypt]
      required: false
      description: |
        edge: TLS terminated at ingress controller.
        passthrough: TLS passed through to the app; route MUST be root (/), hostname required.
        reencrypt: TLS terminated and re-encrypted toward the backend.
    strategy:
      type: enum
      values: [random, strict]
      default: random
      description: |
        random: Platform assigns a unique route/hostname, using preferences as hints.
        strict: Platform MUST assign exactly the preferredRoute and/or preferredHostname.
                If unavailable, deployment fails with RESOURCE_CONSTRAINTS_CANNOT_BE_MET.
                WFM MAY proactively block deployment if it maintains a route registry.
  output:
    assignedRoute:
      type: string
      description: The URL path prefix assigned by the platform.
    assignedHostname:
      type: string
      description: The hostname assigned by the platform.
    assignedURL:
      type: string
      description: Fully composed URL (scheme + hostname + route) for convenience.
```

#### `capability.margo.org/storage/volume`

```yaml
providerType: capability.margo.org/storage/volume
schema:
  input:
    size:
      type: string
      pattern: "^[0-9]+(Mi|Gi|Ti)$"
      required: true
      description: Requested storage size (e.g., 10Gi).
    accessMode:
      type: enum
      values: [ReadWriteOnce, ReadOnlyMany, ReadWriteMany]
      required: true
    persistenceMode:
      type: enum
      values: [Retain, Delete, Recycle]
      required: true
      description: |
        Retain: Volume persists after application removal.
        Delete: Volume is deleted with the application.
        Recycle: Volume is scrubbed and made available again.
    storageClass:
      type: string
      required: false
      description: Optional storage class hint. Platform may override.
  output:
    volumeName:
      type: string
      description: Name of the provisioned PersistentVolumeClaim (Kubernetes) or named volume (Compose).
    volumeNamespace:
      type: string
      description: Namespace in which the volume was provisioned (Kubernetes only).
    mountPath:
      type: string
      description: Resolved host or cluster mount path for Compose or direct-mount scenarios.
```

#### `capability.margo.org/camera`

```yaml
providerType: capability.margo.org/camera
schema:
  input:
    interface:
      type: enum
      values: [USB, CSI, IP, virtual]
      required: true
      description: Physical or logical interface type for the camera.
    minimumResolution:
      type: string
      pattern: "^[0-9]+p$|^[0-9]+x[0-9]+$"
      required: false
      description: Minimum required resolution (e.g., 1080p or 1920x1080).
    minimumFramerate:
      type: integer
      minimum: 1
      required: false
      description: Minimum required frames per second.
    fallback:
      type: enum
      values: [fail, warn]
      default: fail
      description: |
        fail: Deployment fails with RESOURCE_CONSTRAINTS_CANNOT_BE_MET if no
              matching camera is available. WFM MAY proactively block deployment.
        warn: Deployment proceeds; device agent emits a warning if constraints
              are not fully met.
  output:
    assignedDevicePath:
      type: string
      description: Host device path assigned by the platform (e.g., /dev/video0).
    cameraId:
      type: string
      description: Platform-assigned camera identifier for use in application configuration.
    resolvedResolution:
      type: string
      description: Actual resolution of the assigned camera device.
    resolvedFramerate:
      type: integer
      description: Actual framerate of the assigned camera device.
```

Similarily, these provider types can be extended to address other hardware requirements and the vendors can also bring their own providers.

---

### ApplicationDescription (Vendor Artifact — No Bindings)

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDescription
metadata:
  id: test-app-123456789
  name: Test Application
  description: The Test Application
  version: 1.0.0
  catalog:
    application:
      icon: ./resources/app-logo.png
      tagline: Always test before relying on something.
      descriptionFile: ./resources/description.md
      releaseNotes: ./resources/release-notes.md
      licenseFile: ./resources/license.pdf
      site: http://www.margo.org
      tags: ["test"]
    author:
      - name: ExampleAuthor
        email: example@margo.org
    organization:
      - name: Margo
        site: http://margo.org

deploymentProfiles:
  - type: helm
    id: test-app-123456789-helm
    components:
      - name: hello-world
        properties:
          repository: oci://example-registry.margo.org/charts/hello-world
          revision: 1.0.0

  - type: compose
    id: test-app-123456789-compose
    components:
      - name: hello-world-docker
        properties:
          packageLocation: https://example.margo.org

# Parameters declare symbolic names only. No bindings here.
# Bindings are resolved in the Deployment Manifest at deploy time.
parameters:
  - name: apiPort
    targets:
      - component: "hello-world"
        pointer: "service.port"
      - component: "hello-world-docker"
        pointer: "HOST_API_PORT"

  - name: metricsPort
    targets:
      - component: "hello-world"
        pointer: "service.metricsPort"
      - component: "hello-world-docker"
        pointer: "HOST_METRICS_PORT"

  - name: apiRoute
    targets:
      - component: "hello-world"
        pointer: "ingress.path"
      - component: "hello-world-docker"
        pointer: "REVERSE_PROXY_PATH"

  - name: dataVolume
    targets:
      - component: "hello-world"
        pointer: "persistence.existingClaim"
      - component: "hello-world-docker"
        pointer: "HOST_VOLUME_MOUNT"

  - name: cameraDevice
    targets:
      - component: "hello-world"
        pointer: "hardware.camera.device"
      - component: "hello-world-docker"
        pointer: "CAMERA_DEVICE_PATH"
```

---

### Deployment Manifest (WFM/Operator Artifact — Bindings Resolved Here)

```yaml
apiVersion: margo.org/v1-alpha1
kind: DeploymentManifest
metadata:
  id: deployment-test-app-device-xyz-001
  applicationId: test-app-123456789

###############################################################################
infrastructureBindings:
  - id: portA
    provider: capability.margo.org/network/port
    hint:
      preferredPort: 8080
      protocol: TCP
      strategy: random

  - id: portB
    provider: capability.margo.org/network/port
    hint:
      preferredPort: 8081
      protocol: TCP
      strategy: strict
      # strict: if port 8081 is unavailable, deployment fails with
      # RESOURCE_CONSTRAINTS_CANNOT_BE_MET. WFM MAY block proactively
      # if it maintains a port registry/book for the target device.

  - id: ingressA
    provider: capability.margo.org/network/ingress
    hint:
      preferredRoute: /portal
      protocol: HTTPS
      tlsTermination: edge
      strategy: random

  - id: volumeA
    provider: capability.margo.org/storage/volume
    hint:
      size: 10Gi
      accessMode: ReadWriteOnce
      persistenceMode: Retain

  - id: camera1
    provider: capability.margo.org/camera
    hint:
      interface: USB
      minimumResolution: "1080p"
      minimumFramerate: 30
      fallback: fail
      # fail: deployment fails with RESOURCE_CONSTRAINTS_CANNOT_BE_MET
      # if no matching camera is available.

# Bindings resolve to ApplicationDescription parameter names
parameters:
  - name: apiPort
    fromBinding: portA.output.assignedPort

  - name: metricsPort
    fromBinding: portB.output.assignedPort

  - name: apiRoute
    fromBinding: ingressA.output.assignedRoute

  - name: dataVolume
    fromBinding: volumeA.output.volumeName

  - name: cameraDevice
    fromBinding: camera1.output.assignedDevicePath
```

---

### Conflict Resolution & Failure Semantics

| Scenario | `strategy` / `fallback` | Behavior |
|---|---|---|
| Preferred port available | `strict` or `random` | Port assigned as preferred |
| Preferred port unavailable | `random` | Platform assigns next available port |
| Preferred port unavailable | `strict` | Deployment fails: `RESOURCE_CONSTRAINTS_CANNOT_BE_MET` |
| WFM has device port registry | `strict` | WFM MAY proactively reject before reaching device agent |
| WFM has no device registry | `strict` | Device agent enforces and reports failure |
| No matching camera | `fail` | Deployment fails: `RESOURCE_CONSTRAINTS_CANNOT_BE_MET` |
| No matching camera | `warn` | Deployment proceeds; device agent emits warning |
| Route conflict | `random` | Platform assigns unique route |
| Route conflict | `strict` | Deployment fails: `RESOURCE_CONSTRAINTS_CANNOT_BE_MET` |

---

### How This Addresses the Identified Problems

| Problem | Resolution |
|---|---|
| Port conflicts (Compose/WASM/K8s NodePort) | `capability.margo.org/network/port` with `random` strategy |
| Reserved/restricted ports | Platform enforces policy during binding resolution |
| Ingress hostname/route conflicts | `capability.margo.org/network/ingress` with platform-assigned routes |
| Unknown ingress controller | App declares intent via binding; platform maps to installed controller |
| NodePort/LoadBalancer port conflicts | Platform manages uniqueness via port binding registry |
| Hardware device conflicts | `capability.margo.org/camera` with explicit capability matching |
| Namespace naming conflicts | Platform scopes bindings to deployment namespace at manifest level |

---

### Open Questions unable to answer yet

1. **WFM registry depth** — Should WFM maintain a full resource registry per device (ports, routes, volumes, devices) to enable proactive conflict detection, or should it rely on the device agent to report failures? This is a significant architectural decision affecting WFM complexity.
2. **Binding output timing** — Are binding outputs (e.g., `assignedPort`) resolved before or after the deployment manifest is sent to the device agent? This affects whether the manifest is fully resolved at WFM or partially resolved on-device.
3. **Multi-profile binding** — If both Helm and Compose profiles are valid for an app, are separate deployment manifests required per profile, or can one manifest reference both? 
I think that we need to have separate providers for helm, and compose and then define separate params for each of them, or we can club the infra bindings inside the deploymentProfile as well.

---

### Impacted Margo Components
- **Application Definition Package** — `ApplicationDescription` schema: Intact. Nothing touched.
- **Deployment Manifest** — New artifact type with `infrastructureBindings` and `parameterBindings`
- **Workload Fleet Manager** — Optional binding resolution engine, and proactive conflict detection registry
- **Device Agent** — Optional binding resolution engine, and Must binding enforcement, failure reporting with errors like `RESOURCE_CONSTRAINTS_CANNOT_BE_MET`

### Conformance

- Unknown providers MUST be ignored by default.

## Alternatives considered (optional)

1. Vendor-Defined Bindings in the ApplicationDescription
The first approach considered placing infrastructureBindings directly inside the ApplicationDescription, allowing vendors to declare their resource requirements alongside their deployment profiles. While this kept all application-related configuration in one place, it was rejected because it forced vendors to make environment-specific decisions — such as preferred ports and ingress routes — at authoring time. An ApplicationDescription authored for one device class would carry assumptions that may be invalid or conflicting on another, making the artifact no longer truly environment-agnostic. The binding concerns belong to the deployment context, not the application definition.

2. Static Pre-Assignment by the Operator
An alternative considered was requiring operators to statically pre-assign all resource values (ports, routes, volumes) before deployment, effectively treating them as manually configured parameters passed into the deployment. While simple to implement, this approach places the full burden of conflict detection and resource tracking on the human operator, which does not scale across fleets of devices with many concurrently deployed applications. It also provides no standardized schema or tooling support, meaning each WFM implementation would handle it differently, again undermining interoperability.

3. Application-Level Namespacing and Convention-Based Conflict Avoidance
This approach proposed that Margo define strict naming and port-range conventions — for example, assigning each registered application vendor a reserved port range or route namespace — so that conflicts are avoided by convention rather than runtime resolution. While attractive for its simplicity, it was rejected because it requires a centralized global registry of vendor allocations, introduces significant coordination overhead, and still cannot prevent conflicts arising from non-Margo processes already running on the device. It also does not address hardware resource conflicts such as camera access, where convention-based namespacing is not applicable.

4. Device Agent as the Sole Resolver with No WFM Involvement
This proposal suggested that all binding resolution be deferred entirely to the Device Agent at deployment time, with the WFM acting only as a pass-through. The Device Agent would detect conflicts locally and report failures back. While this minimizes WFM complexity, it was considered insufficient because it allows deployments to reach the device before any conflict validation occurs, resulting in late-stage failures that are more disruptive and harder to surface to the operator. It also precludes the possibility of proactive fleet-wide conflict detection, which is valuable when the WFM manages many devices and applications simultaneously.

## Rejection reason (optional)

N/A