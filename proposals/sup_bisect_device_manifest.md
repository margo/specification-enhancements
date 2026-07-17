# Specification Update Proposal — Device Manifest Bisection

**Status:** Draft  
**Owner:** @singhmj-1  
**Depends on:** `sup_device_roles_to_capabilities`

**Note:** This SUP has been extracted from [Resource Discovery and Conflict Resolution](https://github.com/margo/specification-enhancements/pull/66) SUP, as that one has become too complex.

---

## Summary

This proposal splits the `DeviceCapabilitiesManifest` into a distinct `DeviceManifest` for static identity and `ProfileState` documents for dynamic resource reporting. It introduces `ProfileDefinition` as an authoritative schema for device profiles, eliminating hardcoded fields and enabling structured, extensible capability reporting.

---

## Baseline

This proposal replaces the old schema format created by `sup_device_roles_to_capabilities`. In that all device identity and resource data were crammed into one single object called the DeviceCapabilitiesManifest.

The Old Format:
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

This proposal completely eliminates this combined format and rebuilds the way features are reported from scratch.

---

## Reason for Proposal

### Problem 1: Identity and resources are mixed together
Identity data rarel changes. Resource related data changes often. Sending them together creates major issues: 

* Wasted data: Resource updates force you to resend unchanged identity details.
* Confusion: The system cannot separate a serial number fix from a hardware change.
* No independent updates: You cannot version or evolve identity fields and resource fields separately.

### Problem 2: Limited descriptions for hardware and networks
The current setup uses basic labels that cannot share real-world details, as of now:

* A graphics card (GPU) lists only type and manufacturer. It doesn't show its VRAM size, system path, or architecture etc.
* A network connection (like CAN Bus) has only a type field. It doesn't show its speed, protocol, or channel ID etc.

The point to make here is that there would be requirement for a much complexer data structure for different resources and capabilities provided by platforms. It would be better to detach give them their dedicated documents.

### Problem 3: Basic computing fields cannot grow
Standard fields like cpu, memory, and storage are treated as rigid, flat text blocks. You cannot expand their structure easily. They need to become standalone profile files with their own flexible rulebooks.

### Problem 4: Hardware types are hardcoded
Adding a new piece of hardware (like an AI accelerator or secure chip) forces a complete update to the core system specification. This creates a bottleneck. Device makers cannot launch new features until the main community updates the global rules.

---

## Introducing the Profile Framework

A Profile is a specific resource, service, capability or settings on a device/wfm (like a GPU, local storage, vault service). To keep data organized, every Profile is split into two parts: its rules (ProfileDefinition) and its actual live data (ProfileState).

```
ProfileDefinition  →  defines the contract (schema, scope, ownership, category)
ProfileState       →  runtime data conforming to that contract
```

Devices send out many `ProfileState` documents after onboarding and update them when there are changes. Every `ProfileState` must conform to the rules written in its matching `ProfileDefinition` blueprint.

---

### ProfileDefinition

A `ProfileDefinition` is authored once — by Margo Community, a device vendor, or a WFM vendor — and registered with WFM. It establishes five things:

- **What** this profile is — its URI, category, and description
- **Who** owns the profile — scope : device-scoped (Device Agent) or fleet-scoped (WFM)
- **What category** it belongs to — resource, interface, peripheral, service, or capability
- **What format** the runtime data reporting must use

**Example ProfileDefinition:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/peripherals/gpu",
  "description": "Profile Definition for GPU peripherals",
  "scope": "device",
  "category": "peripheral",
  "schemaVersion": "1.0.0",
  "platformStateSchema": {
    "type": "object",
    "required": ["gpus"],
    "properties": {
      "gpus": {
        "type": "array",
        "minItems": 1,
        "items": {
          "type": "object",
          "required": ["model", "manufacturer"],
          "properties": {
            "manufacturer": { "type": "string" },
            "model": { "type": "string" },
            "vram": { "type": "string" },
            "architecture": { "type": "string" }
          }
        }
      }
    }
  }
}
```

> Note: This document only covers the reported data format (i.e. `platformStateSchema`). A separate SUP (the [Resource Profile Discovery & Resolution SUP](https://github.com/margo/specification-enhancements/pull/66) introduces the mechanism of how apps find and request these profiles during deployment. That SUP will extend the `ProfileDefinition` to include discovery rules under the same document with the same URI.
     
**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `profileDefinitionId` | string | Y | URI uniquely identifying this profile type. It must be a lower-case Reverse DNS string (e.g., deviceprofile.margo.org/peripherals/gpu). |
| `scope` | enum | Y | `device` — owned and published by Device Agent, `fleet` - owned by WFM(not introduced in this SUP) |
| `category` | enum | Y | One of `resource`, `interface`, `peripheral`, `service`, `capability`. Formally declares which category this profile belongs to. |
| `schemaVersion` | string | Y | Semantic version of this `ProfileDefinition` schema (e.g. `1.0.0`). Used to detect schema evolution and trigger re-validation of existing `ProfileState` documents. |
| `description` | string | Y | Human-readable description of this profile type. |
| `platformStateSchema` | object | Y | The format for runtime data that platforms will report. |

---

### ProfileDefinition Registration

`ProfileDefinition` documents must be registered with the WFM before any `ProfileState` can reference them. Margo-defined definitions are pre-loaded as part of the WFM specification bundle. Vendor-defined definitions can also be side loaded into the WFM. There is no API endpoint in this SUP to register them.

---

### ProfileState

A `ProfileState` is submitted by the profile owner after device onboarding and updated when hardware changes. The Device Agent submits it for device-scoped profiles.

**Example:**

```json
[{
  "profileDefinitionId": "deviceprofile.margo.org/peripherals/gpu",
  "platformState": {
    "gpus": [
      {
        "manufacturer": "NVIDIA",
        "model": "A100",
        "vram": "80Gi",
        "architecture": "Ampere"
      }
    ]
  }
}]
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `profileDefinitionId` | string | Y | URI referencing the matching `ProfileDefinition`. The WFM uses this to locate the blueprint and validate `platformState`. |
| `platformState` | object | Y | Runtime instance data. Must conform to the `platformStateSchema` of the referenced `ProfileDefinition`. |

