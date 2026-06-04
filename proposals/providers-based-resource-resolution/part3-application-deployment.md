[<- Home](part1-providers.md#top)
# Specification Update Proposal — Providers-Based Resource Resolution - Part 3 : Deployment

---

Part 1 defined the Provider concept. Part 2 defined how devices advertise providers to the WFM. This SUP defines how deployments *consume* providers —  how resolved values flow into application components, how the WFM/Device validates before dispatch, and how the Device Agent resolves and enforces bindings locally.

This SUP introduces one new optional field to `ApplicationDeployment`:

- `requiredCapabilities` — utilized the providers to resolve device resources

No changes are made to `ApplicationDescription`. It remains a portable, environment-agnostic vendor artifact.

> **⚠️ Illustrative Examples — Not Normative**
>
> The `ApplicationDeployment` examples in this document are illustrative. The specific input and output field names within each provider's `request` block and `valueFrom` references will be defined by individual provider SUPs.

---

### The Key Architectural Decision

Bindings live in `ApplicationDeployment` only.

`ApplicationDescription` is a vendor artifact. It must be environment-agnostic. An app vendor cannot know at authoring time which port will be safe on a given device, which ingress controller is installed, or which GPU is available. Putting environment-specific binding declarations in `ApplicationDescription` would break portability — the same app could not be deployed to different devices without modifying the vendor artifact.

`ApplicationDeployment` is an operator artifact. It is created per deployment, per device. It is the correct place for environment-specific declarations.

```
ApplicationDescription (vendor artifact — portable, environment-agnostic)
    ↓ declares symbolic parameter names and targets
ApplicationDeployment (operator artifact — per deployment, per device)
    ↓ declares requiredCapabilities + parameter bindings
    ↓ WFM resolves fleet-scope bindings
    ↓ dispatches to Device Agent
Device Agent
    ↓ resolves device-scope bindings locally
    ↓ applies resolved values to components via parameter bindings
    ↓ reports status via Deployment Status API
```

---

## Technical Proposal

### `ApplicationDescription` — No Changes

The existing parameter structure is unchanged. Parameters declare symbolic names and targets only:

```yaml
parameters:
  hostPort:
    value: 8080          # default value — not a hard requirement
    targets:
      - pointer: service.port
        components: ["my-app"]
  gpuDevicePath:
    targets:
      - pointer: hardware.gpu.devicePath
        components: ["my-app"]
      - pointer: ENV.GPU_DEVICE_PATH
        components: ["my-app-compose"]
```

The app vendor ships one `ApplicationDescription`. It works on any Margo-compliant device regardless of what port is actually assigned or which GPU device path is used. The binding resolution is entirely the operator's and platform's concern.

### `ApplicationDeployment` Extensions

There are two changes to the `spec` section of `ApplicationDeployment`:

#### `requiredCapabilities`

Uses the provider contracts to resolve the device resources.

```yaml
spec:
  requiredCapabilities:
    - id: <binding-reference-name>
      provider: <provider-uri>
      scope: device | fleet # scope of resolving this provider at
      request:
        <provider-specific-input-fields>
```

| Field | Type | Required? | Description |
|---|---|---|---|
| `id` | string | Y | A unique name for this binding within the deployment. Used in `parameters` to reference the resolved output. Must be unique within the `requiredCapabilities` array. |
| `provider` | string | Y | The provider URI as defined in SUP 1. Must follow the `capability.<namespace>/<category>/<name>` convention. |
| `scope` | string | N | `device` or `fleet`. If omitted, the WFM infers scope from the provider's `ProviderSchema`. Explicit declaration is recommended for clarity. |
| `request` | object | N | Provider-specific request fields as defined by the provider's `ProviderSchema`. All request fields are advisory unless the provider schema declares them required. |

#### `Mention parameter bindings`

This will be used to wire the resolved provider output values to application parameter names

```yaml
spec:
  parameters:
    - name: <parameter-name>
      valueFrom: <binding-id>.<output-field-path>
```

| Field | Type | Required? | Description |
|---|---|---|---|
| `name` | string | Y | The parameter name as declared in `ApplicationDescription`. |
| `valueFrom` | string | Y | A dot-notation reference to a resolved output field. Format: `<binding-id>.result.<field-name>`. The `binding-id` MUST match an `id` in `requiredCapabilities`. |


### Full `ApplicationDeployment` Example

```yaml
# -----------------------------------------------------------------------
# Full ApplicationDeployment Example
# Scenario: Hello World deployed to a standalone Kubernetes
# cluster device. The deployment requires:
#   - A safe HTTP port (vendor defaults to 8080, accepts any assignment)
#   - An ingress hostname the operator controls (validate before deploy)
#   - A storage class matching ReadWriteOnce persistent storage
#   - An available NVIDIA GPU for ML inference
# -----------------------------------------------------------------------

apiVersion: application.margo.org/v1alpha1
kind: ApplicationDeployment
id: a3e2f5dc-912e-494f-8395-52cf3769bc06
metadata:
  annotations:
    applicationId: hello-world-app-100
  name: hello-world-app-deployment
  namespace: margo-poc

spec:
  deploymentProfile:
    type: helm
    components:
      - name: hello-world
        properties:
          repository: oci://registry.someorganisation.com/charts/hello-world
          revision: 1.0.0
          wait: "true"

  # -----------------------------------------------------------------------
  # requiredCapabilities
  # Declares what the platform must assign or validate before deployment.
  # Each binding has a unique id used to wire its output to a parameter.
  # -----------------------------------------------------------------------
  requiredCapabilities:
    # Need a safe HTTP port.
    # The vendor defaults to 8080 but accepts any available port.
    # fallback.strategy: nextAvailable — WFM assigns the next available port if 8080 is taken.
    - id: webPort
      provider: capability.margo.org/network/available-port
      scope: device
      request:
        port: 8080
        protocol: TCP
      fallback:
        strategy: nextAvailable

    # Validate if ingress hostname is already taken.
    # strategy: validate — if "helloworld.com" is already allocated,
    # the WFM rejects with a named conflict error. The platform NEVER
    # silently assigns an alternative. The operator changes the prefix
    # and resubmits.
    - id: ingressRoute
      provider: capability.margo.org/network/ingress
      scope: device
      request:
        hostname: helloworld.com
      fallback:
        strategy: fail

    # Binding 3: Fetch the device's preferred storage class.
    # No input required — the device advertises its default storage class.
    # on failure, the app deployment request will be rejected
    - id: appStorage
      provider: capability.margo.org/storage/volume
      scope: device
      request:
        defaultClassName: true
      fallback:
        strategy: fail

    # Request an available NVIDIA GPU.
    # fallback: fail — if no NVIDIA GPU is available, block deployment.
    # The resolved devicePath is injected into the output by the provider so the
    # chart can mount the correct device node into the container.
    - id: mlGPU
      provider: capability.margo.org/hardware/gpu
      scope: device
      request:
        manufacturer: NVIDIA
        minimumVRAMGiB: 16
      fallback:
        strategy: fail

  # -----------------------------------------------------------------------
  # parameters
  # Wires each capability's resolved output to an ApplicationDescription
  # If a parameter appears in both user-provided values and a binding,
  # the binding value takes precedence.
  # -----------------------------------------------------------------------
  parameters:
    - name: hostPort
      valueFrom: webPort.result.assignedPort
      targets:
        - pointer: service.nodePort
          components:
            - hello-world

    - name: storageClass
      valueFrom: appStorage.result.storageClassName
      targets:
        - pointer: persistence.storageClass
          components:
            - hello-world

    - name: gpuDevicePath
      valueFrom: mlGPU.result.devicePath
      targets:
        - pointer: hardware.gpu.devicePath
          components:
            - hello-world
```

### WFM Pre-Dispatch Validation Flow

Before dispatching an `ApplicationDeployment` to a device, the WFM MUST execute the following validation steps in order. If any step fails, the WFM MUST reject the deployment and return the corresponding error. The deployment MUST NOT be dispatched to the device.

```
Step 1: For each requiredCapabilities entry, validate provider scope

  Is the scope neither "fleet" nor "device"?
  → YES: Reject — PROVIDER_NOT_SUPPORTED
          "Provider {uri} is not supported."

  Is the scope "fleet"?
  → YES: Is the provider supported by the WFM?
      → NO:  Reject — PROVIDER_NOT_SUPPORTED
              "Provider {uri} is not supported."
      → YES: Continue to Step 2.

  Is the scope "device"?
  → YES: Is the provider URI declared `supported: true` in the device's
          current DeviceCapabilitiesManifest?
      → NO:  Reject — PROVIDER_NOT_SUPPORTED
              "Provider {uri} is not supported by device {deviceId}."
      → YES: Continue to Step 2.

Step 2: For each requiredCapabilities entry, resolve the provider schema

  Is the ProviderSchema resolvable?
  (Official margo.org providers: from spec.
   Custom providers: from cached schemaURL.)
  → NO:  Reject — UNKNOWN_PROVIDER_TYPE
          "Provider schema for {uri} could not be resolved."
  → YES: Continue to Step 3.

Step 3: Resolve fleet-scoped bindings

  For each requiredCapabilities entry with scope: fleet,
  resolve the binding using WFM-internal mechanisms.
  Replace fleet-scoped requiredCapabilities from the
  ApplicationDeployment with the actual values before dispatch.
  (Device Agent MUST NOT receive unresolved fleet-scoped bindings.)

Step 4: Dispatch

  Forward ApplicationDeployment to Device Agent via Desired State API.
  Device-scoped requiredCapabilities are forwarded as-is.
```

> **Important:** WFM pre-dispatch validation is advisory — it is based on cached device capability state which may be stale. The Device Agent MUST enforce all constraints independently and authoritatively, regardless of whether the WFM performed pre-dispatch validation.

### Device Agent Resolution Flow

When the Device Agent receives an `ApplicationDeployment` containing `requiredCapabilities`, it MUST execute the following steps before applying any deployment components:

```
Step 1: Identify device-scope bindings
  Process only requiredCapabilities with scope: device.

Step 2: For each device-scope binding, resolve locally
  Apply provider constraints as defined in the ProviderSchema.
  Apply resolution strategy (random / strict / validate).

  If resolution fails:
    Record failure with provider-defined failure code and message.
    DO NOT apply any deployment components.
    Report Deployment Status with Provider defined error codes.

Step 3: Apply resolved values in the Application Deployment

Step 4: Apply deployment components
  On success: Report Deployment Status Update
  On failure: Report Deployment Status Update with Provider runtime error.

Step 5: Trigger DeviceCapabilitiesManifest update (SUP 2).
```

## Deployment Status API — Binding Failure Codes

The following failure codes are added to the existing error reporting model for binding-related failures. These codes appear in the `error.code` field of the `DeploymentStatusManifest`.

| Code | Description | Fatal |
|---|---|---|
| `PROVIDER_NOT_SUPPORTED` | The deployment references a provider URI the device does not support. | Yes |
| `UNKNOWN_PROVIDER_TYPE` | The provider schema could not be resolved. | Yes |
| `RESOURCE_CONSTRAINTS_CANNOT_BE_MET` | The requested resource is outside the device's advertised constraints, or the preferred value is unavailable. | Yes |
| `BINDING_RESOLUTION_FAILED` | A binding could not be resolved for a reason not covered by the above codes. The error message MUST include the binding `id`, the provider URI, and a human-readable description. | Yes |
| `UNKNOWN_PROVIDER_FIELD` | A device-scope provider contains an input field the device agent does not recognize, and the field is declared `required: true` in the provider schema version the device supports. | Yes |

**Example `DeploymentStatusManifest` with binding failure:**

```json
{
  "apiVersion": "deployment.margo.org/v1alpha1",
  "kind": "DeploymentStatusManifest",
  "deploymentId": "a3e2f5dc-912e-494f-8395-52cf3769bc06",
  "status": {
    "state": "failed",
    "error": {
      "code": "RESOURCE_CONSTRAINTS_CANNOT_BE_MET",
      "message": "helloworld.com already allocated to deployment-other-app-001"
    }
  },
  "components": [
    {
      "name": "hello-world",
      "state": "pending",
      "error": {
        "code": "",
        "message": ""
      }
    }
  ]
}
```

> Note: When a binding failure occurs, all components remain in `pending` state as their deployment hasn't even started yet. The overall deployment state is `failed`.

---

## Use Cases

The following use cases are drawn directly from `margo/specification#75`. Each demonstrates the end-to-end flow through `requiredCapabilities`, WFM validation, and Device Agent resolution.

### UC1: Port Conflict — Compose

**Scenario:** Two vendors both default to port `8080` on a Compose device. App A is already deployed. The operator deploys App B.

**Without this SUP:** App B's container starts, tries to bind `8080`, gets `bind: address already in use`. The Compose service crashes. The WFM shows "installed."

**With this SUP:**

App B's `ApplicationDeployment`:
```yaml
requiredCapabilities:
  - id: webPort
    provider: capability.margo.org/network/port
    scope: device
    request:
      port: 8080
      protocol: TCP
    fallback:
      strategy: nextAvailable
    

parameters:
  - name: hostPort
    valueFrom: webPort.result.assignedPort
```

**Flow — WFM pre-dispatch (Path A, capability report current):**
1. WFM checks requiredCapabilities.
2. It sees `capability.margo.org/network/port` is device scoped and supported by the device. Prepares the `desiredState`:
.....
3. Device agent pulls the desired state.
4. Device agent iterates over requiredCapabilities.
5. Checks that Port `8080` is allocated to `deployment-app-a-001`
6. `strategy: nextAvailable` — Agent selects next available port: `8083`
7. Device agent fills this value and dispatches `ApplicationDeployment` with `webPort` binding
8. Deploys the application
9. Pushes updated `DeviceCapabilitiesManifest` with `8083` allocated

**Result:** No runtime crash. Operator sees assigned port `8083` in deployment status.

---

### UC2: Ingress Hostname Conflict — Kubernetes

**Scenario:** Phil's use case from the PR review. An app defaults to `hello-world.com`. The supplier defines the ingress manifest, TLS, and route path inside the Helm chart. The operator wants to avoid a conflict but must remain in control — the platform must never silently assign an unknown hostname.

**Without this SUP:** If `hello-world.com` is taken, the ingress controller picks one app arbitrarily. The other is silently unreachable. No error is ever reported.

**With this SUP:**

`ApplicationDescription` (vendor artifact — unchanged):
```yaml
parameters:
  ingressPrefix:
    value: helloworld    # default — operator can change at deploy time
    targets:
      - pointer: ingress.hostname
        components: ["hello-world-chart"]
```

`ApplicationDeployment` (operator artifact):
```yaml
requiredCapabilities:
  - id: ingressRoute
    provider: capability.margo.org/network/ingress
    scope: device
    request:
      preferredHostname: helloworld.com
    fallback:
      strategy: fail
```

**Flow — conflict path (Path A):**
1. WFM checks requiredCapabilities.
2. It sees `capability.margo.org/network/ingress` is device scoped and supported by the device. Prepares the `desiredState`:
.....
3. Device agent pulls the desired state.
4. Device agent iterates over requiredCapabilities.
1. Find that `helloworld.com` is allocated to `deployment-other-app-001`
3. `strategy: fail` — Device rejects with:
   ```
   RESOURCE_CONSTRAINTS_CANNOT_BE_MET:
   "helloworld.com already allocated to
    deployment-other-app-001"
   ```
4. Operator changes `preferredHostname` to `hello-world-v2` and resubmits
5. WFM checks — `hello-world-v2.com` is free
6. Dispatches to Device Agent, which continues with deployment.

**Result:** If conflict is there, the deployment is rejected. Operator is always in control.

---

### UC3: Exclusive Hardware Conflict — Camera

**Scenario:** A machine vision app requires exclusive access to a 4K USB camera. A security monitoring app already holds the only 4K camera on the device.

**Without this SUP:** The machine vision app's container starts, tries to open `/dev/video0`, gets `Device or resource busy`. The WFM shows "installed."

**With this SUP:**

```yaml
requiredCapabilities:
  - id: primaryCamera
    provider: capability.margo.org/hardware/camera
    scope: device
    request:
      interface: USB
      minimumResolution: 4K
    fallback:
      strategy: fail

parameters:
  - name: cameraDevicePath
    valueFrom: primaryCamera.result.devicePath
```

**Flow:**
1. WFM checks requiredCapabilities.
2. It sees `capability.margo.org/hardware/camera` is device scoped and supported by the device. Prepares the `desiredState`:
.....
3. Device agent pulls the desired state.
4. Device agent iterates over requiredCapabilities.
5. Device agent checks device camera usage locally
6. `cam-0` (4K) is allocated to `deployment-security-app-001`
7. `cam-1` is free but only 1080p — fails `minimumResolution: 4K`
8. Device agent rejects:
   ```
   DEVICE_ALREADY_ALLOCATED:
   "No camera meeting 4K requirement is currently available.
    cam-0 (4K) is allocated to deployment-security-app-001."
   ```
5. Operator decides: remove security app first, or deploy to a different device

**Result:** No runtime crash. No two apps fighting over `/dev/video0`. Hardware exclusivity enforced before deployment.

---

## Conformance

**`ApplicationDeployment`:**
- `requiredCapabilities[].id` MUST be unique within the `requiredCapabilities` array of a single `ApplicationDeployment`.
- `requiredCapabilities[].provider` MUST follow the `capability.<namespace>/<category>/<name>` URI convention defined in SUP 1.
- `parameters[].name` MUST match a parameter name declared in the associated `ApplicationDescription`.
- `parameters[].valueFrom` MUST reference a valid `id` in `requiredCapabilities` using the format `<id>.result.<field-name>`.

---

### Alternatives Considered

#### "Why not add `allocatedPorts`, `allocatedRoutes` directly to `DeviceCapabilitiesManifest`?"

Hardcoding resource types into the manifest schema makes a permanent architectural decision: the Margo spec owns the complete list of every resource type a device will ever need to track. Every new resource type — CAN Bus channels, FPGA partitions, fieldbus addresses — requires a TWG proposal, a spec version bump, and updates to all WFM and device agent implementations. The manifest becomes a kitchen sink that no single device will ever fully populate.

The Provider concept separates two concerns: the manifest schema stays stable, and the provider ecosystem grows independently via URI-based extensibility. WFMs ignore unknown provider URIs gracefully without requiring updates.

Furthermore, a flat `allocatedPorts` field cannot express the structured access descriptors that many resources require — MPS slot counts, NVLink topology, compute modes, VRAM fragmentation state etc... Whereas a typed schema with defined input and output fields can carry that information.

#### "Why not extend `requiredResources` in `ApplicationDescription`?"

`ApplicationDescription` is a vendor artifact — written once, and should solely focus on the application logic. It must be environment-agnostic. Putting `requiredPorts: [8080]` in `ApplicationDescription` hardcodes an environment assumption into a portable artifact. An operator deploying the same app to two devices cannot express different port assignments per deployment.

The existing parameter model is completely fine. The missing piece is not a new `requiredInfrastructure` kind of field in `ApplicationDescription` but a mechanism required at the deployment time so that the platform can be the *source* of the parameter values. That is exactly what `infrastructureBindings` in `ApplicationDeployment` (defined in SUP 3) provides.

#### "Analysis of Arne's `valueFrom` / `installContext` SUP"

Arne's SUP solves a real problem: **static device context injection**. The device knows its hostname, its preferred storage class, its data path. The app needs those values injected at deploy time without the operator having to discover and type them manually. The `valueFrom` + `installContext` mechanism is exactly the right answer for that.

But the `installContext` is a key-value dictionary of device identity facts, it cannot express:
1. What resources are already occupied on the device.
2. Who is using the resources.
3. Rulesets policies like: `port 8080 is reserved and shouldn't be used by anyone`.
4. Doesn't provide a schema definition of the dictionary, and mechanism to extend the schema.
These are some rulesets that the Arne's SUP doesn't clarify as of now.

**Note:** If we target portable edge application ecosystem where applications can be deployed across heterogeneous hardware without knowing device details, then Provider SUP is the model that aligns best with that goal. If we target deployments on known devices, then Arne's SUP are much cheaper solutions.

---

## Open Questions

**Q1: Multi-scope providers.** Some resources (e.g., network-attached storage) are neither purely device-scoped nor purely fleet-scoped. Should a provider be allowed to declare multiple scopes, or should multi-scope resources be modeled as separate providers?

**Q2: Provider URI versioning.** Should the URI include a version segment (e.g., `capability.margo.org/hardware/gpu/v2`) or should versioning be handled entirely within the `ProviderSchema`?

**Q3: Scope enforcement.** Should a WFM that receives a device-scope binding it cannot pre-validate (due to stale capability report) block the deployment or forward it to the device agent for authoritative resolution? This determines whether the WFM's pre-dispatch validation is a hard gate or an advisory check.

**Q4: Binding output timing.**
Should the Device Agent resolve all bindings before fetching any components (image pull, chart download), or may it fetch components in parallel with binding resolution? Earlier resolution enables faster config injection but adds latency before downloads begin. Later resolution wastes bandwidth if a binding ultimately fails.

**Q5: Multi-profile binding.**
The current `ApplicationDeployment` supports one `deploymentProfile` per document. If an operator wants to deploy both a Helm profile and a Compose profile to different devices from the same `ApplicationDescription`, should `requiredCapabilities` be scoped per profile within one document, or should separate `ApplicationDeployment` documents be created per profile?

[<- Home](part1-providers.md#top)