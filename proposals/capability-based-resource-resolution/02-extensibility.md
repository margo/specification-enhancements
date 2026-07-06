# Extensibility

## The Problem with the Current Model

The current specification encodes capability types directly into the core schema.
Adding a new resource type — a CAN Bus channel, an FPGA partition, a hardware
security module — requires:

1. A change to the `DeviceCapabilitiesManifest` schema
2. A new `PeripheralType` or `CommunicationInterfaceType` enumeration value
3. A specification release
4. All WFM and Device Agent implementations updated to understand the new type

This means the specification is the bottleneck. No capability can exist until
the specification recognizes it. Vendors cannot move faster than the TWG.

```text
Before — capability types are hardcoded in the core spec

Core Specification
 ├── DeviceCapabilitiesManifest
 │    ├── resources.cpu          ← hardcoded
 │    ├── resources.memory       ← hardcoded
 │    ├── resources.storage      ← hardcoded
 │    ├── peripherals[].type     ← enumeration, closed set
 │    └── interfaces[].type      ← enumeration, closed set
 │
 └── ApplicationDescription
      └── (no capability consumption model at all)
```

Every new capability type requires a core spec change. The spec is a
monolith of capability knowledge.

---

## How This Framework Changes It

The framework moves capability-specific knowledge out of the core specification
and into `CapabilityDefinitions` — documents that live outside the core spec and
can be authored independently.

```text
After — capability types are defined outside the core spec

Core Specification (the rule book)
 ├── CapabilityDefinition    ← the schema for defining a capability type
 ├── CapabilityState         ← the schema for publishing runtime state
 └── CapabilityDiscoveryRequest  ← the schema for consuming a capability

Capability Definitions (outside the core spec)
 ├── capability.margo.org/compute/cpu
 ├── capability.margo.org/compute/memory
 ├── capability.margo.org/compute/storage
 ├── capability.margo.org/hardware/gpu
 ├── capability.margo.org/hardware/camera
 ├── capability.margo.org/interface/network
 ├── capability.margo.org/interface/canbus
 ├── capability.margo.org/service/otel-collector
 └── capability.vendor.com/hardware/fpga   ← vendor-defined, no spec change needed
```

The core specification defines the **rules for defining capabilities** — not
the capabilities themselves. This is the key shift.

---

## Extensibility as a Specification Pipeline

The plug-and-play nature of `CapabilityDefinitions` creates a natural
pipeline from experimentation to standardization:

```text
Stage 1 — Vendor PoC
  A vendor authors a custom CapabilityDefinition under their own namespace.
  `capability.vendor.com/hardware/fpga`
  They implement it on their device and WFM. No spec involvement needed.

Stage 2 — Ecosystem Adoption
  Other vendors find the capability useful. They implement support for the
  same URI. The capability gains real-world usage and feedback without
  being in the spec.

Stage 3 — Standardization Proposal
  The vendor proposes the capability for inclusion in the Margo spec.
  The CapabilityDefinition already exists — it is the proposal.
  The TWG reviews the schema, the failure codes, the scope decision.
  If ratified, the URI moves to capability.margo.org/hardware/fpga.

Stage 4 — Standard Capability
  The capability is now part of the spec. All conformant implementations
  are expected to support it.


---

## What Extensibility Looks Like in Practice

### Scenario - A device vendor adds FPGA support

A device vendor ships an FPGA. They want applications to be able to request
FPGA partitions. Under the current spec, they cannot — `PeripheralType` has
no `fpga` value and they cannot add one without a spec change or breaking it.

Under this framework:

**Step 1 — Vendor authors a `CapabilityDefinition`:**
```yaml
apiVersion: margo.org/v1alpha1
kind: CapabilityDefinition

metadata:
  id: capability.vendor.com/hardware/fpga    # vendor-namespaced URI

spec:
  scope: device
  description: FPGA partition available for workload acceleration.

  sourceState:
    schema:
      type: object
      properties:
        partitions:
          type: array
          items:
            type: object
            properties:
              partitionId:
                type: string
              bitstreamSlots:
                type: integer
              allocated:
                type: boolean
              allocatedBy:
                type: string

  discovery:
    requestSchema:
      type: object
      properties:
        minBitstreamSlots:
          type: integer
        exclusive:
          type: boolean

    outputSchema:
      type: object
      properties:
        partitionId:
          type: string
        assignedSlots:
          type: integer

    failureCodes:
      - NoFPGAAvailable
      - InsufficientBitstreamSlots
      - ExclusiveAccessDenied
```

**Step 2 — Device Agent publishes `CapabilityState` by materializing the definition:**
```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "CapabilityState",
  "metadata": {
    "capability": "capability.vendor.com/hardware/fpga",
    "deviceId": "vendor-edge-device-001"
  },
  "spec": {
    "partitions": [
      {
        "partitionId": "fpga-partition-0",
        "bitstreamSlots": 4,
        "allocated": false,
        "allocatedBy": ""
      }
    ]
  }
}
```

**Step 3 — Application operator declares a `CapabilityDiscoveryRequest`:**
```yaml
discoverCapabilities:
  fpgaAccelerator:
    id: capability.vendor.com/hardware/fpga
    request:
      minBitstreamSlots: 2
      exclusive: true
```

No core specification change. No implementation update
required from WFM vendors who don't support this device. The capability
is self-contained.

---

## The Boundary Between Standard and Custom Capabilities

The framework distinguishes two classes of capabilities by URI namespace:

| Namespace | Authored by | Ratified by |
|---|---|---|
| `capability.margo.org/*` | Margo TWG | Margo specification process |
| `capability.{vendor}.com/*` | Any vendor | The vendor themselves |

Standard capabilities under `capability.margo.org` are part of the
specification and carry interoperability guarantees — any conformant WFM
or Device Agent MUST support them.

Custom capabilities under vendor namespaces carry no interoperability
guarantee — they work on devices and WFMs that understand them.

And the implementations MUST return a structured failure if a requested
capability URI is not recognised. Silent failure is not permitted.