---

## Introducing Device Profile

A Device Profile is a complete list of Profiles it has to offer. It describes all the hardware resources, communication tools, extra plug-in parts, and built-in software services available on that machine.

A Device Profile groups these features into five categories:
* **Resources:** Countable hardware parts like CPU cores, system memory (RAM), and disk drive storage space.
* **Interfaces:** Parts that connect the device to networks or other hardware, such as Ethernet ports, Wi-Fi chips, CAN Bus links, and USB ports.
* **Peripherals:** Extra physical hardware parts built into or attached to the device, like graphics cards (GPUs), cameras, or microphones.
* **Services:** Built-in software features run by the device itself, such as a secure password vault, a local DNS server, a centralized message queue, or an OpenTelemetry (OTel) server.
* **Capabilities:** The actual actions the device can do (like processing AI models, encoding video files, or gathering system metrics) because it has the right mix of hardware and services.

NOTE: We can add more or change them based on the consensus amongst the community.

## Room to Grow (Future Profiles)

The Device Profile design can be reused for other parts later on, but they are not introduced yet in this SUP. Similar profiles could be defined for:

* **WFM Profiles:** To describe fleet manager managed resources and services.
* **Gateway Profiles:** To describe gateway specific resources and services.

This creates a consistent mental model across the Margo ecosystem.

---

## Technical Proposal

### New Endpoints

The existing `DeviceCapabilitiesManifest` endpoint is replaced by two separate endpoint groups:

```
# Device identity
POST   /api/v1/clients/{clientId}/devices/{deviceId}
PUT    /api/v1/clients/{clientId}/devices/{deviceId}
DELETE /api/v1/clients/{clientId}/devices/{deviceId}

# Device profile state
POST   /api/v1/clients/{clientId}/devices/{deviceId}/profiles
PUT    /api/v1/clients/{clientId}/devices/{deviceId}/profiles
DELETE /api/v1/clients/{clientId}/devices/{deviceId}/profiles
```

**Ordering requirement:** `DeviceManifest` MUST be registered before any `ProfileState` can be submitted for the same `deviceId`. If the WFM receives a `ProfileState` for an unknown `deviceId`, it MUST reject with `404 Not Found`.

**Scope enforcement:** The WFM MUST verify that the submitting client matches the `scope` declared in the referenced `ProfileDefinition`:
- Violations MUST be rejected with `403 Forbidden`.

**Update granularity — bulk operations:**

Each POST or PUT to the profile endpoint carries an array of `ProfileState` documents in a single request:

