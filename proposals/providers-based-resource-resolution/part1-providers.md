# Specification Update Proposal — Providers-Based Resource Resolution - Part 1 - Provider Concept

<a name="home"></a>

> This proposal is split across three parts. **Part 1** (this document) establishes the Provider concept, URI convention, and schema structure. **Part 2** covers how devices advertise provider state to the WFM. **Part 3** defines how deployments consume providers and how resolution flows through the WFM and Device Agent.

## Table of Contents

1. [Summary](#summary)
2. [Reason for Proposal](#reason-for-proposal)
   - 2.1 [Triggering Problem](#triggering-problem)
3. [The Provider Framework](#the-provider-framework)
   - 3.1 [What a Provider Is](#what-a-provider-is)
   - 3.2 [Provider URI Naming Convention](#provider-uri-naming-convention)
   - 3.3 [Provider Scopes](#provider-scopes)
   - 3.4 [Provider Schema Structure](#provider-schema-structure)
   - 3.5 [Resolution Strategy Model](#resolution-strategy-model)
4. [Device Capabilities Extension](part2-device-capabilities.md#device-capabilities-extension) *(Part 2)*
   - 4.1 [Extended DeviceCapabilitiesManifest Schema](part2-device-capabilities.md#extended-devicecapabilitiesmanifest-schema)
   - 4.2 [ProviderAdvertisement Structure](part2-device-capabilities.md#provideradvertisement-structure)
   - 4.3 [Illustrative Provider Advertisements](part2-device-capabilities.md#illustrative-provider-advertisements)
   - 4.4 [Alternatives Considered](part2-device-capabilities.md#alternatives-considered-part2)
   - 4.5 [Open Questions](part2-device-capabilities.md#open-questions-part2)
5. [Deployment — Consuming Providers](part3-application-deployment.md#deployment--consuming-providers) *(Part 3)*
   - 5.1 [Key Architectural Decision](part3-application-deployment.md#key-architectural-decision)
   - 5.2 [ApplicationDescription — No Changes](part3-application-deployment.md#applicationdescription--no-changes)
   - 5.3 [ApplicationDeployment Extensions](part3-application-deployment.md#applicationdeployment-extensions)
   - 5.4 [WFM Pre-Dispatch Validation Flow](part3-application-deployment.md#wfm-pre-dispatch-validation-flow)
   - 5.5 [Device Agent Resolution Flow](part3-application-deployment.md#device-agent-resolution-flow)
   - 5.6 [Deployment Status Failure Codes](part3-application-deployment.md#deployment-status-failure-codes)
6. [Use Cases](part3-application-deployment.md#use-cases)
   - 6.1 [UC1: Port Conflict — Compose](part3-application-deployment.md#uc1-port-conflict--compose)
   - 6.2 [UC2: Ingress Hostname Conflict — Kubernetes](part3-application-deployment.md#uc2-ingress-hostname-conflict--kubernetes)
   - 6.3 [UC3: Exclusive Hardware Conflict — Camera](part3-application-deployment.md#uc3-exclusive-hardware-conflict--camera)
7. [Alternatives Considered](part2-device-capabilities.md#alternatives-considered)
8. [Conformance](#conformance)
9. [Open Questions](#open-questions)


---

## Owner

@singhmj-1

---

## Summary

This SUP introduces a **Provider framework**: a typed, URI-identified capability contract model that gives the platform a structured language for expressing what a device offers, what is currently allocated, what an application needs from the platform, and how conflicts should be resolved — before a deployment takes palce on device.

---

## Reason for Proposal

### The Triggering Problem

This SUP created a framework to solve `margo/specification#75 — Address problems exposing application endpoints` but later realized that it solves couple of additional scenarios as well.

The existing specification has these gaps:

1.**No declaration of resource intent.** Applications have no Margo way to 
  declare what exclusive capabilities they will acquire on a device, so 
  neither the WFM nor the Device can anticipate or prevent conflicts 
  before deployment begins.

2.**No dynamic value resolution.** Applications cannot receive 
  device-specific values they need at runtime — such as a GPU device 
  path or an available storage class — without the operator manually 
  discovering and entering them.

3.**Weak definition for Capabilities.** 
  The `DeviceCapabilitiesManifest` is a **static inventory model**. It tells very little information about the software/hardware capabilities. Most of the fields are scalar in nature and can't be used in real environment.

  For example:
  | What the existing spec can express | What it cannot express |
  |---|---|
  | "This device has a GPU" | "This device has Nvidia Tesla GPU with 20Gi VRAM, the GPU device path is /gpu/v0 etc..." |
  
4.**No extensibility.**
  The device capability model cannot be extended to support vendor-specific capability types without a spec change.

  For example: To add a `canbus` capability one would require to make addition to the DeviceCapabilitiesManifest definition, which will cause a bump in the spec version.
  This is completely unnecessary and should be decoupled.

5.**No way to convey device constraints/policies.**
  The existing specification uses OTEL for runtime observability 
  and it works well for resource availability metrices. However, OTEL has no concept 
  of device-level policies or constraints — it cannot express that 
  certain ports are reserved, that a specific network interface is 
  restricted to OT traffic only, etc.
  These are static rules that live on the device, not runtime metrics.

  Supporting this through OTEL would require extending existing 
  collectors and publishers to carry policy data — a purpose they 
  were not designed for. And anyways, that data would be needed by the device
  and the WFM to make decisions.
  
  We need a framework that keeps these concerns 
  separate: OTEL continues to handle runtime observability, while 
  `Providers` handle capability contracts, allocation state, and 
  device-level constraints at deploy time.

---

## Technical Proposal

> **⚠️ Illustrative Schemas — Not Normative**
>
> The provider examples(for eg, capability.margo.org/network/port, capability.margo.org/hardware/gpu) presented in this document are **illustrative only**. They exist to make the `Provider` concept concrete and reviewable. Their field names, strategies, constraints, and output structures are starting points for discussion, not normative definitions. Each provider must be formally defined via a SUP, and hence will have their own schema, with full use case documentation, TWG review, and conformance requirements.

### What a Provider Is

A **Provider** is a typed, named capability contract. It is not a service, a container, or a runtime component. It has no lifecycle of its own. A Provider defines:

1. **A contract** — a formal schema specifying input a consumer may provide, output values the platform will resolve, and failure codes that may be returned.
2. **An owner** — a specific scope (device or fleet) responsible for resolving the contract.
3. **An identity** — a URI that uniquely identifies the provider type across the Margo ecosystem.

### Provider URI Naming Convention

All providers are identified by a URI of the form:

```
capability.<namespace>/<category>/<name>
```

| Segment | Description | Example |
|---|---|---|
| `capability` | Fixed prefix for all Margo provider URIs | `capability` |
| `<namespace>` | Domain controlled by the defining organization | `margo.org`, `siemens.com` |
| `<category>` | Logical grouping of related providers | `network`, `storage`, `hardware`, `identity` |
| `<name>` | Specific provider within the category | `port`, `ingress`, `volume`, `gpu` |

**Official Margo providers** use the `margo.org` namespace and are ratified by the TWG:
_NOTE: These are just examples._

```
capability.margo.org/network/port
capability.margo.org/network/ingress
capability.margo.org/storage/volume
capability.margo.org/hardware/gpu
capability.margo.org/hardware/camera
```

**Custom vendor providers** use a namespace the vendor controls:

```
capability.siemens.com/canbus/channel
capability.acme-devices.io/fpga/partition
```

**Conformance:** A vendor defining a custom provider URI MUST control the domain used as the namespace. Using `margo.org` in a custom provider URI is not permitted.

### Provider Scopes

**Device-scope** providers are resolved by the Device Agent. The WFM MAY use cached device capability state for proactive pre-dispatch validation, but the Device Agent is the authoritative resolver. The WFM MUST NOT poll the device to query provider state.

**Fleet-scope** providers are resolved by the WFM. Resolved values are injected into the `ApplicationDeployment` before dispatch. The Device Agent receives no unresolved fleet-scope bindings.

### Provider Schema Structure

Each provider is formally defined by a `ProviderSchema` document. Individual provider SUPs will publish these documents.

```yaml
apiVersion: margo.org/v1alpha1
kind: ProviderSchema
metadata:
  uri: capability.<namespace>/<category>/<name>
  scope: device | fleet
  description: <human-readable description>

spec:
  # -----------------------------------------------------------------------
  # Each provider defines its own request, output, and failureCodes.
  # -----------------------------------------------------------------------
  request:
    - name: <field-name>
      type: string | number | boolean | enum
      required: true | false
      default: <value>
      description: <text>
      minimum: <number>
      maximum: <number>
      pattern: <regex>
      values: [<v1>, <v2>]
  fallbackStrategies:
    - <strategy-name>
  output:
    - name: <field-name>
      type: string | number | boolean
      description: <text>
  failureCodes:
    - code: <ERROR_CODE>
      description: <text>
      fatal: true | false
```

#### Example:

```yaml
apiVersion: margo.org/v1alpha1
kind: ProviderSchema
metadata:
  uri: capability.margo.org/hardware/gpu
  scope: device
  description: This Provider helps in discovering GPU capabilities from a device.
spec:
  request:
    - name: manufacturer
      type: enum
      values: [NVIDIA, AMD]
      required: false
      description: Will be used to find GPU on the device.
  fallbackStrategies:
    - none
    - nextAvailable
  output:
    - name: devicePath
      type: string
      description: The device path to mount into the your program to use the GPU.
  failureCodes:
    - code: RESOURCE_CONSTRAINTS_CANNOT_BE_MET
      description: No GPU matching the specified manufacturer is available.
      fatal: true
```

### Resolution Strategy Model

The provider authors are responsible for defining what fallbackStrategies their contract should have.