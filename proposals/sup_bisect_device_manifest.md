# Specification Update Proposal — Device Manifest Bisection

**Status:** Draft  
**Owner:** @singhmj-1  
**Depends on:** `sup_device_roles_to_capabilities`
**Note:** This SUP has been extracted from [Resource Discovery and Conflict Resolution](https://github.com/margo/specification-enhancements/pull/66) SUP, as that one has become too complex.

---

## Summary

The `sup_device_roles_to_capabilities` SUP removed `roles` and promoted all device fields to the `properties` root. The resulting `DeviceCapabilitiesManifest` still conflates two distinct concerns:

- **Device identity** — `id`, `vendor`, `modelNumber`, `serialNumber`. Static after onboarding. Never changes.
- **Device capabilities** — `cpu`, `memory`, `storage`, `otelCollector`, `supportedRuntimes`, `supportedDeploymentTypes`, `peripherals`, `interfaces`. Dynamic — updated as hardware and runtime state changes.

This proposal:

1. Bisects the document into two separate documents with separate endpoints: **DeviceManifest** (identity only) and individual **CapabilityState** documents (one per capability).
2. Expands the definition of a capability.
3. Removes the flat `peripherals` and `interfaces` enumerations entirely.
4. Promotes all capability data — including base compute fields — into structured `CapabilityState` documents, each posted individually to the capabilities endpoint.
5. Introduces the `CapabilityState` model inline, as the Capability Definition Framework SUP has not yet been formally introduced.

---

## Baseline

This proposal modifies the schema produced by `sup_device_roles_to_capabilities`. The current baseline after that SUP is:

```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "DeviceCapabilitiesManifest",
  "properties": {
    "id": "northstarida.xtapro.k8s.edge",
    "vendor": "Northstar Industrial Devices",
    "modelNumber": "332ANZE1-N1",
    "serialNumber": "PF45343-AA",
    "cpu": [{ "cores": 24, "architecture": "amd64" }],
    "memory": "59 Gi",
    "storage": "1862 Gi",
    "peripherals": [{ "type": "gpu", "manufacturer": "NVIDIA" }],
    "interfaces": [{ "type": "ethernet" }, { "type": "wifi" }],
    "otelCollector": true,
    "supportedRuntimes": ["oci"],
    "supportedDeploymentTypes": ["helm", "compose"]
  }
}
```

---

## Reason for Proposal

### Problem 1 — Identity and capabilities are coupled in a single document

Identity fields (`id`, `vendor`, `modelNumber`, `serialNumber`) never change after onboarding. Capability fields change frequently — whenever workloads are deployed, hardware is added, or runtime configuration changes. Sending both together means:

- A capability update forces re-sending identity data that has not changed
- The WFM cannot distinguish a capability update from an identity change
- The two concerns cannot evolve or be versioned independently

### Problem 2 — `peripherals` and `interfaces` use closed enumerations that cannot express structured state

A GPU reported as:

```json
{ "type": "gpu", "manufacturer": "NVIDIA" }
```

Cannot express VRAM, device path, allocation state, or compute architecture. A CAN Bus interface reported as:

```json
{ "type": "canbus" }
```

Cannot express baud rate, protocol, channel identity, or allocation state.

### Problem 3 — Base compute fields are not capabilities in the structured sense

Fields like `cpu`, `memory`, `storage`, `otelCollector`, `supportedRuntimes`, and `supportedDeploymentTypes` are currently special-cased flat/scalar fields. There is no mechanism to extend their schema, express allocation state, or update them independently of each other. Treating them as first-class `CapabilityState` entries resolves this.

---

## Expanding the Definition of Capability

So far, the definition of a capability is restrictive, but this framework expands it:

> A capability represents wide variety of **platform-managed concerns** 
including the following:
> * "Hardware resources" such as GPUs, cameras, fieldbus channels etc.
> * "Software capabilities" such as persistent storage, pre-installed 
runtimes etc.
> * "Platform services" such as api gateways, security vaults, otel collectors 
endpoints, pki, message buses etc.


### Capability Model
The current spec treats a "capability" as a flat enumeration value — a GPU is `{ "type": "gpu" }`, an interface is `{ "type": "ethernet" }`. This model cannot express runtime state, allocation, or structured metadata.

This proposal introduces a richer capability model. A **CapabilityState** is a structured document that:

- References a **capability URI** — a globally unique identifier for the capability type (e.g. `capability.margo.org/hardware/gpu`)
- Carries a **spec** — a capability-specific object whose schema is defined by the capability type

The capability URI is the key. It identifies not just the type of capability, but the schema that governs the `spec` content. This allows:

- Vendors to define their own capability types under their own URI namespace (e.g. `capability.acme.com/fieldbus/profinet`)
- The WFM to validate `spec` content against the schema for the referenced URI
- Capabilities to be updated, added, or removed independently — one `CapabilityState` per capability, one request per update

---

## Technical Proposal

### New Endpoints

The existing combined endpoint is replaced by two separate endpoint groups:

```
# Device identity
POST   /api/v1/clients/{clientId}/devices/{deviceId}
PUT    /api/v1/clients/{clientId}/devices/{deviceId}
DELETE /api/v1/clients/{clientId}/devices/{deviceId}

# Individual capability states
POST   /api/v1/clients/{clientId}/devices/{deviceId}/capabilities
PUT    /api/v1/clients/{clientId}/devices/{deviceId}/capabilities
DELETE /api/v1/clients/{clientId}/devices/{deviceId}/capabilities
```

**Ordering requirement:** `DeviceManifest` MUST be registered before any `CapabilityState` can be submitted for the same `deviceId`. If the WFM receives a `CapabilityState` for an unknown `deviceId`, it MUST reject with `404 Not Found`.

**Individual updates:** Each POST or PUT to the capabilities endpoint carries exactly one `CapabilityState` document. The WFM uses `metadata.id` to identify which capability is being created or updated. Multiple capabilities are registered by sending multiple requests.

**Gateway hierarchy:** The `{deviceId}` path hierarchy encoding (e.g. `gateway1/deviceA`) is retained unchanged on both endpoint groups.

---

### Response Codes

| Code | Description |
|------|-------------|
| `201 Created` | Document was added successfully. |
| `200 OK` | Document was updated successfully. |
| `204 No Content` | Document was deleted successfully. |
| `400 Bad Request` | Missing or invalid `content-digest` header, or malformed request body. |
| `401 Unauthorized` | Signature verification failed. |
| `403 Forbidden` | Client certificate is not trusted or has been revoked. |
| `404 Not Found` | No client with the given `clientId` found, or no `DeviceManifest` registered for the given `deviceId` (capabilities endpoint only). |
| `422 Unprocessable Content` | Request body includes a semantic error. |

---

### DeviceManifest

Carries device identity only. Sent once at onboarding. Updated only if identity information changes.

**Endpoint:** `POST /api/v1/clients/{clientId}/devices/{deviceId}`

**Example:**

```json
{
  "apiVersion": "margo.org/v1",
  "kind": "DeviceManifest",
  "properties": {
    "id": "northstarida.xtapro.k8s.edge",
    "vendor": "Northstar Industrial Devices",
    "modelNumber": "332ANZE1-N1",
    "serialNumber": "PF45343-AA"
  }
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `apiVersion` | string | Y | Must be `margo.org/v1`. |
| `kind` | string | Y | Must be `DeviceManifest`. |
| `properties.id` | string | Y | Unique device identifier. Must include only unreserved characters as specified in RFC3986 plus the path separator `/`. |
| `properties.vendor` | string | Y | Device vendor name. |
| `properties.modelNumber` | string | Y | Device model number. |
| `properties.serialNumber` | string | Y | Device serial number. |

---

### CapabilityState

Each capability is submitted as an individual `CapabilityState` document. There is no wrapping manifest — the `CapabilityState` is the document.

**Endpoint:** `POST /api/v1/clients/{clientId}/devices/{deviceId}/capabilities`

The WFM uses `metadata.id` to identify which capability is being registered or updated. Submitting a `CapabilityState` for a `capability` URI that already exists for the device replaces the previous state.

**Document structure:**

```json
{
  "apiVersion": "margo.org/v1",
  "kind": "CapabilityState",
  "metadata": {
    "id": "<capability-uri>"
  },
  "spec": {
    // capability-specific state object
  }
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `apiVersion` | string | Y | Must be `margo.org/v1`. |
| `kind` | string | Y | Must be `CapabilityState`. |
| `metadata.id` | string | Y | URI identifying the capability type. Determines the schema of `spec`. |
| `spec` | object | Y | Runtime state for this capability. Content is governed by the schema for the referenced capability URI. |

**DELETE — removing a capability:**

To remove a capability from a device (e.g. hardware was physically removed), send a DELETE request with the capability URI in the request body:

```
DELETE /api/v1/clients/{clientId}/devices/{deviceId}/capabilities
```

```json
{
  "id": "capability.margo.org/hardware/gpu"
}
```

The WFM MUST remove the stored `CapabilityState` for the given `capability` URI on the given device.

---

### Capability URI Namespace

Capability URIs follow a reverse-DNS convention:

| Namespace | Owner | Example |
|-----------|-------|---------|
| `capability.margo.org/*` | Margo specification — standardised capabilities | `capability.margo.org/hardware/gpu` |
| `capability.<vendor-domain>/*` | Vendor-defined capabilities | `capability.acme.com/fieldbus/profinet` |

The WFM MUST accept any well-formed URI in `metadata.id`. It MUST validate `spec` content against the schema for the referenced URI if a schema is registered. If no schema is registered for the URI, the WFM MUST store the `spec` as-is without validation.

---

### Standard Capability URIs Introduced by This SUP

The following capability URIs replace the flat fields from the baseline schema. All are under the `capability.margo.org` namespace.

#### `capability.margo.org/compute/cpu`

Replaces the `cpu` array field.

```json
{
  "apiVersion": "margo.org/v1",
  "kind": "CapabilityState",
  "metadata": {
    "id": "capability.margo.org/compute/cpu"
  },
  "spec": {
    "cpus": [
      { "cores": 24, "architecture": "amd64" }
    ]
  }
}
```

#### `capability.margo.org/compute/memory`

Replaces the `memory` field.

```json
{
  "apiVersion": "margo.org/v1",
  "kind": "CapabilityState",
  "metadata": {
    "id": "capability.margo.org/compute/memory"
  },
  "spec": {
    "available": "59 Gi"
  }
}
```

#### `capability.margo.org/compute/storage`

Replaces the `storage` field.

```json
{
  "apiVersion": "margo.org/v1",
  "kind": "CapabilityState",
  "metadata": {
    "id": "capability.margo.org/compute/storage"
  },
  "spec": {
    "available": "1862 Gi"
  }
}
```

#### `capability.margo.org/service/otel-collector`

Replaces the `otelCollector` boolean field.

```json
{
  "apiVersion": "margo.org/v1",
  "kind": "CapabilityState",
  "metadata": {
    "id": "capability.margo.org/service/otel-collector"
  },
  "spec": {
    "present": true
  }
}
```

#### `capability.margo.org/runtime/supported`

Replaces the `supportedRuntimes` and `supportedDeploymentTypes` fields.

```json
{
  "apiVersion": "margo.org/v1",
  "kind": "CapabilityState",
  "metadata": {
    "id": "capability.margo.org/runtime/supported"
  },
  "spec": {
    "runtimes": ["oci"],
    "deploymentTypes": ["helm", "compose"]
  }
}
```

#### `capability.margo.org/hardware/gpu`

Replaces `peripherals` entries of type `gpu`.

```json
{
  "apiVersion": "margo.org/v1",
  "kind": "CapabilityState",
  "metadata": {
    "id": "capability.margo.org/hardware/gpu"
  },
  "spec": {
    "gpus": [
      {
        "devicePath": "/dev/nvidia0",
        "model": "NVIDIA A100",
        "architecture": "ampere",
        "vramGiB": 40,
        "allocatedTo": ""
      }
    ]
  }
}
```

#### `capability.margo.org/interface/ethernet`

Replaces `interfaces` entries of type `ethernet` and `wifi`.

```json
{
  "apiVersion": "margo.org/v1",
  "kind": "CapabilityState",
  "metadata": {
    "id": "capability.margo.org/interface/ethernet"
  },
  "spec": {
    "interfaces": [
      { "name": "eth0", "speed": "1Gbps" },
      { "name": "wlan0", "speed": "300Mbps" }
    ]
  }
}
```

#### Vendor-defined example — `capability.acme.com/fieldbus/canbus`

Vendor-defined capabilities follow the same structure. The WFM stores the `spec` as-is if no schema is registered for the URI.

```json
{
  "apiVersion": "margo.org/v1",
  "kind": "CapabilityState",
  "metadata": {
    "id": "capability.acme.com/fieldbus/canbus"
  },
  "spec": {
    "channels": [
      {
        "channelId": "can0",
        "baudRate": 500000,
        "protocol": "CANopen",
        "allocated": false,
        "allocatedBy": ""
      },
      {
        "channelId": "can1",
        "baudRate": 250000,
        "protocol": "J1939",
        "allocated": true,
        "allocatedBy": "deployment-sensor-monitor-003"
      }
    ]
  }
}
```

---

## What Changes, What Stays the Same

| Field | Baseline (post roles-SUP) | After This SUP |
|-------|--------------------------|----------------|
| `id`, `vendor`, `modelNumber`, `serialNumber` | In `DeviceCapabilitiesManifest` | Moved to `DeviceManifest` — separate endpoint |
| `cpu` | Flat array in `DeviceCapabilitiesManifest` | `CapabilityState` — `capability.margo.org/compute/cpu` |
| `memory` | Flat field in `DeviceCapabilitiesManifest` | `CapabilityState` — `capability.margo.org/compute/memory` |
| `storage` | Flat field in `DeviceCapabilitiesManifest` | `CapabilityState` — `capability.margo.org/compute/storage` |
| `otelCollector` | Flat boolean in `DeviceCapabilitiesManifest` | `CapabilityState` — `capability.margo.org/service/otel-collector` |
| `supportedRuntimes` | Flat array in `DeviceCapabilitiesManifest` | `CapabilityState` — `capability.margo.org/runtime/supported` |
| `supportedDeploymentTypes` | Flat array in `DeviceCapabilitiesManifest` | `CapabilityState` — `capability.margo.org/runtime/supported` |
| `peripherals` | Flat enum | Removed — expressed as `CapabilityState` per hardware type |
| `interfaces` | Flat enum | Removed — expressed as `CapabilityState` per interface type |
| Gateway hierarchy encoding | In endpoint path | Unchanged — retained on both new endpoint groups |
| Onboarding order | Single document | `DeviceManifest` MUST precede first `CapabilityState` |
| Update granularity | Entire document re-sent | One `CapabilityState` per capability, updated independently |

---

## Alternatives Considered

**Keep a single endpoint, split the schema into two sub-objects.** Rejected. Separate endpoints make the lifecycle difference explicit — the WFM can apply different caching, validation, and update logic to identity vs capability data without inferring it from the payload structure.

**Keep `peripherals` and `interfaces` as deprecated fields alongside `capabilities[]`.** Rejected. Keeping deprecated fields creates ambiguity about which is authoritative when both are present. A clean removal forces implementations to adopt the structured model immediately and avoids a dual-path validation burden on the WFM.

**Wrap all capabilities in a `DeviceCapabilitiesManifest` array.** Rejected. A wrapping manifest requires re-sending all capabilities to update one. Individual `CapabilityState` documents allow the device to update a single capability (e.g. GPU allocation state changed) without touching others.

**Encode the capability URI in the endpoint path.** Rejected. The `CapabilityState` document already carries the URI in `metadata.id`. Duplicating it in the path adds no information and introduces encoding complexity for URIs containing slashes.

---

## Open Questions

1. Gateway capability reporting — opaque and see-thru gateway `CapabilityState` update semantics following Gateway SUP injection. To be addressed in a follow-on proposal.