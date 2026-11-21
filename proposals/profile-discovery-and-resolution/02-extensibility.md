# Extensibility

## The Problem with the Current Model

The current specification encodes profile types directly into the core schema.
Adding a new resource type — a CAN Bus channel, an FPGA partition, a hardware
security module — requires:

1. A change to the `DeviceCapabilitiesManifest` schema
2. A new `PeripheralType` or `CommunicationInterfaceType` enumeration value
3. A specification release
4. All WFM and Device Agent implementations updated to understand the new type

This means the specification is the bottleneck. No profile can exist until
the specification recognizes it. Vendors cannot move faster than the TWG.

```text
Before — profile types are hardcoded in the core spec

Core Specification
 ├── DeviceCapabilitiesManifest
 │    ├── resources.cpu          ← hardcoded
 │    ├── resources.memory       ← hardcoded
 │    ├── resources.storage      ← hardcoded
 │    ├── peripherals[].type     ← enumeration, closed set
 │    └── interfaces[].type      ← enumeration, closed set
 │
 └── ApplicationDescription
      └── (no profile consumption model at all)
```

Every new profile type requires a core spec change. The spec is a
monolith of profile knowledge.

---

## How This Framework Changes It

The framework moves profile-specific knowledge out of the core specification
and into `ProfileDefinitions` — documents that live outside the core spec and
can be authored independently.

```text
After — profile types are defined outside the core spec

Core Specification (the rule book)
 ├── ProfileDefinition           ← the schema for defining a profile type
 ├── ProfileState                ← the schema for publishing runtime state
 └── ProfileDiscoveryRequest     ← the schema for consuming a profile

Profile Definitions (outside the core spec)
 ├── deviceprofile.margo.org/compute/cpu
 ├── deviceprofile.margo.org/compute/memory
 ├── deviceprofile.margo.org/compute/storage
 ├── deviceprofile.margo.org/peripherals/gpu
 ├── deviceprofile.margo.org/peripherals/camera
 ├── deviceprofile.margo.org/interface/network
 ├── deviceprofile.margo.org/interface/canbus
 ├── deviceprofile.margo.org/service/otel-collector
 └── deviceprofile.vendor.com/hardware/fpga   ← vendor-defined, no spec change needed
```

The core specification defines the **rules for defining profiles** — not
the profiles themselves. This is the key shift.

---

## Extensibility as a Specification Pipeline

The plug-and-play nature of `ProfileDefinitions` creates a natural
pipeline from experimentation to standardization:

```text
Stage 1 — Vendor PoC
  A vendor authors a custom ProfileDefinition under their own namespace.
  `deviceprofile.vendor.com/hardware/fpga`
  They implement it on their device and WFM. No spec involvement needed.

Stage 2 — Ecosystem Adoption
  Other vendors find the profile useful. They implement support for the
  same URI. The profile gains real-world usage and feedback without
  being in the spec.

Stage 3 — Standardization Proposal
  The vendor proposes the profile for inclusion in the Margo spec.
  The ProfileDefinition already exists — it is the proposal.
  The TWG reviews the schema, the failure codes, the scope decision.
  If ratified, the URI moves to deviceprofile.margo.org/hardware/fpga.

Stage 4 — Standard Profile
  The profile is now part of the spec. All conformant implementations
  are expected to support it.
```

---

## What Extensibility Looks Like in Practice

### Scenario - A device vendor adds FPGA support

A device vendor ships an FPGA. They want applications to be able to request
FPGA partitions. Under the current spec, they cannot — `PeripheralType` has
no `fpga` value and they cannot add one without a spec change or breaking it.

Under this framework:

**Step 1 — Vendor authors a `ProfileDefinition`:**
```json
{
  "profileDefinitionId": "deviceprofile.vendor.com/hardware/fpga",
  "description": "FPGA partition available for workload acceleration.",
  "scope": "device",
  "category": "peripheral",
  "schemaVersion": "1.0.0",
  "platformStateSchema": {
    "type": "object",
    "properties": {
      "partitions": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "partitionId": {
              "type": "string"
            },
            "bitstreamSlots": {
              "type": "integer"
            }
          }
        }
      }
    }
  },
  "discoverySchema": {
    "mode": "generator",
    "requestSchema": {
      "type": "object",
      "properties": {
        "minBitstreamSlots": {
          "type": "integer"
        },
        "exclusive": {
          "type": "boolean"
        }
      }
    },
    "outputSchema": {
      "type": "object",
      "properties": {
        "partitionId": {
          "type": "string"
        },
        "assignedSlots": {
          "type": "integer"
        }
      }
    },
    "failureCodes": [
      "NoFPGAAvailable",
      "InsufficientBitstreamSlots",
      "ExclusiveAccessDenied"
    ]
  }
}
```

**Step 2 — Device Agent publishes `ProfileState` by materializing the definition:**
```json
{
  "profileDefinitionId": "deviceprofile.vendor.com/hardware/fpga",
  "platformState": {
    "partitions": [
      {
        "partitionId": "fpga-partition-0",
        "bitstreamSlots": 4
      }
    ]
  }
}
```

**Step 3 — Application operator declares a `ProfileDiscoveryRequest`:**
```yaml
discoverProfiles:
  fpgaReference:                               # unique name within this deployment
    id: deviceprofile.vendor.com/hardware/fpga  # references ProfileDefinition URI
    request:                                     # conforms to discoverySchema.requestSchema
      partitionId: fpga-partition-0
      bitstreamSlots: 2
```

No core specification change. No implementation update
required from WFM vendors who don't support this device. The profile
is self-contained.

---

## The Boundary Between Standard and Custom Profiles

The framework distinguishes two classes of profiles by URI namespace:

| Namespace | Authored by | Ratified by |
|---|---|---|
| `deviceprofile.margo.org/*` | Margo TWG | Margo specification process |
| `deviceprofile.{vendor}.com/*` | Any vendor | The vendor themselves |

Standard profiles under `deviceprofile.margo.org` are part of the
specification and carry interoperability guarantees — any conformant Device Agent MUST support them.

Custom profiles under vendor namespaces carry no interoperability
guarantee — they work on devices and WFMs that understand them.