```json
[
  {
    "profileDefinitionId": "deviceprofile.margo.org/resource/cpu",
    "platformState": { }
  },
  {
    "profileDefinitionId": "deviceprofile.margo.org/peripherals/gpu",
    "platformState": { }
  }
]
```

The WFM processes all items atomically: either all succeed or all fail. When it fails, the WFM MUST return a `422 Unprocessable Content` response identifying which `profileDefinitionId` caused the failure:

```json
{
  "error": "Validation failed",
  "failures": [
    {
      "profileDefinitionId": "deviceprofile.margo.org/peripherals/gpu",
      "reason": "incorrect format"
    }
  ]
}
```

**Gateway hierarchy submission rules:**
- A gateway MAY submit `ProfileState` documents on behalf of child devices using the hierarchical `{deviceId}` path (e.g. `gateway1/deviceA`).
- The `{deviceId}` path hierarchy encoding is retained unchanged on all endpoint groups.

---

### Response Codes

| Code | Description |
|------|-------------|
| `201 Created` | Document was added successfully. |
| `200 OK` | Document was updated successfully. |
| `204 No Content` | Document was deleted successfully. |
| `400 Bad Request` | Missing or invalid `content-digest` header, or malformed request body. |
| `401 Unauthorized` | Signature verification failed. |
| `403 Forbidden` | Client certificate is not trusted, has been revoked, or the submitting client does not match the `scope` of the referenced `ProfileDefinition`. |
| `404 Not Found` | No client with the given `clientId` found, or no `DeviceManifest` registered for the given `deviceId`. |
| `422 Unprocessable Content` | `platformState` does not conform to `platformStateSchema`, `profileDefinitionId` references an unregistered `ProfileDefinition`, capability dependency not satisfied, or batch contains semantic errors. Response body MUST identify the failing `profileDefinitionId` and reason. |

---

### DeviceManifest

Carries device identity only. Sent once at onboarding. Updated only if identity information changes.

**Endpoint:** `POST /api/v1/clients/{clientId}/devices/{deviceId}`

**Example:**

```json
{
  "deviceId": "northstarida.xtapro.k8s.edge",
  "vendor": "Northstar Industrial Devices",
  "modelNumber": "332ANZE1-N1",
  "serialNumber": "PF45343-AA"
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `deviceId` | string | Y | Unique device instance identifier. Distinct from `profileDefinitionId` which is a type reference URI. |
| `vendor` | string | Y | Device vendor name. |
| `modelNumber` | string | Y | Device model number. |
| `serialNumber` | string | Y | Device serial number. |

---

### Migrating DeviceCapabilitiesManifest Fields to Standalone Profiles

---

#### `deviceprofile.margo.org/resource/cpu`

**ProfileDefinition:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/resource/cpu",
  "description": "Profile Definition for CPU resources",
  "scope": "device",
  "category": "resource",
  "schemaVersion": "1.0.0",
  "platformStateSchema": {
    "type": "object",
    "required": ["cpus"],
    "properties": {
      "cpus": {
        "type": "array",
        "minItems": 1,
        "items": {
          "type": "object",
          "required": ["unit", "value", "architecture"],
          "properties": {
            "unit": {
              "type": "string",
              "enum": ["millicore", "core"],
              "description": "The unit for CPU cores"
            },
            "value": {
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

**ProfileState Example:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/resource/cpu",
  "platformState": {
    "cpus": [
      { "unit": "core", "value": 24, "architecture": "amd64", "model": "Intel Xeon" }
    ]
  }
}
```

---

#### `deviceprofile.margo.org/resource/memory`

**ProfileDefinition:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/resource/memory",
  "description": "Profile Definition for system memory resources",
  "scope": "device",
  "category": "resource",
  "schemaVersion": "1.0.0",
  "platformStateSchema": {
    "type": "object",
    "required": ["total"],
    "properties": {
      "total": {
        "type": "object",
        "required": ["unit", "value"],
        "description": "Total system memory",
        "properties": {
          "unit": {
            "type": "string",
            "enum": ["bytes", "Ki", "Mi", "Gi"],
            "description": "Binary unit for memory measurement"
          },
          "value": {
            "type": "integer",
            "minimum": 1,
            "description": "Total memory value in the specified unit"
          }
        }
      }
    }
  }
}
```

**ProfileState Example:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/resource/memory",
  "platformState": {
    "total": { "unit": "Gi", "value": 59 }
  }
}
```

