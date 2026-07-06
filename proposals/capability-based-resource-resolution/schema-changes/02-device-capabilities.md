## Device Reporting Changes

The existing `DeviceCapabilitiesManifest` is replaced by two separate
documents with separate API endpoints. This separation reflects the
fundamental difference in the nature and update cadence of the two
concerns:

| Document | Concern | Update Cadence |
|---|---|---|
| `DeviceManifest` | Identity — who is this device | Once at onboarding, rarely after |
| `CapabilityState` | Runtime state — what can this device offer right now | Whenever state changes |

---

### DeviceManifest — replaces DeviceCapabilitiesManifest

`DeviceManifest` carries device identity only. It has no `resources` block.
All resource and capability information moves to `CapabilityState`.

```
POST   /api/v1/clients/{clientId}/devices/{deviceId}
PUT    /api/v1/clients/{clientId}/devices/{deviceId}
DELETE /api/v1/clients/{clientId}/devices/{deviceId}
```

```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "DeviceManifest",
  "properties": {
    "id": "northstarida.xtapro.k8s.edge",
    "vendor": "Northstar Industrial devices",
    "modelNumber": "332ANZE1-N1",
    "serialNumber": "PF45343-AA",
    "roles": ["standalone cluster", "cluster leader"]
  }
}
```

**Rules:**
- MUST be sent once during onboarding before any `CapabilityState` is reported
- MUST be resent if identity fields change
- MUST NOT include any resource or capability state information

---

### CapabilityState — new, replaces resources block

`CapabilityState` documents are sent independently, one per capability type.
Each document conforms to the `sourceState.schema` of the referenced
`CapabilityDefinition`.

```
POST   /api/v1/clients/{clientId}/devices/{deviceId}/capabilities
PUT    /api/v1/clients/{clientId}/devices/{deviceId}/capabilities
DELETE /api/v1/clients/{clientId}/devices/{deviceId}/capabilities/{capabilityId}
```

The device sends one `CapabilityState` per capability it supports. The
following are the standard capabilities that replace the existing `resources`
block:

**CPU**
```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "CapabilityState",
  "metadata": {
    "capability": "capability.margo.org/compute/cpu",
    "deviceId": "northstarida.xtapro.k8s.edge"
  },
  "spec": {
    "architecture": "x86_64",
    "cores": 24,
    "allocatedCores": 0
  }
}
```

**Memory**
```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "CapabilityState",
  "metadata": {
    "capability": "capability.margo.org/compute/memory",
    "deviceId": "northstarida.xtapro.k8s.edge"
  },
  "spec": {
    "totalGiB": 59,
    "availableGiB": 59
  }
}
```

**Storage**
```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "CapabilityState",
  "metadata": {
    "capability": "capability.margo.org/compute/storage",
    "deviceId": "northstarida.xtapro.k8s.edge"
  },
  "spec": {
    "totalGiB": 1862,
    "availableGiB": 1862
  }
}
```

**GPU** — example of a hardware peripheral capability
```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "CapabilityState",
  "metadata": {
    "capability": "capability.margo.org/hardware/gpu",
    "deviceId": "northstarida.xtapro.k8s.edge"
  },
  "spec": {
    "gpus": [
      {
        "devicePath": "/dev/nvidia0",
        "model": "NVIDIA RTX 4090",
        "architecture": "ada",
        "vramGiB": 24,
        "allocated": false,
        "allocatedBy": ""
      }
    ]
  }
}
```

**Network Interfaces** — example of a communication interface capability
```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "CapabilityState",
  "metadata": {
    "capability": "capability.margo.org/interface/network",
    "deviceId": "northstarida.xtapro.k8s.edge"
  },
  "spec": {
    "interfaces": [
      {"type": "ethernet", "name": "eth0"},
      {"type": "wifi", "name": "wlan0"}
    ]
  }
}
```

**Rules:**
- A `DeviceManifest` MUST be accepted by the WFM before any `CapabilityState`
  is accepted for that device
- Each `capability` in `metadata` MUST reference a URI that exists in the
  Capability Registry
- Each `spec` block MUST conform to the referenced
  `CapabilityDefinition.sourceState.schema`
- Devices MUST resend a `CapabilityState` document whenever the state of
  that capability changes — for example when a GPU becomes allocated or a
  port becomes occupied
- Devices MUST send `CapabilityState` for `compute/cpu`, `compute/memory`,
  and `compute/storage` if their role is `Standalone Device`, `Standalone
  Cluster`, or `Cluster Leader`
- Each `CapabilityState` document is independent — updating one capability's
  state does not require resending others