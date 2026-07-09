# Specification Update Proposal — Device Manifest Bisection

**Status:** Draft  
**Owner:** @singhmj-1  
**Depends on:** `sup_device_roles_to_capabilities`

**Note:** This SUP has been extracted from [Resource Discovery and Conflict Resolution](https://github.com/margo/specification-enhancements/pull/66) SUP, as that one has become too complex.

---

## Summary

The `sup_device_roles_to_capabilities` SUP removed `roles` and promoted all device fields to the `properties` root. The resulting `DeviceCapabilitiesManifest` still conflates two distinct concerns:

- **Device identity** — `id`, `vendor`, `modelNumber`, `serialNumber`. Static after onboarding. Never changes.
- **Device profile information** — `cpu`, `memory`, `storage`, `otelCollector`, `supportedRuntimes`, `supportedDeploymentTypes`, `peripherals`, `interfaces`. Changes primarily when hardware is added/removed or runtime support is modified.

This proposal:

1. Bisects the document into two separate documents with separate endpoints: **DeviceManifest** (identity only) and individual **ProfileState** documents (one per device profile attribute).
2. Introduces the concept of **Device Profile** — a structured description of all hardware resources, interfaces, peripherals, and services available on a device.
3. Removes the flat `peripherals` and `interfaces` enumerations entirely.
4. Promotes all profile data — including base compute fields and services — into structured `ProfileState` documents, each posted individually to the profile endpoint.
5. Simplifies the `ProfileState` schema to remove unnecessary wrapper fields.
6. Provides formal schema specifications for each standard profile URI.

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

### Problem 1 — Identity and profile information are coupled in a single document

Identity fields (`id`, `vendor`, `modelNumber`, `serialNumber`) never change after onboarding. Profile fields change infrequently — only when hardware is added/removed or when runtime support is modified. Sending both together means:

- A profile update forces re-sending identity data that has not changed
- The WFM cannot distinguish an identity change from a profile change
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

### Problem 3 — Base compute fields lack a structured model for extension

Fields like `cpu`, `memory`, `storage`, `otelCollector`, `supportedRuntimes`, and `supportedDeploymentTypes` are currently special-cased flat/scalar fields. There is no mechanism to extend their schema, express allocation metadata, or document their update semantics. Treating them as first-class `ProfileState` entries with formal schema specifications resolves this.

---

## Introducing Device Profile

This proposal introduces the concept of a **Device Profile** — a comprehensive description of all hardware resources, communication interfaces, physical peripherals, and platform services available on a device.

A **Device Profile** encompasses:

- **Resources** (CPU, Memory, Storage) — Quantifiable hardware resources
- **Interfaces** (Ethernet, WiFi, CAN Bus, USB) — Communication and connectivity mechanisms
- **Peripherals** (GPU, Camera, Microphone) — Specialized hardware components
- **Services** (Secret Vault, DNS Server, OTeL Collector) — Platform-managed services
- **Capabilities** (Video encoding, AI acceleration, observability collection) — Functional abilities enabled by hardware, services, or software

### Key Insight: Things vs. Capabilities

A crucial distinction: not all profile attributes are themselves *capabilities*. Rather, they are platform-managed *resources* and *services* that collectively form the device's profile and enable certain capabilities:

- A **GPU** (peripheral) is not a capability, but it *enables* video encoding and AI acceleration (capabilities)
- A **Secret Vault** (service) is not a capability, but it *enables* secure credential management (capability)
- A **DNS Server** (service) is not a capability, but it *enables* IP address assignment (capability)
- An **Ethernet interface** (interface) is not a capability, but it *enables* network communication (capability)

By treating all of these under the unified **Device Profile** concept, we can describe the complete environment available to workloads without conflating the *providers* of capabilities with the *capabilities* themselves.

### Pattern Extensibility

This Device Profile pattern is designed to be reusable beyond devices. Similar profiles could be defined for:

- **Workload Profile** — describing workload requirements and constraints
- **Workflow Profile** — describing workflow execution environment needs
- **Gateway Profile** — describing gateway-specific resources and services

This creates a consistent mental model across the Margo ecosystem.

---

## Technical Proposal

### New Endpoints

The existing combined endpoint is replaced by two separate endpoint groups:

```
# Device identity
POST   /api/v1/clients/{clientId}/devices/{deviceId}
PUT    /api/v1/clients/{clientId}/devices/{deviceId}
DELETE /api/v1/clients/{clientId}/devices/{deviceId}

# Device profile state
POST   /api/v1/clients/{clientId}/devices/{deviceId}/profile
PUT    /api/v1/clients/{clientId}/devices/{deviceId}/profile
DELETE /api/v1/clients/{clientId}/devices/{deviceId}/profile
```

**Ordering requirement:** `DeviceManifest` MUST be registered before any `ProfileState` can be submitted for the same `deviceId`. If the WFM receives a `ProfileState` for an unknown `deviceId`, it MUST reject with `404 Not Found`.

**Update granularity:**
- **Individual updates (default):** Each POST or PUT to the profile endpoint carries exactly one `ProfileState` document. The WFM uses `id` to identify which profile attribute is being created or updated. Multiple attributes are registered by sending multiple requests.
- **Bulk updates (optional):** Alternatively, clients MAY submit an array of `ProfileState` documents in a single request to reduce chattiness during initial onboarding:
  ```json
  [
    { "id": "deviceprofile.margo.org/resource/cpu", "spec": { ... } },
    { "id": "deviceprofile.margo.org/resource/memory", "spec": { ... } },
    { "id": "deviceprofile.margo.org/peripherals/gpu", "spec": { ... } }
  ]
  ```
  The WFM processes all items atomically: either all succeed or all fail.

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
| `404 Not Found` | No client with the given `clientId` found, or no `DeviceManifest` registered for the given `deviceId` (profile endpoint only). |
| `422 Unprocessable Content` | Request body includes a semantic error. |

---

### DeviceManifest

Carries device identity only. Sent once at onboarding. Updated only if identity information changes.

**Endpoint:** `POST /api/v1/clients/{clientId}/devices/{deviceId}`

**Example:**

```json
{
  "id": "northstarida.xtapro.k8s.edge",
  "vendor": "Northstar Industrial Devices",
  "modelNumber": "332ANZE1-N1",
  "serialNumber": "PF45343-AA"
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Y | Unique device identifier. Must include only unreserved characters as specified in RFC3986 plus the path separator `/`. |
| `vendor` | string | Y | Device vendor name. |
| `modelNumber` | string | Y | Device model number. |
| `serialNumber` | string | Y | Device serial number. |

---

### ProfileState

Each device profile attribute is submitted as an individual `ProfileState` document.

**Endpoint:** `POST /api/v1/clients/{clientId}/devices/{deviceId}/profile`

The WFM uses `id` to identify which profile attribute is being registered or updated. Submitting a `ProfileState` for a profile URI that already exists for the device replaces the previous state.

**Document structure:**

```json
{
  "id": "<profile-uri>",
  "spec": {
    // profile-specific state object
  }
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Y | URI identifying the profile attribute type. Determines the schema of `spec`. |
| `spec` | object | Y | State information for this profile attribute. Content is governed by the schema for the referenced profile URI. |

**DELETE — removing a profile attribute:**

To remove a profile attribute from a device (e.g. hardware was physically removed), send a DELETE request with the profile URI in the request body:

```
DELETE /api/v1/clients/{clientId}/devices/{deviceId}/profile
```

```json
{
  "id": "deviceprofile.margo.org/peripherals/gpu"
}
```

The WFM MUST remove the stored `ProfileState` for the given profile URI against the given device.

---

### Profile URI Namespace

Profile URIs follow a reverse-DNS convention and include a category prefix indicating the type of profile information:

| Namespace | Category | Owner | Example |
|-----------|----------|-------|---------|
| `deviceprofile.margo.org/resource/*` | Hardware resources | Margo specification | `deviceprofile.margo.org/resource/cpu` |
| `deviceprofile.margo.org/interface/*` | Communication interfaces | Margo specification | `deviceprofile.margo.org/interface/ethernet` |
| `deviceprofile.margo.org/peripherals/*` | Peripheral hardware | Margo specification | `deviceprofile.margo.org/peripherals/gpu` |
| `deviceprofile.margo.org/service/*` | Platform services | Margo specification | `deviceprofile.margo.org/service/vault` |
| `deviceprofile.margo.org/capability/*` | Functional capabilities | Margo specification | `deviceprofile.margo.org/capability/video-encoding` |
| `deviceprofile.<vendor-domain>/*` | Vendor-defined profile attributes | Vendor | `deviceprofile.acme.com/fieldbus/profinet` |

The WFM MUST accept any well-formed URI in `id`. It MUST validate `spec` content against the schema for the referenced URI if a schema is registered. If no schema is registered for the URI, the WFM MUST report a rejection.

---

### Standard Profile URIs Introduced by This SUP

The following profile URIs replace the flat fields from the baseline schema. All are under the `deviceprofile.margo.org` namespace. Each includes a formal schema specification.

#### `deviceprofile.margo.org/resource/cpu`

Replaces the `cpu` array field. Describes the total CPU capacity of the device (does not change unless physical hardware is replaced).

**Schema:**

```json
{
  "id": "deviceprofile.margo.org/resource/cpu",
  "spec": {
    "type": "object",
    "required": ["cpus"],
    "properties": {
      "cpus": {
        "type": "array",
        "minItems": 1,
        "items": {
          "type": "object",
          "required": ["cores", "architecture"],
          "properties": {
            "cores": {
              "type": "integer",
              "minimum": 1,
              "description": "Number of CPU cores"
            },
            "architecture": {
              "type": "string",
              "enum": ["amd64", "arm64", "arm", "x86"],
              "description": "CPU architecture"
            },
            "model": {
              "type": "string",
              "description": "Optional CPU model name"
            }
          }
        }
      }
    }
  }
}
```

**Example:**

```json
{
  "id": "deviceprofile.margo.org/resource/cpu",
  "spec": {
    "cpus": [
      { "cores": 24, "architecture": "amd64", "model": "Intel Xeon" }
    ]
  }
}
```

---

#### `deviceprofile.margo.org/resource/memory`

Replaces the `memory` field. Describes the total system memory capacity (does not change unless physical memory is upgraded).

**Schema:**

```json
{
  "id": "deviceprofile.margo.org/resource/memory",
  "spec": {
    "type": "object",
    "required": ["available"],
    "properties": {
      "available": {
        "type": "string",
        "pattern": "^\\d+\\s*(Gi|Mi|Ki|G|M|K)$",
        "description": "Total memory in binary units (Gi, Mi, etc.) or SI units (G, M, etc.)"
      }
    }
  }
}
```

**Example:**

```json
{
  "id": "deviceprofile.margo.org/resource/memory",
  "spec": {
    "available": "59 Gi"
  }
}
```

---

#### `deviceprofile.margo.org/resource/storage`

Replaces the `storage` field. Describes the total persistent storage capacity (does not change unless physical storage is upgraded).

**Schema:**

```json
{
  "id": "deviceprofile.margo.org/resource/storage",
  "spec": {
    "type": "object",
    "required": ["available"],
    "properties": {
      "available": {
        "type": "string",
        "pattern": "^\\d+\\s*(Gi|Mi|Ki|G|M|K)$",
        "description": "Total storage in binary units (Gi, Mi, etc.) or SI units (G, M, etc.)"
      }
    }
  }
}
```

**Example:**

```json
{
  "id": "deviceprofile.margo.org/resource/storage",
  "spec": {
    "available": "1862 Gi"
  }
}
```

---

#### `deviceprofile.margo.org/service/otel-collector`

Replaces the `otelCollector` boolean field. Indicates whether the device has OTeL collection enabled.

**Schema:**

```json
{
  "id": "deviceprofile.margo.org/service/otel-collector",
  "spec": {
    "type": "object",
    "required": ["present"],
    "properties": {
      "present": {
        "type": "boolean",
        "description": "Whether OTeL collection is enabled on this device"
      }
    }
  }
}
```

**Example:**

```json
{
  "id": "deviceprofile.margo.org/service/otel-collector",
  "spec": {
    "present": true
  }
}
```

---

#### `deviceprofile.margo.org/capability/runtimes`

Replaces the `supportedRuntimes` and `supportedDeploymentTypes` fields. Describes the runtime environments and deployment mechanisms supported by the device.

**Schema:**

```json
{
  "id": "deviceprofile.margo.org/capability/runtimes",
  "spec": {
    "type": "object",
    "properties": {
      "runtimes": {
        "type": "array",
        "items": { "type": "string" },
        "description": "List of supported container/runtime standards (e.g., 'oci', 'wasm')"
      },
      "deploymentTypes": {
        "type": "array",
        "items": { "type": "string" },
        "description": "List of supported deployment mechanisms (e.g., 'helm', 'compose', 'bare')"
      }
    }
  }
}
```

**Example:**

```json
{
  "id": "deviceprofile.margo.org/capability/runtimes",
  "spec": {
    "runtimes": ["oci"],
    "deploymentTypes": ["helm", "compose"]
  }
}
```

---

#### `deviceprofile.margo.org/peripherals/gpu`

Replaces `peripherals` entries of type `gpu`. Describes GPU hardware available on the device, including allocation state for orchestration.

**Schema:**

```json
{
  "id": "deviceprofile.margo.org/peripherals/gpu",
  "spec": {
    "type": "object",
    "required": ["gpus"],
    "properties": {
      "gpus": {
        "type": "array",
        "minItems": 1,
        "items": {
          "type": "object",
          "required": ["devicePath", "model"],
          "properties": {
            "devicePath": {
              "type": "string",
              "description": "Device path (e.g., '/dev/nvidia0')"
            },
            "model": {
              "type": "string",
              "description": "GPU model name (e.g., 'NVIDIA A100')"
            },
            "architecture": {
              "type": "string",
              "description": "GPU architecture (e.g., 'ampere', 'hopper')"
            },
            "vramGiB": {
              "type": "number",
              "minimum": 0,
              "description": "Total GPU memory in GiB"
            },
            "allocatedTo": {
              "type": "string",
              "description": "If set, identifies the workload/deployment claiming this GPU. Empty string means unallocated."
            }
          }
        }
      }
    }
  }
}
```

**Example:**

```json
{
  "id": "deviceprofile.margo.org/peripherals/gpu",
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

**Note:** The `allocatedTo` field tracks logical assignment for orchestration purposes. It is distinct from real-time usage metrics, which are reported separately via observability channels. An empty `allocatedTo` means the GPU is available; any non-empty string value indicates logical claim to that GPU.

---

#### `deviceprofile.margo.org/interface/ethernet`

Replaces `interfaces` entries of type `ethernet` and `wifi`. Describes network interfaces available on the device.

**Schema:**

```json
{
  "id": "deviceprofile.margo.org/interface/ethernet",
  "spec": {
    "type": "object",
    "required": ["interfaces"],
    "properties": {
      "interfaces": {
        "type": "array",
        "minItems": 1,
        "items": {
          "type": "object",
          "required": ["name", "type"],
          "properties": {
            "name": {
              "type": "string",
              "description": "Interface name (e.g., 'eth0', 'wlan0')"
            },
            "type": {
              "type": "string",
              "enum": ["ethernet", "wifi", "usb"],
              "description": "Interface type"
            },
            "speed": {
              "type": "string",
              "description": "Link speed (e.g., '1Gbps', '300Mbps')"
            }
          }
        }
      }
    }
  }
}
```

**Example:**

```json
{
  "id": "deviceprofile.margo.org/interface/ethernet",
  "spec": {
    "interfaces": [
      { "name": "eth0", "type": "ethernet", "speed": "1Gbps" },
      { "name": "wlan0", "type": "wifi", "speed": "300Mbps" }
    ]
  }
}
```

---

#### Vendor-defined example — `deviceprofile.acme.com/fieldbus/canbus`

Vendor-defined profile attributes follow the same structure. The vendor first needs to register the profile schema with WFM. The WFM rejects the profile state no schema is registered for the URI.

**Example:**

```json
{
  "id": "deviceprofile.acme.com/fieldbus/canbus",
  "spec": {
    "channels": [
      {
        "channelId": "can0",
        "baudRate": 500000,
        "protocol": "CANopen",
        "allocatedTo": ""
      },
      {
        "channelId": "can1",
        "baudRate": 250000,
        "protocol": "J1939",
        "allocatedTo": "deployment-sensor-monitor-003"
      }
    ]
  }
}
```

**Note:** Like the GPU example, `allocatedTo` (or similar allocation-tracking fields) distinguishes logical assignment from real-time usage.

---

## What Changes, What Stays the Same

| Field | Baseline (post roles-SUP) | After This SUP |
|-------|--------------------------|----------------|
| `id`, `vendor`, `modelNumber`, `serialNumber` | In `DeviceCapabilitiesManifest` | Moved to `DeviceManifest` — separate endpoint, simplified schema |
| `cpu` | Flat array in `DeviceCapabilitiesManifest` | `ProfileState` — `deviceprofile.margo.org/resource/cpu` with formal schema |
| `memory` | Flat field in `DeviceCapabilitiesManifest` | `ProfileState` — `deviceprofile.margo.org/resource/memory` with formal schema |
| `storage` | Flat field in `DeviceCapabilitiesManifest` | `ProfileState` — `deviceprofile.margo.org/resource/storage` with formal schema |
| `otelCollector` | Flat boolean in `DeviceCapabilitiesManifest` | `ProfileState` — `deviceprofile.margo.org/service/otel-collector` with formal schema |
| `supportedRuntimes` | Flat array in `DeviceCapabilitiesManifest` | `ProfileState` — `deviceprofile.margo.org/capability/runtimes` with formal schema |
| `supportedDeploymentTypes` | Flat array in `DeviceCapabilitiesManifest` | `ProfileState` — `deviceprofile.margo.org/capability/runtimes` with formal schema |
| `peripherals` | Flat enum | Removed — expressed as `ProfileState` per hardware type, with allocation metadata |
| `interfaces` | Flat enum | Removed — expressed as `ProfileState` per interface type |
| Gateway hierarchy encoding | In endpoint path | Unchanged — retained on both new endpoint groups |
| Onboarding order | Single document | `DeviceManifest` MUST precede first `ProfileState` |
| Update granularity | Entire document re-sent | One `ProfileState` per attribute; bulk array option available for onboarding |

---

## Important Semantic Distinctions

This proposal makes important distinctions between different types of device state:

### Capacity vs. Allocation vs. Usage

- **Capacity** — The total amount of a resource available (e.g., total CPU cores, total GPU memory). Reported via profile state, changes infrequently.
- **Allocation** — Which workloads or deployments claim which resources (e.g., "GPU 0 is allocated to deployment X"). Reported via profile state fields like `allocatedTo`, changes when workloads are scheduled.
- **Usage/Metrics** — Real-time consumption of resources (e.g., current CPU utilization, GPU memory in use). Reported via separate observability channels, not via device profile state.

The device profile state mechanism is appropriate for reporting capacity and allocation metadata. Real-time usage metrics should use the observability framework instead.

---

## Profile Pattern Extensibility

The Device Profile concept introduced here is intentionally reusable across the Margo ecosystem:

- **Device Profile** (`deviceprofile.margo.org/*`) — Describes what resources, services, and capabilities are available on a device
- **Workload Profile** (`workloadprofile.margo.org/*`, future) — Could describe workload requirements, constraints, and expected capabilities
- **Workflow Profile** (`workflowprofile.margo.org/*`, future) — Could describe workflow execution environment needs and service dependencies
- **Gateway Profile** (`gatewayprofile.margo.org/*`, future) — Could describe gateway-specific resources and services

This creates a consistent mental model across the Margo specification ecosystem where any entity's profile is a structured collection of attributes organized by category (resource, interface, peripheral, service, capability).

---

## Alternatives Considered

**Keep a single endpoint, rename to indicate broader scope.** Rejected. Separate endpoints make the lifecycle difference explicit — the WFM can apply different caching, validation, and update logic to identity vs. profile data without inferring it from the payload structure.

**Keep `peripherals` and `interfaces` as deprecated fields alongside profile state.** Rejected. Keeping deprecated fields creates ambiguity about which is authoritative when both are present. A clean removal forces implementations to adopt the structured model immediately and avoids a dual-path validation burden on the WFM.

**Wrap all profile attributes in a `DeviceProfileManifest` array.** Rejected. A wrapping manifest requires re-sending all attributes to update one. Individual `ProfileState` documents (with optional bulk array support) allow the device to update a single attribute (e.g., GPU allocation state) without touching others.

**Encode the profile URI in the endpoint path.** Rejected. The `ProfileState` document already carries the URI in `id`. Duplicating it in the path adds no information and introduces encoding complexity for URIs containing slashes.


---

## Open Questions

1. **Schema registry** — Should formal schema specifications (as provided above) be stored separately and versioned, or embedded inline in this SUP?

2. **Allocation tracking scope** — Should `allocatedTo` (or similar) be present in all allocatable resources, or only those where dynamic allocation is actually needed?

3. **Service profile standardization** — Which platform services (vault, DNS, etc.) should have standardized profiles in Margo, and what should their schemas be?