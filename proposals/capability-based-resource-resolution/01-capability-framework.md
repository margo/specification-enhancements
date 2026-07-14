# Specification Update Proposal — Profile Discovery & Resolution Framework

**Status:** Draft  
**Owner:** @singhmj-1  
**Depends on:** `sup_profile_state_framework`  
**Extracted from:** `sup_capability_definition_framework`

**Note:** This SUP establishes the mechanism by which applications discover and request profile attributes, and how the platform resolves those requests. It defines the discovery contract, evaluation semantics, and ownership model. It builds on the ProfileState Framework to enable applications to discover device-specific values at deployment time and prevent resource conflicts before deployment begins.

---

## Table of Contents

1. [Summary](#summary)
2. [Motivation](#motivation)
3. [Framework Concepts](#framework-concepts)
4. [Profile Definition](#profile-definition)
5. [Profile Discovery Request](#profile-discovery-request)
6. [Resolution Model](#resolution-model)
7. [Fleet-Scoped Profiles](#fleet-scoped-profiles)
8. [Ownership Model](#ownership-model)
9. [Use Cases](#use-cases)
10. [Design Decisions](#design-decisions)
11. [Open Questions](#open-questions)

---

## Summary

Applications deployed on edge devices frequently depend on platform-managed resources and services — GPUs, network ports, storage classes, secrets, API gateways — whose exact identifiers and availability are only known by the platform at deployment time.

This proposal establishes the **Profile Discovery & Resolution Framework** — a mechanism by which:

1. Application operators declare what profile attributes they need to discover
2. The platform resolves those declarations against runtime profile state
3. Resolved values are injected into application configuration
4. Resource conflicts are detected and prevented before deployment

This framework solves four concrete problems:

- Device-specific values must no longer be supplied manually by operators
- Resource conflicts between applications are now detected before deployment
- New profile types require no core specification changes
- Complex resolution logic is standardized and reusable

---

## Motivation

The ProfileState Framework defines *what* profile information is available. This framework defines *how* applications discover it and *when* conflicts are prevented.

### Problem 1 — Device-specific values are unknown at authoring time

A GPU device path, an available network port, or a vault secret path are only known by the platform at deployment time. Today, operators must manually discover and supply these values. If the value changes or becomes invalid, the deployment fails at runtime with unclear errors.

### Problem 2 — Resource conflicts are only detected at runtime

Two applications deployed to the same device both request port `8080` or hostname `app.example.com`. Only one can succeed. The conflict surfaces at runtime — through cryptic infrastructure errors, not through the Margo management plane. By then, partial deployments have occurred.

### Problem 3 — Discovery logic is scattered and inconsistent

There is no standard mechanism for applications to declare "I need a GPU with 8 GiB VRAM" or "I need an available network port" or "I need a secret from the vault." Each implementation invents its own approach. No cross-implementation conflict detection is possible.

### Problem 4 — Operators cannot distinguish between resources

When a device has multiple GPUs or multiple storage volumes, the operator has no way to express preferences (e.g., "use the GPU with the most VRAM") or constraints (e.g., "use an exclusive channel"). Discovery becomes trial-and-error.

---

## Framework Concepts

### ProfileDefinition

A **ProfileDefinition** is the authoritative schema for a profile attribute. It answers three questions:

**What is this profile attribute?** — its URI, scope (device or fleet), and description.

**What does the platform publish?** — the `sourceState` schema defining what the platform knows about this attribute.

**How can applications discover it?** — the `discovery` schema defining request inputs, output values, and failure outcomes.

**Structure:**

```yaml
apiVersion: margo.org/v1
kind: ProfileDefinition

metadata:
  id: deviceprofile.margo.org/peripherals/gpu

spec:
  scope: device | fleet
  description: "..."

  sourceState:
    schema:
      type: object
      properties:
        gpus:
          type: array
          # ...

  discovery:
    requestSchema:
      type: object
      properties:
        minVramGiB:
          type: number
        architecture:
          type: string

    outputSchema:
      type: object
      properties:
        devicePath:
          type: string
        vramGiB:
          type: number

    failureCodes:
      - NoGPUAvailable
      - InsufficientVRAM
      - GPUAlreadyAllocated
```

### ProfileDiscoveryRequest

A **ProfileDiscoveryRequest** is authored by an application operator inside an `ApplicationDeployment`. It expresses what the application needs the platform to discover about a specific profile attribute.

It is a **deferred declaration** — authored before deployment, resolved by the platform at evaluation time (when the deployment is triggered).

**Structure:**

```yaml
discoverProfiles:
  gpuForInference:                        # unique name within this deployment
    id: deviceprofile.margo.org/peripherals/gpu  # references ProfileDefinition URI
    request:                              # conforms to discovery.requestSchema
      minVramGiB: 8
      architecture: ampere
```

### The `valueFrom` Mechanism

Profile discovery outputs are unknown at authoring time. The `valueFrom` mechanism bridges this gap — a **deferred value binding** that references resolved outputs.

**Usage:**

```yaml
parameters:
  gpuDevicePath:
    valueFrom: discoverProfiles.gpuForInference.output.devicePath
    targets:
      - pointer: env.GPU_DEVICE_PATH
        components: ["ml-inference"]
```

The binding is not evaluated at authoring time. It is resolved by the platform at evaluation time and the concrete value is injected into the parameter.

---

## Profile Definition

### Purpose

A `ProfileDefinition` is the single authoritative document for a profile attribute type. It establishes identity, purpose, and discovery mechanism.

This is a definition document — it contains no runtime data. Other artifacts — `ProfileState`, `ProfileDiscoveryRequest` — are runtime materializations of this definition.

### Identity and Scope

Every profile attribute is uniquely identified by a URI. Examples:

```text
deviceprofile.margo.org/resource/cpu
deviceprofile.margo.org/peripherals/gpu
deviceprofile.acme.com/fieldbus/canbus
workloadprofile.margo.org/constraint/memory-limit
```

**Scope** determines who resolves discovery requests:

- **Device-scoped** — resolved by the Device Agent against local device state
- **Fleet-scoped** — resolved by the WFM against fleet-wide state

**Device-scoped examples:**
- `deviceprofile.margo.org/peripherals/gpu` — device agent selects a free GPU
- `deviceprofile.margo.org/interface/ethernet` — device agent selects an available port
- `deviceprofile.acme.com/fieldbus/canbus` — device agent selects a free CAN channel

**Fleet-scoped examples:**
- `deviceprofile.margo.org/security/secret` — WFM resolves from vault
- `deviceprofile.margo.org/network/ingress` — WFM manages ingress hostname allocation across fleet
- `workloadprofile.margo.org/certificate` — WFM provisions TLS certificates

### Source State Schema

The `sourceState` schema defines what the platform publishes about a profile attribute. It answers: *What information does the platform know about this?*

For a GPU, the platform knows:
- Device paths
- Model names
- VRAM capacity
- Current allocation state

For a vault service, the platform knows:
- Whether it's reachable
- What backend is configured

For a secret, the platform knows:
- Whether the vault is available

**Example:**

```yaml
sourceState:
  schema:
    type: object
    properties:
      gpus:
        type: array
        items:
          type: object
          properties:
            devicePath:
              type: string
            model:
              type: string
            vramGiB:
              type: number
            allocation:
              type: object
              properties:
                allocatable:
                  type: boolean
                model:
                  type: string
                  enum: [exclusive]
                claims:
                  type: array
                  items:
                    type: object
                    properties:
                      id:
                        type: string
                      resource:
                        type: string
```

### Discovery Schema

The `discovery` schema defines a **deferred request/response contract** between applications and the platform. It is not an API call — the application does not invoke anything at authoring time. Instead, it declares what it needs, and the platform fulfills that declaration at evaluation time.

**Three parts:**

#### requestSchema

The inputs an application declares when it needs this profile attribute. These are the application's requirements, not platform state.

**Example:**

```yaml
requestSchema:
  type: object
  properties:
    minVramGiB:
      type: number
      description: Minimum GPU VRAM required
    architecture:
      type: string
      description: Preferred GPU architecture (e.g., ampere, ada)
```

#### outputSchema

The values the platform will generate upon successful resolution. These values are unknown at authoring time and only exist after evaluation.

**Example:**

```yaml
outputSchema:
  type: object
  properties:
    devicePath:
      type: string
      description: Kernel device path of the assigned GPU
    model:
      type: string
      description: GPU model name
    vramGiB:
      type: number
      description: GPU memory in GiB
```

#### failureCodes

The exhaustive set of named failure outcomes the platform may return if resolution cannot succeed. Each code is machine-readable and actionable.

**Example:**

```yaml
failureCodes:
  - NoGPUAvailable
  - InsufficientVRAM
  - GPUAlreadyAllocated
  - UnsupportedArchitecture
```

### Discovery vs. Allocation

**Important distinction:** Discovery does not necessarily allocate or reserve a resource. It declares what the application needs the platform to discover, and the platform evaluates that declaration against current state.

The framework defines the contract — inputs, outputs, failure modes — but does not mandate what happens after resolution:

- A network port discovery may mark the port as occupied after resolution
- A GPU discovery may enforce exclusive access
- A storage class discovery may simply return a name with no locking
- A secret discovery may return a value with no allocation

The platform implementation chooses the enforcement policy appropriate to the resource type.

---

## Profile Discovery Request

### Structure

A `ProfileDiscoveryRequest` appears inside the `discoverProfiles` map of an `ApplicationDeployment`. Each entry is keyed by a name unique within that deployment.

```yaml
discoverProfiles:
  inferenceGPU:                                   # unique key within deployment
    id: deviceprofile.margo.org/peripherals/gpu   # references ProfileDefinition URI
    request:                                      # conforms to discovery.requestSchema
      minVramGiB: 8
      architecture: ampere
```

### Using Discovery Outputs

Resolved values are injected into application configuration via `valueFrom`:

```yaml
parameters:
  gpuDevicePath:
    valueFrom: discoverProfiles.inferenceGPU.output.devicePath
    targets:
      - pointer: env.GPU_DEVICE_PATH
        components: ["ml-inference"]
  gpuModel:
    valueFrom: discoverProfiles.inferenceGPU.output.model
    targets:
      - pointer: env.GPU_MODEL
        components: ["ml-inference"]
```

The `valueFrom` reference is not evaluated at authoring time. It is resolved by the platform at evaluation time, and the concrete value is injected before the deployment is applied.

### Guard vs. Generator Discoveries

**Generator discovery:** The platform produces values the application does not know at authoring time.

- Examples: GPU device path, available port number, resolved secret value
- Uses `valueFrom` to inject outputs
- Outputs are empty string if not generated

**Guard discovery:** The platform validates a known value rather than generating one.

- Examples: Hostname uniqueness, port availability, channel occupancy
- Uses `valueFrom` but outputs are empty or minimal
- Primary purpose is conflict detection before deployment

---

## Resolution Model

### How Resolution Works

Resolution combines three artifacts:

```text
ProfileDefinition
      +
ProfileState
      +
ProfileDiscoveryRequest
         ↓
   Resolution Engine
         ↓
   (Success / Failure)
      +
   Output Values (if success)
```

**Device-scoped resolution:**
- Triggered when a deployment is applied to a device
- Performed by the Device Agent
- Evaluated against the device's current ProfileState
- Resolution may update ProfileState (e.g., mark GPU as allocated)

**Fleet-scoped resolution:**
- Triggered when a deployment is dispatched by the WFM
- Performed by the WFM
- Evaluated against WFM-managed state
- WFM injects outputs into the deployment before sending to device

### Evaluation Timing

1. **Authoring time** — Operator declares `ProfileDiscoveryRequest` with `valueFrom` references
2. **Pre-flight time (WFM)** — WFM validates request against `requestSchema` and may perform advisory checks using last known `ProfileState`
3. **Evaluation time (WFM/Device)** — Platform resolves request against current state:
   - WFM resolves fleet-scoped profiles
   - Device Agent resolves device-scoped profiles
4. **Injection time** — Resolved values replace `valueFrom` references in parameters
5. **Execution time** — Deployment executes with concrete values

### Device-Scoped Resolution

The Device Agent resolves against its local `ProfileState`:

```yaml
# Device publishes this ProfileState
apiVersion: margo.org/v1
kind: ProfileState
metadata:
  entity: device-001
spec:
  id: deviceprofile.margo.org/peripherals/gpu
  spec:
    gpus:
      - devicePath: /dev/nvidia0
        model: NVIDIA A100
        vramGiB: 40
        allocation:
          allocatable: true
          model: exclusive
          claims:
            - id: ""  # unclaimed
              resource: /dev/nvidia0
      - devicePath: /dev/nvidia1
        model: NVIDIA A100
        vramGiB: 40
        allocation:
          allocatable: true
          model: exclusive
          claims:
            - id: deployment-video-001
              resource: /dev/nvidia1
```

**Application requests:**

```yaml
discoverProfiles:
  gpu:
    id: deviceprofile.margo.org/peripherals/gpu
    request:
      minVramGiB: 8
      architecture: ampere
```

**Device Agent evaluation:**
- Scans available GPUs in ProfileState
- Finds `/dev/nvidia0` matches criteria and is unclaimed
- Resolves to:
  ```yaml
  output:
    devicePath: /dev/nvidia0
    model: NVIDIA A100
    vramGiB: 40
  ```
- Updates ProfileState to mark `/dev/nvidia0` as allocated to this deployment
- Injects `devicePath: /dev/nvidia0` into application parameter

**If no GPU matches:**
- Returns failure code: `InsufficientVRAM` or `GPUAlreadyAllocated`
- Deployment is rejected before any component is installed
- WFM surfaces structured failure to operator

### Fleet-Scoped Resolution

The WFM resolves against fleet-managed state. Example: Secret resolution.

```yaml
# WFM publishes this ProfileState
apiVersion: margo.org/v1
kind: ProfileState
metadata:
  scope: fleet
spec:
  id: deviceprofile.margo.org/security/secret
  spec:
    available: true
    backend: hashicorp-vault
```

**Application requests:**

```yaml
discoverProfiles:
  databasePassword:
    id: deviceprofile.margo.org/security/secret
    request:
      secretName: production/db/password
      field: password
```

**WFM evaluation:**
- Connects to configured vault backend
- Retrieves `production/db/password`
- Extracts field `password`
- Resolves to:
  ```yaml
  output:
    value: s3cr3t-p@ssw0rd
  ```
- Injects value into application parameter before dispatch to device
- Device never sees the `valueFrom` reference or the secret

**If secret not found:**
- Returns failure code: `SecretNotFound` or `VaultUnavailable`
- Deployment is blocked entirely
- WFM surfaces structured failure to operator

### Conformance Requirements

A resolving actor (Device Agent or WFM) that declares support for a profile attribute MUST:

- Accept `ProfileDiscoveryRequest` inputs conforming to `discovery.requestSchema`
- Produce outputs conforming to `discovery.outputSchema` on success
- Return only failure codes declared in `discovery.failureCodes`
- Never return a failure code not declared in the `ProfileDefinition`
- Return a structured failure indicating unsupported profile URI; never silently ignore

---

## Fleet-Scoped Profiles

Fleet-scoped profiles are managed and resolved by the WFM, not individual devices. They represent fleet-wide state or services.

### Examples

**Networking/Ingress:**
```yaml
id: deviceprofile.margo.org/network/ingress
scope: fleet
description: "Manages ingress hostname allocation across the fleet"
```

**Certificates:**
```yaml
id: deviceprofile.margo.org/network/certificate
scope: fleet
description: "Provisions TLS certificates for deployments"
```

**Identity/Provisioning:**
```yaml
id: deviceprofile.margo.org/identity/service-account
scope: fleet
description: "Provisions service accounts for workloads"
```

### FleetProfileState

The WFM publishes `FleetProfileState` for attributes it manages:

**Security/Secrets:**
```yaml
id: wfmprofile.margo.org/security/secret
scope: fleet
description: "Resolves secrets from a centralized vault"
```

---

## Ownership Model

Each artifact has a clearly designated owner. No actor may produce an artifact it does not own.

### ProfileDefinition Authors

**Who:** Margo TWG members, device vendors, WFM vendors

**Responsibilities:**
- Define the profile URI
- Define the scope (device or fleet)
- Author the `sourceState` schema
- Author the `discovery` schema (requestSchema, outputSchema, failureCodes)
- Version the definition on breaking changes

**Rules:**
- MUST declare all possible `failureCodes`
- MUST use namespaced URIs for custom profiles
- Standard profiles use `margo.org` namespace
- Vendor profiles use vendor domain namespace

### Device Vendors

**Role:** Publish `ProfileState` for device-scoped profiles

**Responsibilities:**
- Discover resources at runtime (GPUs, interfaces, storage, etc.)
- Publish `ProfileState` conforming to `ProfileDefinition.sourceState`
- Update `ProfileState` when resource availability changes
- Implement resolution logic for device-scoped profiles
- Update allocation metadata when resources are claimed

### WFM Vendors

**Role:** Manage profile registry, validate requests, resolve fleet-scoped profiles

**Responsibilities:**
- Host and serve `ProfileDefinitions` from a registry
- Validate `ProfileDiscoveryRequest` inputs against `requestSchema`
- Evaluate fleet-scoped discovery requests
- Inject resolved values into deployments before device dispatch
- Publish `FleetProfileState` for managed profiles
- Block deployment if resolution fails
- Perform pre-flight advisory checks using last known device `ProfileState`

### Application Operators

**Role:** Declare profile discovery requirements

**Responsibilities:**
- Author `ProfileDiscoveryRequest` inside `ApplicationDeployment`
- Conform request inputs to `discovery.requestSchema`
- Reference generated outputs via `valueFrom`
- Handle failure codes in deployment fallback strategies
- Ensure requested profile URIs are supported by target devices/WFM

---

## Use Cases

### Use Case 1 — GPU Resource Discovery and Allocation

**Problem:** An AI inference workload requires a GPU. Device paths, models, and availability are unknown at deployment time.

**Solution:** Application operator uses GPU discovery to request a GPU with minimum VRAM and preferred architecture. Device Agent selects a suitable free GPU, updates allocation metadata, and injects device path into application.

**ProfileDefinition:**
```yaml
id: deviceprofile.margo.org/peripherals/gpu
scope: device
sourceState:
  # schema includes gpus array with allocation metadata
discovery:
  requestSchema:
    properties:
      minVramGiB: number
      architecture: string
  outputSchema:
    properties:
      devicePath: string
      model: string
      vramGiB: number
  failureCodes:
    - NoGPUAvailable
    - InsufficientVRAM
    - GPUAlreadyAllocated
```

### Use Case 2 — CAN Bus Channel Discovery (Vendor-Defined)

**Problem:** Industrial application needs a CAN bus channel. Multiple channels exist with different baud rates and protocols. Device agent must prevent two deployments from claiming the same channel.

**Solution:** Vendor defines profile under their namespace. Application operator requests channel with specific baud rate and protocol. Device Agent validates against current allocations and assigns unclaimed channel.

**ProfileDefinition:**
```yaml
id: deviceprofile.acme.com/fieldbus/canbus
scope: device
sourceState:
  # schema includes channels array with allocation metadata
discovery:
  requestSchema:
    properties:
      baudRate: integer
      protocol: string
  outputSchema:
    properties:
      channelId: string
  failureCodes:
    - NoChannelAvailable
    - BaudRateMismatch
    - ProtocolMismatch
    - ChannelAlreadyAllocated
```

### Use Case 3 — Secret Resolution from Vault

**Problem:** Application needs database password from vault. Operator must not manually inject secrets into deployment manifests.

**Solution:** WFM resolves secret request by connecting to vault backend. Resolved value is injected into deployment before it reaches device. Device never sees secret reference.

**ProfileDefinition:**
```yaml
id: deviceprofile.margo.org/security/secret
scope: fleet
sourceState:
  properties:
    available: boolean
    backend: string
discovery:
  requestSchema:
    properties:
      secretName: string
      field: string (optional)
      version: string (optional)
  outputSchema:
    properties:
      value: string
      resolvedVersion: string
  failureCodes:
    - SecretNotFound
    - VaultUnavailable
    - AccessDenied
    - InvalidSecretPath
```

### Use Case 4 — Hostname Allocation Guard

**Problem:** Two applications both request hostname `app.example.com`. Only one can own it. Conflict should be detected before deployment, not at runtime.

**Solution:** WFM validates hostname request against fleet-wide ingress state. If hostname is taken, deployment is rejected immediately. If available, allocation is recorded and deployment proceeds.

**ProfileDefinition:**
```yaml
id: deviceprofile.margo.org/network/ingress
scope: fleet
sourceState:
  properties:
    domain: string
    allocatedHostnames: array
discovery:
  requestSchema:
    properties:
      hostname: string
      tlsRequired: boolean
  outputSchema: {}  # guard only, no output
  failureCodes:
    - HostnameAlreadyAllocated
    - InvalidHostnameFormat
```

---

## Design Decisions

### Q1. Why separate `ProfileDefinition` from runtime `ProfileState` and `ProfileDiscoveryRequest`?

**Because the definition is authoritative and stable, while runtime state changes constantly.**

The `ProfileDefinition` is authored once and rarely changes. It establishes the contract: what can be requested, what will be output, what failures are possible.

`ProfileState` changes every time a resource becomes available or is allocated. Mixing them would force re-validating the definition on every state change.

`ProfileDiscoveryRequest` is authored by operators at deployment time. Coupling it to the definition would prevent independent versioning.

Separation keeps concerns clear: definition = contract, state = runtime reality, request = intent.

### Q2. Why use `valueFrom` instead of direct API calls?

**Because deployment-time values are not yet known, and we need to defer resolution to evaluation time.**

Applications cannot know GPU device paths or resolved secrets when they're being authored. Direct API calls would require runtime state inspection before authoring, which is impractical.

`valueFrom` is a **deferred binding** — a reference that says "fill this in at evaluation time." This defers resolution to when the platform has current state.

### Q3. Why exhaustive `failureCodes` instead of free-form error messages?

**Because free-form errors cannot be acted upon programmatically.**

If resolution fails with "sorry, no GPU available," the WFM cannot:
- Distinguish temporary conflicts from permanent hardware absence
- Trigger fallback strategies based on failure type
- Surface predictable errors to operators

Named, exhaustive `failureCodes` defined in `ProfileDefinition` mean:
- Every failure outcome is known upfront
- Application vendors can write explicit fallback strategies per code
- WFM can make scheduling decisions based on failure type
- Operators see consistent error surfaces across all profile types

### Q4. Why is device-scoped resolution authoritative at the device, not the WFM?

**Because the WFM's view of device state is always potentially stale.**

The device publishes `ProfileState` to the WFM periodically. By the time a deployment is triggered, that state may no longer reflect reality — a GPU might have been allocated, a channel might have become occupied.

The device agent resolves against its own current state at apply time, which is always accurate. The WFM's pre-flight check is a best-effort early warning, not an authoritative gate.

### Q5. Why do devices publish `ProfileState` to the WFM if the device resolves locally?

**Because the WFM needs it for reasons beyond resolution.**

Resolution is only one consumer. The WFM uses `ProfileState` for:

1. **Pre-flight validation** — Check if device is likely to satisfy request before dispatch
2. **Scheduling** — Select most suitable device from multiple candidates
3. **Operator visibility** — Surface device capabilities via dashboards and APIs
4. **Fleet-scoped resolution** — WFM needs to know state of resources it manages
5. **Conflict detection** — Track which profiles are occupied across deployments

---

## Open Questions

1. **Profile versioning** — How should breaking changes to `ProfileDefinition` be handled? Separate URIs? Numeric suffixes? Registry negotiation?

2. **Conflict resolution strategy** — Should the framework define strategies for resolving conflicts when multiple deployments request the same discrete resource? (First-come-first-served, priority, etc.)

3. **Partial resolution** — If an `ApplicationDeployment` has multiple `discoverProfiles` entries and one fails, should the entire deployment be rejected or should partial success be allowed?

4. **Conditional discovery** — Should profile discovery support conditional evaluation? (e.g., "only if this other profile resolution succeeded")

5. **Cross-device discovery** — Can applications discover profiles across multiple candidate devices? How does selection work?

6. **Async resolution** — Should resolution be purely synchronous, or support async/eventual resolution for resources that take time to provision (e.g., certificates)?

7. **Profile caching** — Should WFM cache profile resolutions, or always re-resolve? What cache invalidation strategy?
```

This captures all the discovery, resolution, and allocation concepts from the original Capability Framework, but:

- Uses ProfileState terminology consistently
- Builds on ProfileState Framework as foundation
- Removes device-specific examples (those stay in ProfileState Framework as instantiations)
- Focuses on the meta-framework for HOW discovery/resolution works
- Includes all the important design decisions and semantics
- Has use cases that demonstrate both device and fleet scoped patterns
- Opens questions for future refinement