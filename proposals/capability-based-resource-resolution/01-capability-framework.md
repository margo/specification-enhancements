# Specification Update Proposal — Capability Definition Framework

## Table of Contents

1. [Summary](#summary)
2. [Motivation](#motivation)
3. [Framework](#framework)
   - [Redefining a Capability](#redefining-a-capability)
   - [Model](#model)
4. [Capability Definition](#capability-definition)
   - [Identity and Scope](#identity-and-scope)
   - [Source State](#source-state)
   - [Discovery](#discovery)
5. [Runtime Materialization](#runtime-materialization)
   - [Capability State](#capability-state)
   - [Capability Request](#capability-request)
6. [Request Evaluation Model](#evaluation-model)
   - [How Evaluation Works](#how-evaluation-works)
7. [Ownership Model](#ownership-model)
   - [Capability Authors](#capability-definition-authors)
   - [Device Vendors](#device-vendors)
   - [WFM Vendors](#wfm-vendors)
   - [Application Operators](#application-operators)
8. [Extensibility](#extensibility)
9. [Schema Changes](#schema-changes)
10. [Use Cases](#use-cases)
11. [Design Decisions](#design-decisions)
12. [Open Questions](#open-questions)

## Summary

Applications deployed on edge devices frequently depend on platform-managed 
concerns — GPUs, network ports, storage classes, API gateways etc. — whose   
availability, occupancy, exact identifiers and hierarchies are only known by the
platform at deployment time. The current specification has no mechanism to
express, discover, or resolve these concerns in a structured way.

This results in four concrete gaps: device-specific values must be supplied 
manually by operators; resource conflicts between applications are only detected
at runtime; adding new resource types requires modifying the core specification;
and complex resources cannot be modeled beyond flat boolean or string fields.

This framework solves that problem by rethinking the capabilities from ground-up.

---

# Motivation

The current specification is :

* **Unable to model complex platform resources.**
  A flat capability field like `"gpu": true` cannot express that a GPU has 
  40 GiB VRAM, is available at `/dev/nvidia0`, and is currently allocated to 
  another deployment. Structured, typed schemas are required.

* **Unable to extend the capability framework without modifying the core 
specification.**
  Adding support for a new resource type — such as a CAN Bus channel or an 
  FPGA partition — requires changes directly to the `DeviceCapabilitiesManifest`
  schema, and there is no framework as of now to extend this without modifying 
  it.

* **Unable to resolve device-specific values at deployment time.**
  A GPU device path, an available network port, or a storage class name are 
  only known by the platform at deployment time. Today, operators must discover 
  and supply these values manually.

* **Unable to prevent resource conflicts before deployment.**
  Two applications defaulting to port `8080` on the same device will not 
  conflict until one of them crashes at runtime. There is no mechanism as of now
  to detect or prevent this before deployment begins.


This proposal establishes a uniform mechanism for defining and discovering 
capabilities.

---

# Framework

## Redefining a Capability

So far, the definition of a capability is restrictive, but this framework 
expands it:

> A capability represents wide variety of **platform-managed concerns** 
including the following:
> * "Hardware resources" such as GPUs, cameras, fieldbus channels etc.
> * "Software capabilities" such as persistent storage, pre-installed 
runtimes etc.
> * "Platform services" such as api gateways, security vaults, otel collectors 
endpoints, pki, message buses etc.

---

## Model

A capability is defined once through an authoritative definition and is
subsequently materialized by different actors throughout its lifecycle. Its 
sole purpose is to establish identity, purpose, and discovery mechanism for a
capability.


```text
                    CapabilityDefinition
                              │
                              │
                    Defines The Rules
                              │
                              ▼

         ┌─────────────────────────────────────┐
         │      Capability Type Semantics      │
         └─────────────────────────────────────┘

                    ▲                 ▲
                    │                 │

                    │                 │

            CapabilityState     CapabilityDiscoveryRequest
```

The rest of this document defines each of these artifacts in detail, starting 
first with the `CapabilityDefinition` itself.

---

# Capability Definition

## Purpose

A `CapabilityDefinition` is the single authoritative document for a capability 
type. It is authored once — by a Margo specification author or a vendor.

It answers three questions:

**What is this capability?** — its URI, scope, and description.

**What does the platform publish about it?** — the state that devices or the WFM
 populate at runtime.

**How does an application discover it?** — the discovery schema defining inputs,
 outputs, and failure outcomes.

This is just a definition document and hence contains no runtime data.
But other artifacts mentioned below in this framework — `CapabilityState`, 
`CapabilityDiscoveryRequest` — are the runtime materialization of this definition.

## Structure

```yaml
apiVersion: margo.org/v1
kind: CapabilityDefinition

metadata:
  id: capability.margo.org/network/port

spec:

  scope: device | fleet

  description: |
    Provides network port information, and a discovery schema for their 
    selection and conflict detection.

  sourceState:

    schema:
      ...

  discovery:

    requestSchema:
      ...

    outputSchema:
      ...

    failureCodes:
      ...
```

## Identity and Scope

### Unique Identity

Every capability is uniquely identified by a URI across the Margo ecosystem.

URI Example:

```text
capability.margo.org/network/port
capability.margo.org/hardware/gpu
capability.margo.org/storage/class
```

### Capability Scopes

- Device-scoped capabilities are resolved by the Device Agent.

- Fleet-scoped capabilities are resolved by the WFM.

_Note: The examples are documented separately for these scope types._

## Source State

The source state schema defines the shape of information that platform actors 
will publish, i.e. it defines the schema of the data that the capability's 
owning actor — a device or the WFM — publishes to describe the current condition
of this capability. This may include what is available, what is already 
occupied, or any meta data about them. The schema is fixed by the
**CapabilityDefinition Author**; whereas the runtime values conforming to this
schema are published through `CapabilityState`, described in 
[Runtime Materialization](#runtime-materialization).

Example:

```yaml
sourceState:
  schema:
    type: object
    properties:
      occupiedPorts:
        type: array
        items:
          type: integer
```

---

## Discovery Schema

The discovery schema defines a **deferred request/response contract** between 
an application and the platform. It is not an API call — the application does 
not invoke anything at authoring time. Instead, it declares what it needs, and 
the platform fulfills that declaration at evaluation time, which occurs when a 
deployment is triggered against a specific device or fleet.

Example:
```yaml
apiVersion: margo.org/v1
kind: CapabilityDefinition

metadata:
  id: capability.margo.org/network/port

spec:
  ...

  discovery:
    requestSchema:
      type: object
      properties:
        preferredPort:
          type: integer
        findNextAvailable:
          type: boolean
    outputSchema:
      type: object
      properties:
        availablePort:
          type: integer
    failureCodes:
      - PortUnavailable
      - NoFreePortFound
```

The term **discovery** is deliberate. The application is not requesting an 
allocation or a reservation. It is declaring what it needs to discover about the
platform, and the platform resolves that declaration against real runtime state
when the time comes.

It has three parts that together describe this request and deferred response 
cycle:

**requestSchema** — the inputs an application declares when it needs this 
capability. These are the application's requirements, not platform state.

**outputSchema** — the values the platform will generate upon successful 
resolution. These values are unknown at authoring time and only exist after 
evaluation.

**failureCodes** — the exhaustive set of named failure outcomes the platform 
may return if resolution cannot succeed. Each code is machine-readable and 
actionable.

Applications materialize this contract through `CapabilityDiscoveryRequest`, 
described in [Runtime Materialization](#runtime-materialization).

> **Discovery is not Allocation.**
>
> The discovery does not reserve or allocate a resource. It declares
> what the application needs the platform to resolve. The platform evaluates that
> declaration against current state and produces an output — but the framework itself
> takes no position on whether that evaluation constitutes a reservation, a soft
> preference, or a hard allocation.
>
> That distinction is intentionally left to the capability implementation. A network
> port capability may choose to mark a port as occupied after resolution. A GPU
> capability may choose to enforce exclusive access. A storage class capability may
> simply return a name with no locking at all. The framework defines the contract —
> not the enforcement policy.
>
> This keeps the framework free of provisioning semantics and allows capability
> authors to define enforcement behavior appropriate to their resource type.
>
> Capability resolution may return an available resource with no allocation, perform a soft reservation, or perform a hard allocation. The framework does not require or enforce any of these. Conflict prevention is best-effort only.

---

# Runtime Materialization

## Capability State - Owned by Device/WFM

CapabilityState is a materialized contract derived from the 
`CapabilityDefinition.sourceState`. It represents runtime information published 
by the platform actor (device or wfm) and is descriptive. It communicates what 
the actor currently knows about the capability.

Example:

```yaml
apiVersion: margo.org/v1
kind: CapabilityState

metadata:
  capability: capability.margo.org/network/port

spec:
  occupiedPorts:
    - 22
    - 80
    - 443
    - 8080
  availableRangeOfPorts:
    - [11, 1000]
```
---

## Capability Discovery Request — Owned by Application Operators

### What Is It?

A `CapabilityDiscoveryRequest` is the runtime materialization of a 
`CapabilityDefinition.discovery` contract. It is authored by an application 
operator inside an `ApplicationDeployment` and expresses what the application 
needs the platform to resolve about a specific capability.

It is a declaration of intent — authored before deployment, fulfilled by the 
platform at evaluation time.

### Structure

A `CapabilityDiscoveryRequest` appears inside the `discoverCapabilities` map 
of an `ApplicationDeployment`. Each entry is keyed by a name that is unique 
within that deployment.

```yaml
discoverCapabilities:
  webPort:                                      # unique name within this deployment
    id: capability.margo.org/network/port       # references CapabilityDefinition URI
    request:                                    # conforms to discovery.requestSchema
      preferredPort: 8080
      findNextAvailable: true
```

---

### How to use the response of the discovery?

By using `valueFrom` semantics in the application deployment manifest.

> ### The `valueFrom` Mechanism — A Brief Introduction
> The discovery schema defines outputs the platform will
> produce at evaluation time — values like an assigned port number, a GPU device 
> path, or a resolved secret. These values do not exist when the application
> operator authors the deployment. They only exist after the platform evaluates
> the request.
> 
> `valueFrom` is the mechanism that bridges this gap. It is a 
> **deferred value binding** — a reference that says:
> 
> *"When the platform resolves this capability request, take the output named
> here and place it into this parameter."*
> 
> It is written as a dotted path that identifies the source:
> 
> ```yaml
> # General form
> valueFrom: discoverCapabilities.<requestName>.output.<outputField>
> 
> # Example — reference the port assigned by the webPort request
> valueFrom: discoverCapabilities.webPort.output.availablePort
> ```
> 
> The binding is not evaluated at authoring time. It is resolved by the platform 
> — Device Agent or WFM — at evaluation time.

---

# Evaluation Model

## How Evaluation Works

Device-scoped capabilities are resolved by the Device Agent. Fleet-scoped 
capabilities are resolved by the WFM.

The WFM MAY perform a pre-flight check using the last known CapabilityState 
published by the device — but this check is advisory. It does not substitute for
 the device agent's authoritative resolution at apply time.


## Capability Evaluation

Capability evaluation combines:

```text
CapabilityDefinition
         +
CapabilityState
         +
CapabilityDiscoveryRequest
```

The framework defines the **contract** for evaluation — its inputs, outputs, and
failure modes. It does not define a universal evaluation engine. Evaluation 
logic is implemented by the resolving actor for each capability it supports.

This means:

- A Device Agent that supports `capability.margo.org/hardware/gpu` contains the 
logic to match a `CapabilityDiscoveryRequest` against its local GPU capability
and produce a `devicePath` output.
- A WFM that supports `capability.margo.org/security/secret` contains the logic 
to connect to a vault backend and resolve a secret by name.

Evaluation may produce:

* Success
* Failure
* Generated outputs

Evaluation occurs before deployment execution.

---

### Conformance Requirement

A resolving actor that declares support for a capability URI MUST:

- Accept `CapabilityDiscoveryRequest` inputs conforming to the capability's `requestSchema`
- Produce outputs conforming to the capability's `outputSchema` on success
- Return only failure codes declared in the capability's `failureCodes` on failure
- Never return a failure code not declared in the `CapabilityDefinition`

A resolving actor that does not support a capability URI MUST return a
structured failure indicating the capability is unsupported. It MUST NOT 
silently ignore the request or proceed with deployment.

---

# Ownership Model

Each artifact in this framework has a clearly designated owner. No actor may 
produce an artifact it does not own.

## Capability Definition Authors

### Who is this?
- Margo TWG members
- Device vendors
- WFM vendors

### Responsibilities
- Define the URI
- Define the scope (device | fleet)
- Author the sourceState schema
- Author the discovery schema (request, output, failureCodes)
- Version the definition on breaking changes

### Rules
- MUST declare possible failureCodes
- MUST use namespaced URIs for custom capabilities

## Device Vendors

### Role in the Framework
Device vendors are responsible for publishing `CapabilityState`
for every capability their device supports.

### Responsibilities
- Discover hardware resources at runtime
- Publish `CapabilityState` conforming to `CapabilityDefinition.sourceState`
- Update `CapabilityState` when resource availability changes
- Enforce resolution logic for device-scoped capabilities

## WFM Vendors

### Role in the Framework
WFM vendors are responsible for:
- Maintaining the Capability Registry [TBD]
- Validating `CapabilityDiscoveryRequests` against `CapabilityDefinitions`
- Resolving fleet-scoped capabilities
- Forwarding device-scoped resolution to Device Agents

### Responsibilities
- Host and serve CapabilityDefinitions from Capbility Registry
- Validate incoming `CapabilityState` from devices against the Definitions
- Evaluate fleet-scoped `CapabilityDiscoveryRequests`
- Block deployment if resolution fails

## Application Operators

### Role in the Framework
Application vendors declare capability requirements inside
ApplicationDescriptions. They do not interact with platform
state directly.

### Responsibilities
- Declare `CapabilityDiscoveryRequests` referencing capability URIs
- Define request inputs conforming to `requestSchema`
- Reference generated outputs via inject mappings (i.e. `valueFrom`)
- Handle failure codes in fallback strategies

---

# Extensibility

Please read [this document](02-extensibility.md#extensibility) to understand
how this framework introduces extensiblilty.

---

# Use Cases:

The example use-cases can be found under `use-cases/` directory. 

---

# Schema Changes

The schema changes are documented under `schema-changes/` directory.

---

# Design Decisions

### Q1. Why are `sourceState` and `discovery` defined in the same
`CapabilityDefinition`? Why not separate documents?

**Because they are two sides of the same contract.**

`sourceState` defines what the platform knows about a capability.
`discovery` defines what an application can ask about that same capability. 
They are semantically coupled — you cannot author a valid 
`discovery.requestSchema` without knowing what the platform will publish in 
`sourceState`, and you cannot validate a `CapabilityDiscoveryRequest` without 
knowing both.

Separating them into two documents would:
- Allow them to drift out of sync independently
- Require a join operation at evaluation time across two separately versioned 
documents
- Create ambiguity about which version of `discovery` applies to which version 
of `sourceState`

Binding them in one document makes the coupling explicit and keeps versioning 
atomic — a breaking change to either side produces a new version of the whole 
definition.

---

### Q2. Why a URI as the identifier? Why not a simple name or integer ID?

**Because capabilities need to be globally unique, human-readable, and 
namespace-safe across vendors.**

A simple name like `gpu` is ambiguous — a device vendor's GPU capability and a 
platform vendor's GPU capability would collide. An integer ID requires a central
 registry to assign and track IDs.

A URI like `capability.margo.org/hardware/gpu` is:
- **Globally unique** — namespaced by domain ownership
- **Vendor-extensible** — a vendor can introduce 
`capability.vendor.com/hardware/fpga` without any central coordination

---

### Q3. Why are `failureCodes` exhaustive and defined upfront? Why not let the 
platform return free-form error messages?

**Because free-form errors cannot be acted upon programmatically.**

If the platform returns `"sorry, no GPU available"`, the WFM cannot:
- Distinguish between a temporary conflict and a permanent hardware absence
- Trigger a fallback strategy based on the specific failure reason
- Surface a structured, actionable error to the operator

Exhaustive, named `failureCodes` defined in the `CapabilityDefinition` mean:
- Every possible failure outcome is known at authoring time
- Application vendors can write explicit fallback strategies per code
- The WFM can make scheduling decisions based on failure type
- Operators see consistent, predictable error surfaces across all capability types

---

### Q4. Why is device-scoped resolution authoritative at the device and not at 
the WFM?

**Because the WFM's view of device state is always potentially stale.**

The device publishes CapabilityState to the WFM periodically. By the time
a deployment is triggered, that state may no longer reflect reality — a port
may have become occupied, a GPU may have been allocated to another process.

The device agent resolves against its own current state at apply time, which
is always accurate. The WFM's pre-flight check is a best-effort early warning,
not an authoritative gate.

---

### Q5. If the device resolves device-scoped capabilities itself, why does it 
publish CapabilityState to the WFM at all?

**Because the WFM needs it for reasons beyond resolution.**

Resolution is only one consumer of `CapabilityState`. The WFM uses it for:

1. **Pre-flight validation** — Before sending a deployment, the WFM can check 
whether the device is even likely to satisfy the request. If the last known 
state shows no GPU at all, the WFM can fail fast and surface the error to the 
operator immediately — without waiting for the device to pull and reject the 
deployment.

2. **Scheduling decisions** — When a deployment can target multiple devices,
the WFM uses published `CapabilityState` to select the most suitable device.
Without it, scheduling is blind.

3. **Operator visibility** — Operators need to see what capabilities each device
has. The WFM is the management plane — it surfaces device state to dashboards, 
CLIs, and APIs. Without `CapabilityState`, the operator has no visibility into 
what the fleet can support.

4. **Fleet-scoped resolution** — Fleet-scoped capabilities are resolved entirely
by the WFM. It cannot do this without knowing the state of the resources it 
manages.

5. **Conflict detection across deployments** — The WFM tracks which capabilities
are occupied across all deployments on a device. Without published state, it 
cannot detect cross-deployment conflicts even at the advisory level.

The device's local resolution at apply time is authoritative — but the WFM's 
access to `CapabilityState` is what makes the system observable, schedulable, 
and operable.

### Q6. Should the specification define concurrency and locking semantics?

**Short Answer:** Not in this version.

This framework does not define:
- Locking models
- Reservation guarantees
- Conflict resolution strategies under concurrent evaluation

These concerns are highly dependent on:
- Capability type (GPU vs port vs DNS)
- Platform architecture
- Deployment model

Instead, the spec defines a deterministic input/output contract for capability 
evaluation.

Concurrency handling is left to implementations.

Future versions MAY standardize optional concurrency patterns.

### Q7. How is versioning handled?

Versioning is not defined in this version of the specification.

Rationale:
- The framework itself is still evolving
- Premature standardization of versioning may constrain extensibility

Current expectation:
- CapabilityDefinitions are versioned out-of-band (e.g. URI suffix, registry 
versioning)

Future versions of this specification will formalize:
- Version negotiation
- Backward compatibility guarantees

---

# Open Questions

1. Schema versioning.
2. Registration and distribution of custom capabilities with WFM.