---

#### `deviceprofile.margo.org/resource/storage`

**ProfileDefinition:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/resource/storage",
  "description": "Profile Definition for persistent storage resources",
  "scope": "device",
  "category": "resource",
  "schemaVersion": "1.0.0",
  "platformStateSchema": {
    "type": "object",
    "required": ["total"],
    "properties": {
      "total": {
        "type": "object",
        "required": ["unit", "value"],
        "description": "Total persistent storage in binary units (Ki, Mi, Gi, Ti) or SI units (K, M, G, T)",
        "properties": {
          "unit": {
            "type": "string",
            "enum": ["bytes", "Ki", "Mi", "Gi", "Ti", "K", "M", "G", "T"],
            "description": "Unit for storage measurement. Use binary units for binary-aligned storage and SI units for disk storage."
          },
          "value": {
            "type": "integer",
            "minimum": 1,
            "description": "Total storage value in the specified unit"
          }
        }
      }
    }
  }
}
```

**ProfileState Example:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/resource/storage",
  "platformState": {
    "total": { "unit": "G", "value": 1862 }
  }
}
```

---

#### `deviceprofile.margo.org/peripherals/gpu`

**ProfileDefinition:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/peripherals/gpu",
  "description": "Profile Definition for GPU peripheral hardware",
  "scope": "device",
  "category": "peripheral",
  "schemaVersion": "1.0.0",
  "platformStateSchema": {
    "type": "object",
    "required": ["gpus"],
    "properties": {
      "gpus": {
        "type": "array",
        "minItems": 1,
        "items": {
          "type": "object",
          "required": ["manufacturer"],
          "properties": {
            "manufacturer": { "type": "string", "description": "GPU manufacturer name" },
            "model": { "type": "string", "description": "GPU model name" },
            "vram": { 
              "unit": {
                "type": "string",
                "enum": ["bytes", "Ki", "Mi", "Gi"],
                "description": "Binary unit for memory measurement"
              },
              "value": {
                "type": "integer",
                "minimum": 1,
                "description": "Total memory value in the specified unit"
              },
              "description": "Video RAM capacity (e.g. 80Gi)"
            },
            "architecture": { "type": "string", "description": "GPU compute architecture (e.g. Ampere)" }
          }
        }
      }
    }
  }
}
```

**ProfileState Example:**

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

---

#### `deviceprofile.margo.org/interface/ethernet`

**ProfileDefinition:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/interface/ethernet",
  "description": "Profile Definition for Ethernet network interfaces",
  "scope": "device",
  "category": "interface",
  "schemaVersion": "1.0.0",
  "platformStateSchema": {
    "type": "object",
    "required": ["interfaces"],
    "properties": {
      "interfaces": {
        "type": "array",
        "minItems": 1,
        "items": {
          "type": "object",
          "required": ["name"],
          "properties": {
            "name": { "type": "string", "description": "Interface name (e.g. eth0)" }
          }
        }
      }
    }
  }
}
```

**ProfileState Example:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/interface/ethernet",
  "platformState": {
    "interfaces": [
      {
        "name": "eth0"
      }
    ]
  }
}
```

---

#### Vendor Specific Extension : `deviceprofile.example.org/interface/canbus`

**ProfileDefinition:**

```json
{
  "profileDefinitionId": "deviceprofile.example.org/interface/canbus",
  "description": "Profile Definition for CAN Bus interfaces",
  "scope": "device",
  "category": "interface",
  "schemaVersion": "1.0.0",
  "platformStateSchema": {
    "type": "object",
    "required": ["interfaces"],
    "properties": {
      "interfaces": {
        "type": "array",
        "minItems": 1,
        "items": {
          "type": "object",
          "required": ["channelId"],
          "properties": {
            "channelId": { "type": "string", "description": "CAN channel identifier (e.g. can0)" },
            "baudRate": { "type": "integer", "description": "Baud rate in bits per second (e.g. 500000)" },
            "protocol": { "type": "string", "enum": ["CAN2.0A", "CAN2.0B", "CANFD"], "description": "CAN protocol variant" }
          }
        }
      }
    }
  }
}
```

**ProfileState Example:**

```json
{
  "profileDefinitionId": "deviceprofile.example.org/interface/canbus",
  "platformState": {
    "interfaces": [
      { "channelId": "can0", "baudRate": 500000, "protocol": "CAN2.0B" }
    ]
  }
}
```

---

#### `deviceprofile.margo.org/service/otel-collector`

**ProfileDefinition:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/service/otel-collector",
  "description": "Profile Definition for the OpenTelemetry collector service",
  "scope": "device",
  "category": "service",
  "schemaVersion": "1.0.0",
  "platformStateSchema": {
    "type": "object",
    "required": ["endpoint"],
    "properties": {
      "endpoint": {
        "type": "string",
        "description": "The OTEL collector service endpoints"
      }
    }
  }
}
```

