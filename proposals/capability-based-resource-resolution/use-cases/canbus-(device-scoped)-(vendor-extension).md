# Use Case — CAN Bus Channel Allocation

> **Status:** Informative (Non-Normative)

## Problem

An industrial automation application communicates with field devices —
motors, sensors, PLCs — over a CAN Bus interface. The device has multiple
CAN Bus channels, each configured for a specific baud rate and purpose.

Two applications deployed to the same device accidentally claim the same
channel. One application's messages corrupt the other's communication.
The failure is silent — both applications start successfully, but field
device behavior becomes unpredictable.

## Without the Capability Framework

Channel assignment is implementation-specific. There is no standard
mechanism in the Margo specification to express, discover, or validate
CAN Bus channel requirements. Operators assign channels manually and
track allocations out-of-band.

There is also no standard way for a vendor to introduce CAN Bus support
into the ecosystem without modifying the core specification.

## With the Capability Framework

A device vendor authors a `CapabilityDefinition` under their own namespace.
No core specification change is required. No TWG proposal is needed.

The Device Agent validates channel requests against current allocations
before applying the deployment. Conflicts are caught before any application
starts.

---

## CapabilityDefinition

This is a **vendor-defined capability** — note the namespace
`capability.vendor.com` rather than `capability.margo.org`. The vendor
introduces this capability independently, without modifying the core spec.

```yaml
apiVersion: margo.org/v1alpha1
kind: CapabilityDefinition

metadata:
  id: capability.vendor.com/fieldbus/canbus

spec:
  scope: device

  description: |
    Represents CAN Bus channels available on a device.
    Enables applications to request a channel by baud rate and
    protocol. Prevents two deployments from claiming the same channel.

  sourceState:
    schema:
      type: object
      properties:
        channels:
          type: array
          items:
            type: object
            properties:
              channelId:
                type: string
                description: Interface name e.g. can0, can1
              baudRate:
                type: integer
                description: Configured baud rate in bits/s e.g. 250000, 500000
              protocol:
                type: string
                description: e.g. CANopen, J1939, raw
              allocated:
                type: boolean
              allocatedBy:
                type: string

  discovery:
    requestSchema:
      type: object
      required: [baudRate]
      properties:
        baudRate:
          type: integer
          description: Required baud rate in bits/s
        protocol:
          type: string
          description: Required protocol. Optional — any protocol accepted if omitted.

    outputSchema:
      type: object
      properties:
        channelId:
          type: string
          description: Interface name of the assigned channel e.g. can0
        baudRate:
          type: integer
        protocol:
          type: string

    failureCodes:
      - NoChannelAvailable
      - BaudRateMismatch
      - ProtocolMismatch
      - ChannelAlreadyAllocated
```

---

## Device publishes the following details about the capability

```yaml
apiVersion: margo.org/v1alpha1
kind: CapabilityState
metadata:
  capability: capability.vendor.com/fieldbus/canbus
  deviceId: device-edge-industrial-001
spec:
  channels:
    - channelId: can0
      baudRate: 500000
      protocol: CANopen
      allocated: false
      allocatedBy: ""
    - channelId: can1
      baudRate: 250000
      protocol: J1939
      allocated: true
      allocatedBy: deployment-sensor-monitor-003
```

---

## How It Works

The operator requests a CAN Bus channel matching the application's
requirements. The resolved `channelId` is injected into the application
so it knows which interface to open.

```yaml
# ApplicationDeployment (relevant sections only)
spec:
  discoverCapabilities:
    motorControlBus:
      id: capability.vendor.com/fieldbus/canbus
      request:
        baudRate: 500000
        protocol: CANopen

  parameters:
    canInterface:
      valueFrom: discoverCapabilities.motorControlBus.output.channelId
      targets:
        - pointer: env.CAN_INTERFACE
          components: ["motor-controller"]
```

The Device Agent evaluates the request against its local `CapabilityState`.
If a free channel matching `baudRate: 500000` and `protocol: CANopen` exists,
it is assigned and `channelId` is injected into `canInterface`.

If no matching channel is free:

```yaml
capabilityResolution:
  status: Failure
  failureCode: ChannelAlreadyAllocated
  message: >
    No CANopen channel at 500000 baud is currently free on
    device-edge-industrial-001. All matching channels are allocated
    to existing deployments.
```

---

## What This Demonstrates

| Concern | How it is addressed |
|---|---|
| Channel conflict between deployments | Device Agent checks allocations before applying |
| Application receives correct interface name | `channelId` injected via `valueFrom` |
| No core spec change needed | Vendor authors under `capability.vendor.com` namespace |
| Unknown capabilities | Unrecognised URIs are reported by the WFM/Devices and proper errors should be returned |

---

## Key Architectural Point — Vendor Extensibility

This use case is the primary demonstration of the framework's extensibility
story. The CAN Bus capability does not exist in the Margo standard capability
library. The vendor introduces it independently:

```text
capability.vendor.com/fieldbus/canbus
```

Any device that supports this capability publishes `CapabilityState` for it.
Any WFM that recognises this URI can validate and route requests for it.
WFMs that do not recognise it treat it as unsupported and surface a clear
failure — they do not break silently.

If this capability gains broad adoption across vendors, it can be proposed
for standardisation. The `CapabilityDefinition` already exists — it becomes
the proposal. The TWG reviews a proven, real-world schema rather than a
speculative one. If ratified, the URI moves to:

```text
capability.margo.org/fieldbus/canbus
```

The schema does not change. Implementations update their supported URI.
This is the full extensibility pipeline in practice.