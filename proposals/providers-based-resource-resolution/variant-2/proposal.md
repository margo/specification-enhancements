# Specification Update Proposal — Providers-Based Resource Resolution

## Summary

This proposal introduces a Provider framework that has four core entities:

| Entity             | Purpose                    |
| ------------------ | -------------------------- |
| Provider           | Contract definition        |
| ProviderState      | Runtime state advertisement|
| ProviderBinding    | Deployment request         |
| ProviderResolution | Resolution outcome         |

Together these entities provide a common mechanism for reporting, discovery, validation, and resolution of platform-managed concerns across the Margo ecosystem. The framework also introduces a URI-based extension model that allows new provider types to be defined and adopted independently of the core specification.

---

# 1. Motivation

The current specification lacks a standardized mechanism for:

* Declaring platform-managed functionalities required by an application.
* Resolving device-specific values at deployment time.
* Preventing allocation conflicts before deployment.
* Extending capability framework without modifying the core specification.

As a result, applications frequently rely on operator-supplied values, runtime failure detection, or implementation-specific extensions.

This proposal introduces a Provider framework that addresses these gaps through a common model.

---

# 2. Provider Framework

## Core Concepts

The framework consists of four entities:

```text
Provider
    ↓ defines schemas

ProviderState
    ↓ advertises state

ProviderBinding
    ↓ requests a functionality

ProviderResolution
    ↓ returns the outcome
```

### Provider

A Provider is a first-class entity that defines a typed contract for advertising, and resolving platform-managed concerns.

A Provider defines:

* Identity
* Scope
* Binding Schema
  * Request schema
  * Output schema
  * Failure codes
* State schema

A Provider has no runtime lifecycle. It is a static contract that defines how a particular provider type is advertised, requested, validated, and resolved.

### Provider Types

Providers may be used to model a wide variety of platform-managed concerns.

Some providers represent hardware resources such as GPUs, cameras, or fieldbus channels.

Some providers represent software capabilities such as persistent storage.

Some providers represent platform services such as ingress, certificates, identities, or DNS names.

Some providers are resolved directly by a device, while others are resolved by fleet-level platform services.

The Provider framework intentionally treats all of these uniformly through a common binding and resolution model.

The applications declare requirements through `ProviderBindings` and receive resolved values through `ProviderResolutions` regardless of the underlying implementation.

Note: Scope based provider use cases are covered in the `use-cases.md` file.

---

## Provider Definition

A Provider is the authoritative type definition for a platform-managed concern/functionality.

```yaml
apiVersion: margo.org/v1alpha1
kind: Provider

metadata:
  uri: provider.<namespace>/<category>/<name>
  scope: device | fleet

bindingSchema:
  request: ...
  output: ...
  failureCodes: ...
  fallbackStrategies: ...

stateSchema:
  offer: ...
  allocation: ...
```

### Provider URI Convention

All providers are identified by:

```text
provider.<namespace>/<category>/<name>
```

Examples:

```text
provider.margo.org/network/port
provider.margo.org/network/ingress
provider.margo.org/storage/volume
provider.siemens.com/canbus/channel
```

### Provider Scopes

Device-scoped providers are resolved by the Device Agent.

Fleet-scoped providers are resolved by the WFM.

---
!!!!!! TODO: !!!!!!
## Provider Registry

The WFM maintains a Provider Registry containing all known Provider definitions.

Official Margo providers are distributed as part of specification conformance.

Custom providers may be registered dynamically through ProviderState advertisements.

The registry serves as the authoritative source for validation and authoring.

---

# 3. Provider Advertisement

## Extending DeviceCapabilitiesManifest

Devices advertise provider state through the existing DeviceCapabilitiesManifest.

No new API endpoints are introduced.

```yaml
providers:
  - provider: provider.margo.org/network/port
    supported: true
    offer: {}
    allocation: {}
```

Each entry in the providers array is a `ProviderState`.

---

## ProviderState

`ProviderState` represents the current state of a provider on a device.

| Field      | Description                             |
| ---------- | --------------------------------------- |
| provider   | Provider URI                            |
| supported  | Whether the provider is supported on this device |
| schemaURL  | Optional: Location of a custom provider definition     |
| offer      | Provider-specific description of available functionality |
| allocation | Provider-specific description of current allocations    |

### Offer

Offer describes what the device can provide and the policies that govern usage.

Examples include:

* Available capabilities/resources/services
* Supported features
* Reserved ranges

### Allocation

Allocation describes resources or capabilities currently assigned to deployments.

---

## ProviderState Lifecycle

The Device Agent is responsible for maintaining `ProviderState` and ensuring that it accurately reflects both:

* The provider's advertised `offer`.
* The provider's current `allocation`.

The Device Agent MUST publish an updated `ProviderState` whenever either of these changes.

The WFM consumes `ProviderState` advertisements to perform validation and deployment planning.

The WFM MUST NOT query or poll devices for provider state. All provider state updates are initiated by the device.


---

# 4. Provider Consumption

## ProviderBinding

Applications consume capabilities through ProviderBindings declared in ApplicationDeployment.

```yaml
providerBindings:
  - id: webPort
    provider: provider.margo.org/network/port

    request:
      port: 8080

    fallback:
      strategy: nextAvailable

    inject:
      - parameter: hostPort
        from: result.availablePort
```

A ProviderBinding declares:

* Which provider is being consumed.
* What functionality is requested.
* Which fallback strategy is acceptable.
* How resolved values are injected into the deployment.

---

## Resolution Flow

```text
ApplicationDeployment
    ↓

ProviderBinding
    ↓

WFM validation
    ↓

Provider resolution
    ↓

ProviderResolution
```

### WFM Responsibilities

The WFM MUST:

* Validate ProviderBindings.
* Validate references against the Provider Registry.
* Resolve fleet-scoped providers.
* Dispatch device-scoped providers.

### Device Agent Responsibilities

The Device Agent MUST:

* Resolve device-scoped providers.
* Enforce device policies.
* Update ProviderState.
* Report ProviderResolution results.

---

## ProviderResolution

ProviderResolution records the outcome of a ProviderBinding.

```yaml
providerResolutions:
  - bindingId: webPort
    status: resolved

    result:
      availablePort: 8083
```

ProviderResolution provides:

* Traceability
* Auditability
* Structured failure reporting
* WFM synchronization

---

# 5. Example

The following example requests a network port.

```yaml
providerBindings:
  - id: webPort

    provider: provider.margo.org/network/port

    request:
      port: 8080

    fallback:
      strategy: nextAvailable
```

If port 8080 is unavailable:

```yaml
providerResolutions:
  - bindingId: webPort

    status: resolved

    result:
      availablePort: 8083
```

The deployment receives the resolved value without runtime conflict.

---


```text
┌─────────────────────────────────────────────────────────────┐
│                        PROVIDER                            │
│-------------------------------------------------------------│
│ URI                                                        │
│ Scope (device | fleet)                                     │
│ Binding Schema                                             │
│ State Schema                                               │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ defines
                              ▼

         ┌─────────────────────────────────────┐
         │         Provider Registry           │
         │          (maintained by WFM)        │
         └─────────────────────────────────────┘
                              ▲
                              │
                              │ validates against
                              │

┌───────────────────┐                    ┌────────────────────┐
│ Application       │                    │ Device Agent       │
│ Deployment        │                    │                    │
└─────────┬─────────┘                    └─────────┬──────────┘
          │                                        │
          │ declares                               │ advertises
          ▼                                        ▼

┌───────────────────┐                    ┌────────────────────┐
│ ProviderBinding   │                    │ ProviderState      │
│-------------------│                    │--------------------│
│ request           │                    │ offer              │
│ fallback          │                    │ allocation         │
│ inject            │                    │ supported          │
└─────────┬─────────┘                    └─────────┬──────────┘
          │                                        │
          └──────────────┬─────────────────────────┘
                         │
                         ▼

                ┌───────────────────┐
                │        WFM        │
                │-------------------│
                │ Validation        │
                │ Scheduling        │
                │ Fleet Resolution  │
                └─────────┬─────────┘
                          │
                          │ device-scoped
                          ▼

                 ┌──────────────────┐
                 │  Device Agent    │
                 │------------------│
                 │ Resolution       │
                 │ Policy Checks    │
                 │ Allocation       │
                 └────────┬─────────┘
                          │
                          ▼

                 ┌──────────────────┐
                 │ProviderResolution│
                 │------------------│
                 │ status           │
                 │ result           │
                 │ failure reason   │
                 └──────────────────┘
```


# 6. Conformance

### Provider

* A Provider MUST declare a URI.
* A Provider MUST declare a scope.
* Breaking schema changes require a new Provider version.

### ProviderState

* Devices MUST advertise supported providers through ProviderState.
* Devices MUST update ProviderState when capability or allocation state changes.

### ProviderBinding

* Binding identifiers MUST be unique within a deployment.
* Referenced providers MUST exist in the Provider Registry.
* Fallback strategies MUST be supported by the referenced Provider.

### ProviderResolution

* The Device Agent MUST report a ProviderResolution entry for every processed ProviderBinding.

---

# 7. Open Questions

1. Provider versioning.
2. Multi-scope providers.
3. Registration and distribution of custom providers.