**ProfileState Example:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/service/otel-collector",
  "platformState": { "endpoint": "http://localhost:4317" }
}
```

---

#### `deviceprofile.margo.org/capability/runtimes`

**ProfileDefinition:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/capability/runtimes",
  "description": "Profile Definition for supported container runtimes and deployment types",
  "scope": "device",
  "category": "capability",
  "schemaVersion": "1.0.0",
  "platformStateSchema": {
    "type": "object",
    "required": ["containerRuntimes", "deploymentTypes"],
    "properties": {
      "containerRuntimes": {
        "type": "array",
        "minItems": 1,
        "items": { "type": "string" },
        "description": "List of supported container runtime standards (e.g. oci)"
      },
      "deploymentTypes": {
        "type": "array",
        "minItems": 1,
        "items": { "type": "string" },
        "description": "List of supported deployment mechanisms (e.g. helm, compose)"
      }
    }
  }
}
```

**ProfileState Example:**

```json
{
  "profileDefinitionId": "deviceprofile.margo.org/capability/runtimes",
  "platformState": {
    "containerRuntimes": ["oci"],
    "deploymentTypes": ["helm", "compose"]
  }
}
```

---

## Complete End-to-End Submission Example

**Step 1 — Register DeviceManifest:**

```
POST /api/v1/clients/{clientId}/devices/{deviceId}
```

```json
{
  "deviceId": "northstarida.xtapro.k8s.edge",
  "vendor": "Northstar Industrial Devices",
  "modelNumber": "332ANZE1-N1",
  "serialNumber": "PF45343-AA"
}
```

**Step 2 — Submit Device Profile:**

```
POST /api/v1/clients/{clientId}/devices/{deviceId}/profiles
```

```json
[
  {
    "profileDefinitionId": "deviceprofile.margo.org/resource/cpu",
    "platformState": {
      "cpus": [
        { "unit": "core", "value": 24, "architecture": "amd64", "model": "Intel Xeon" }
      ]
    }
  },
  {
    "profileDefinitionId": "deviceprofile.margo.org/resource/memory",
    "platformState": {
      "total": { "unit": "Gi", "value": 59 }
    }
  },
  {
    "profileDefinitionId": "deviceprofile.margo.org/resource/storage",
    "platformState": {
      "total": { "unit": "G", "value": 1862 }
    }
  },
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
  },
  {
    "profileDefinitionId": "deviceprofile.margo.org/interface/ethernet",
    "platformState": {
      "interfaces": [
        {
          "name": "eth0"
        }
      ]
    }
  },
  {
    "profileDefinitionId": "deviceprofile.margo.org/service/otel-collector",
    "platformState": { "endpoint": "http://localhost:4317" }
  },
  {
    "profileDefinitionId": "deviceprofile.margo.org/capability/runtimes",
    "platformState": {
      "containerRuntimes": ["oci"],
      "deploymentTypes": ["helm", "compose"]
    }
  }
]
```

**Step 3 — Remove a single profile:**

```
DELETE /api/v1/clients/{clientId}/devices/{deviceId}/profiles
[
  {
    "profileDefinitionId": "deviceprofile.margo.org/capability/runtimes"
  }
]
````

## Alternatives Considered

**Keep a single endpoint, rename to indicate broader scope.** Didn't consider as separate endpoints make the lifecycle difference explicit — the WFM can apply different caching, validation, and update logic to identity vs. profile data without inferring it from the payload structure.

---

## Open Questions

1. **Schema registry** — Should formal schema specifications (as provided above) be stored separately and versioned, or embedded inline in this SUP?

2. **Service profile standardization** — Which platform services (vault, DNS, etc.) should have standardized profiles in Margo, and what should their schemas be?