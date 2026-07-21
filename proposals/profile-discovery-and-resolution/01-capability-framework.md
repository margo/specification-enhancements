# Specification Update Proposal — Profile Discovery & Resolution Framework

**Status:** Draft
**Owner:** @singhmj-1
**Depends on:** `sup_bisect_device_manifest`

**Note:** This SUP establishes the mechanism by which applications discover and request profiles, and how the platform resolves those requests.

---

## Table of Contents

1. [Summary](#summary)
2. [Motivation](#motivation)
3. [Framework Concepts](#framework-concepts)
4. [Profile Definition](#profile-definition)
5. [Profile Discovery Request](#profile-discovery-request)
6. [Resolution Model](#resolution-model)
7. [Scope Extensibility](#scope-extensibility)
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
- Complex resolution logic is standardized and reusable
- Operators can now express preferences and constraints when a device has multiple resources of the same type

---

## Motivation

The Profile Framework defined what a profile is, and how platforms publish them. This framework defines *how* applications discover them and *how* the conflicts are prevented between applications.

### Problem 1 — Device-specific values are unknown at authoring time

A GPU device path, an available network port, or a vault secret path are only known by the platform at deployment time. Today, operators must manually discover and supply these values. If the value changes or becomes invalid, the deployment fails at runtime with unclear errors.

### Problem 2 — Resource conflicts are only detected at runtime

Two applications deployed to the same device both request port `8080` or hostname `app.example.com`. Only one can succeed. The conflict surfaces at runtime — through infrastructure errors, not through the Margo management plane.

### Problem 3 — Discovery logic is scattered and inconsistent

There is no standard mechanism for applications to declare "I need a GPU with 8 GiB VRAM" or "I need an available network port" or "I need a secret from the vault." Each implementation invents its own approach. No cross-implementation conflict detection is possible.

### Problem 4 — Operators cannot distinguish between resources

When a device has multiple GPUs or multiple storage volumes, the operator has no way to express preferences (e.g., "use the GPU with the most VRAM") or constraints. Discovery becomes trial-and-error.

---

## Framework Concepts

### Extending ProfileDefinition

As per [`sup_bisect_device_manifest`] :
> A `ProfileDefinition` is authored once — by Margo Community, a device vendor, or a WFM vendor — and registered with WFM. It establishes five things:

> - **What** this profile is — its URI, category, and description
> - **Who** owns the profile — scope : device-scoped (Device Agent)  [note: not introducing any other scopes as of now, keeping this for extensibility]
> - **What category** it belongs to — resource, interface, peripheral, service, or capability
> - **What format** the runtime data reporting must use

This SUP extends by adding one more aspect to it:

**How can applications discover a profile?** — a mechanism for apps to convey what they need, and platforms to resolve their requests as per a defined contract established in `ProfileDefinition`.

**Extended `ProfileDefinition` structure:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/peripherals/gpu",
  "description": "Profile Definition for GPU peripherals",
  "scope": "device",
  "category": "peripheral",
  "schemaVersion": "1.0.0",
  "platformStateSchema": {},
  "discoverySchema": {
    "mode": "generator",
    "requestSchema": {
      "type": "object",
      "properties": {
        "minVramGiB": {
          "type": "number"
        },
        "architecture": {
          "type": "string"
        }
      }
    },
    "outputSchema": {
      "type": "object",
      "properties": {
        "devicePath": {
          "type": "string"
        },
        "vramGiB": {
          "type": "number"
        }
      }
    },
    "failureCodes": [
      "NoGPUAvailable",
      "InsufficientVRAM"
    ]
  }
}
```

### Extended ProfileDefinition Fields

This SUP adds one new field (with sub-fields) to the `ProfileDefinition` established in `sup_bisect_device_manifest`:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `discoverySchema` | object | N | Extends `ProfileDefinition` with a discovery contract. Optional — profiles without it are not discoverable. Contains three sub-fields: `requestSchema`, `outputSchema`, and `failureCodes`. |
| `discoverySchema.requestSchema` | object | N | JSON Schema object defining the inputs an application may declare when requesting this profile. |
| `discoverySchema.outputSchema` | object | N | JSON Schema object defining the values the platform will produce upon successful resolution. For guard-mode discoveries, this SHOULD be an empty object `{}`. |
| `discoverySchema.failureCodes` | array | N | List of named failure codes the platform may return if resolution cannot succeed. |
| `discoverySchema.mode` | enum | Y (if discoverySchema present) | `generator` — platform produces values unknown at authoring time. `guard` — platform validates a known value rather than generating one. |

All other `ProfileDefinition` fields remain unchanged.

---

### Discovery Schema

The `discoverySchema` defines a **deferred request/response contract** between applications and the platform. It is not an API call — the application does not invoke anything at authoring time. Instead, it declares what it needs, and the platform fulfills that declaration at evaluation time.

**Three parts:**

#### requestSchema

The inputs an application declares when it needs this profile attribute. These are the application's requirements, not platform state.

**Example:**

```yaml
"requestSchema": {
  "type": "object",
  "properties": {
    "minVramGiB": {
      "type": "number"
    },
    "architecture": {
      "type": "string"
    }
  }
}
```

#### outputSchema

The values the platform will generate upon successful resolution. These values are unknown at authoring time and only exist after evaluation. For guard-mode discoveries, this SHOULD be an empty object `{}`.

**Example:**

```yaml
"outputSchema": {
  "type": "object",
  "properties": {
    "devicePath": {
      "type": "string"
    },
    "vramGiB": {
      "type": "number"
    }
  }
}
```

#### failureCodes

A set of named failure outcomes the platform may return if resolution cannot succeed. Each code is machine-readable and actionable.

**Example:**

```yaml
"failureCodes": [
  "NoGPUAvailable",
  "InsufficientVRAM"
]
```

## Discovery vs. Allocation Policy

**Important distinction:** Discovery is inherently a declarative inquiry, not a guarantee of reservation. The framework allows an application to declare its resource requirements; the platform then evaluates this declaration against the live infrastructure state.

The core specification defines only the resolution contract (inputs, outputs, and failure modes). It deliberately decouples resource resolution from lifecycle enforcement, and how the resolution is done are implementation defined, for example:

* Transient Resolution: A storage class discovery may simply return a metadata name without locking the underlying pool.
* Implicit Reservation: A network port discovery may mark the resolved port as occupied immediately upon evaluation.
* Exclusive Enforcement: A GPU discovery may trigger strict hardware isolation or exclusive runtime access.

Because resource types have fundamentally different lifecycles, the exact enforcement policy is deferred to the underlying platform implementation.

Note: Need some feedback/guidance/brainstorming on whether the resource resolution should be defined by Margo, otherwise there will not be a deterministic behaviour.

---

### ProfileDiscoveryRequest — A Materialization of the newly introduced discoverySchema

A **ProfileDiscoveryRequest** is authored by an operator inside an `ApplicationDeployment`. It is a materialization of the discovery schema defined in the `ProfileDefinition`. It is a **deferred declaration** — authored before deployment, resolved by the platform at evaluation time (when the deployment is triggered).

**New top-level key added to `ApplicationDeployment` object:**

```yaml
discoverProfiles:
  gpuForInference:                               # unique name within this deployment
    id: deviceprofile.margo.org/peripherals/gpu  # references ProfileDefinition URI
    request:                                     # conforms to discoverySchema.requestSchema
      minVramGiB: 8
      architecture: ampere
```

`discoverProfiles` is a top-level optional map in the `ApplicationDeployment` object. Each key is a name unique within that deployment and maps to a single `ProfileDiscoveryRequest` entry.

### The `valueFrom` Mechanism

Profile discovery outputs are unknown at authoring time. But since the contract is already defined, we can be assured that platforms will generate the output but not until the application deployment request is submitted. Hence, we need a way to inject these deferred values into application configuration, so let us introduce a mechanism of `valueFrom` (word borrowed from Arne's SUP): The `valueFrom` adds — a **deferred value binding** that references resolved outputs.

**`valueFrom` path syntax:** Paths use dot-notation in the form `discoverProfiles.<key>.output.<field>`, where:
- `<key>` is the unique name declared in `discoverProfiles`
- `<field>` is a top-level field name declared in `discoverySchema.outputSchema`
- Paths are case-sensitive
- Nested field paths are not supported in this version

If a `valueFrom` path references an output key or field that does not exist in the contract of the `ProfileDefinition.discoverySchema`, then the platform MUST reject the deployment.

**Usage in `ApplicationDeployment` object:**

```yaml
parameters:
  gpuDevicePath:
    valueFrom: discoverProfiles.gpuForInference.output.devicePath
    targets:
      - pointer: GPU_DEVICE_PATH
        components: ["ml-inference"]
```

Note: The binding is not evaluated at authoring time. It is resolved by the platform at evaluation time and the concrete value is injected into the parameter.

---

### Guard vs. Generator Discoveries

The `mode` field in `discoverySchema` formally declares whether a profile discovery is a guard or a generator. This allows the platform to determine evaluation behaviour programmatically without inferring intent from schema shape.

**Generator discovery** (`mode: generator`): The platform generates values the application needs.

- Examples: GPU device path, available port number, resolved secret value
- Uses `valueFrom` to inject outputs into application parameters
- `outputSchema` MUST declare all produced fields

**Guard discovery** (`mode: guard`): The platform just validates the values that application conveys.

- Examples: Hostname uniqueness, port availability, channel occupancy
- `outputSchema` SHOULD be an empty object `{}`
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
- Evaluated against the device's current `ProfileState`

### Evaluation Timing

1. **Authoring time** — Operator declares `ProfileDiscoveryRequest` with `valueFrom` references
2. **Pre-flight time (WFM)** — WFM validates request against `requestSchema` and may perform advisory checks using last known `ProfileState` as reported by the Device. This is upto the implementation.
3. **Evaluation time (Device Agent)** — Device Agent resolves device-scoped profiles against current `ProfileState`
4. **Injection time** — Resolved values replace `valueFrom` references in parameters
5. **Execution time** — Deployment executes with concrete values

### Atomicity

Resolution of all `discoverProfiles` entries within a single `ApplicationDeployment` is **atomic**: either all entries resolve successfully or the entire deployment is rejected. Partial resolution is not permitted. If any single `ProfileDiscoveryRequest` fails, the platform MUST:

- Reject the deployment before any component is installed
- Return a structured failure identifying which `profileDefinitionId` caused the failure and the applicable failure code

This aligns with the all-or-nothing batch semantics established in [`sup_bisect_device_manifest`] for `ProfileState` submission.

### Device-Scoped Resolution

The Device Agent resolves against its local `ProfileState`.

**Example `ProfileState` as per device:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/peripherals/gpu",
  "platformState": {
    "gpus": [
      {
        "manufacturer": "NVIDIA",
        "model": "A100",
        "vram": { "unit": "Gi", "value": 10 },
        "architecture": "Ampere"
      }
    ]
  }
}
```

**Application requests -- application-deployment.yaml:**

```yaml
discoverProfiles:
  gpu:
    id: deviceprofile.margo.org/peripherals/gpu
    request:
      minVramGiB: 8
      architecture: ampere
```

**Device Agent evaluation:**
- Scans available GPUs in `ProfileState`
- Finds `/dev/nvidia0` matches criteria
- Resolves to:
  ```yaml
  output:
    devicePath: /dev/nvidia0
    model: NVIDIA A100
    vramGiB: 40
  ```
- Injects `devicePath: /dev/nvidia0` into application parameter

**If no GPU matches:**
- Returns failure code: `InsufficientVRAM` or `NoGPUAvailable` via Deployment Status API
- Deployment is rejected before any component is installed
- WFM surfaces structured failure to operator

**How are device-level failures reported?**

Device-level resolution failures are surfaced through the Deployment Status Manifest. When a `ProfileDiscoveryRequest` fails during Device Agent evaluation, the deployment status is updated with a structured error containing the `profileDefinitionId` that caused the failure and the applicable failure code from `discoverySchema.failureCodes`.
```json
{
  "apiVersion": "deployment.margo.org/v1alpha1",
  "kind": "DeploymentStatusManifest",
  "deploymentId": "a3e2f5dc-912e-494f-8395-52cf3769bc06",
  "deviceId": "plant-alfa-zone1-edge01",
  "status": {
    "state": "failed",
    "error": {
      "category": "DISCOVERY", // newly added field - enum: DISCOVERY | GENERIC
      "code": "NoGPUAvailable",
      "message": "The device cannot meet the GPU requirements",
      "details": { // polymorphic field based on the category type
        "profileDefinitionId": "deviceprofile.margo.org/peripherals/gpu"
      }
    }
  }
}
```

### Conformance Requirements

A resolving actor (Device Agent or WFM) that declares support for a profile attribute MUST:

- Accept `ProfileDiscoveryRequest` inputs conforming to `discoverySchema.requestSchema`
- Produce outputs conforming to `discoverySchema.outputSchema` on success
- Return only failure codes declared in `discoverySchema.failureCodes`
- Never return a failure code not declared in the `ProfileDefinition`

---

## Scope Extensibility

This SUP defines only **device-scoped** resolution. The `scope` field in `ProfileDefinition` is intentionally designed to be extensible.

**Example of a future scope — Fleet (WFM-managed):**
Fleet-scoped profiles would represent resources managed at the fleet level rather than on individual devices — for example:

- Secrets backends (e.g., Vault)
- TLS certificate provisioning

In a fleet-scoped model, the WFM would resolve `ProfileDiscoveryRequest` entries against WFM-managed `ProfileState`, and inject resolved outputs into the deployment before dispatching it to the device.

**Why this SUP does not introduce fleet or other scopes:**
- Fleet-scoped resolution requires WFM-side `ProfileState` authoring and lifecycle semantics that are not yet defined
- Introducing fleet scope now would create normative obligations on WFM vendors before the contract is sufficiently specified
- Device-scoped resolution is the foundational building block; fleet scope builds on top of it, and then other options can also be.

---

## Use Cases

### Use Case 1 — GPU Discovery (Generator)

An ML inference application needs a GPU with at least 8 GiB VRAM and Ampere architecture.

**ProfileDefinition (written by Margo Community or vendors):**
```json
{
  "profileDefinitionId": "deviceprofile.margo.org/peripherals/gpu",
  "scope": "device",
  "category": "peripheral",
  "platformStateSchema": {...},
  "discoverySchema": {
    "mode": "generator",
    "requestSchema": {
      "type": "object",
      "properties": {
        "minVramGiB": { "type": "number" },
        "architecture": { "type": "string" }
      }
    },
    "outputSchema": {
      "type": "object",
      "properties": {
        "devicePath": { "type": "string" },
        "vramGiB": { "type": "number" }
      }
    },
    "failureCodes": ["NoGPUAvailable", "InsufficientVRAM"]
  }
}
```

**ApplicationDeployment:**
```yaml
discoverProfiles:
  gpuForInference:
    id: deviceprofile.margo.org/peripherals/gpu
    request:
      minVramGiB: 8
      architecture: ampere

parameters:
  gpuDevicePath:
    valueFrom: discoverProfiles.gpuForInference.output.devicePath
    targets:
      - pointer: GPU_DEVICE_PATH
        components: ["ml-inference"]
```

**Resolution outcome:** Device Agent finds a matching GPU, resolves `devicePath`, and injects it into the `GPU_DEVICE_PATH` parameter before the application starts.

---

### Use Case 2 — Port Availability (Guard)

A web server application needs port `8080` to be available on the device before deployment.

**ProfileDefinition:**
```json
{
  "profileDefinitionId": "deviceprofile.margo.org/network/port",
  "scope": "device",
  "category": "resource",
  "platformStateSchema": {...},
  "discoverySchema": {
    "mode": "guard",
    "requestSchema": {
      "type": "object",
      "properties": {
        "port": { "type": "number" },
        "protocol": { "type": "string" }
      },
      "required": ["port"]
    },
    "outputSchema": {},
    "failureCodes": ["PortUnavailable", "PortReserved"]
  }
}
```

**ApplicationDeployment:**
```yaml
discoverProfiles:
  httpPort:
    id: deviceprofile.margo.org/network/port
    request:
      port: 8080
      protocol: tcp
```

**Resolution outcome:** Device Agent checks whether port `8080` is available. If occupied, it returns `PortUnavailable` and rejects the deployment before any component is installed.

---

### Use Case 3 — Available Network Port (Generator)

An application needs any available port assigned dynamically rather than specifying one explicitly.

**ProfileDefinition:**
```json
{
  "profileDefinitionId": "deviceprofile.margo.org/network/dynamic-port",
  "scope": "device",
  "category": "resource",
  "platformStateSchema": {...},
  "discoverySchema": {
    "mode": "generator",
    "requestSchema": {
      "type": "object",
      "properties": {
        "protocol": { "type": "string" },
        "rangeMin": { "type": "number" },
        "rangeMax": { "type": "number" }
      }
    },
    "outputSchema": {
      "type": "object",
      "properties": {
        "availablePort": { "type": "number" }
      }
    },
    "failureCodes": ["NoPortAvailable"]
  }
}
```

**ApplicationDeployment:**
```yaml
discoverProfiles:
  servicePort:
    id: deviceprofile.margo.org/network/dynamic-port
    request:
      protocol: tcp
      rangeMin: 8000
      rangeMax: 9000

parameters:
  availablePort:
    valueFrom: discoverProfiles.servicePort.output.availablePort
    targets:
      - pointer: SERVICE_PORT
        components: ["web-server"]
```

**Resolution outcome:** Device Agent finds an available port in the declared range, resolves it, and injects the value into `SERVICE_PORT`, if none exists the deployment is rejected and a defined error is returned via `ApplicationDeploymentStatus` document.

---

## Design Decisions

### Why is `discoverySchema` optional on `ProfileDefinition`?

Not all profiles need to be discoverable. A `ProfileDefinition` may exist solely to standardize `ProfileState` reporting — for inventory purposes — without any application needing to request it. Making `discoverySchema` optional preserves this use case without forcing a discovery contract onto every profile.

### Why is resolution atomic?

Partial resolution creates ambiguous deployment states. If two out of three `ProfileDiscoveryRequest` entries resolve but the third fails, the platform cannot safely proceed — some parameters would be injected, others would not. Atomic rejection ensures the deployment either starts with all values resolved or does not start at all.

### Why are `valueFrom` nested paths not supported in this version?

Nested path resolution introduces complexity in schema validation and output traversal. Flat top-level field references cover the majority of practical use cases. Nested paths can be introduced in a future version once the base mechanism is proven.

### Why is `mode` a required field when `discoverySchema` is present?

Without `mode`, the platform must infer evaluation behaviour from schema shape — an unreliable heuristic. Explicit `mode` declaration makes platform behaviour deterministic and removes ambiguity for implementors.

---

## Open Questions

| # | Question | Status |
|---|---|---|
| 1 | Should `requestSchema` support `required` fields, or are all request inputs always optional? | Open |
| 2 | Should `failureCodes` support structured metadata (e.g., human-readable descriptions) in addition to machine-readable codes? | Open |
| 3 | Should `valueFrom` support a default value fallback if resolution produces no output for an optional field? | Open |