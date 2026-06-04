[<- Home](part1-providers.md#top)
# Specification Update Proposal — Providers-Based Resource Resolution - Part 2 : Device Capabilities

### Device Capabilities Extension
We are extending the existing `DeviceCapabilitiesManifest` with an additive `providers` field that enables devices to advertise dynamic provider state to the WFM. It defines the advertisement structure, the update lifecycle. No new API endpoints are introduced — this SUP uses the existing `POST/PUT /api/v1/clients/{clientId}/capabilities` endpoint.

Without this advertisement mechanism, the WFM cannot perform pre-dispatch validation or delegate the validation to device, and the Provider concept has no operational foundation.


### Extended `DeviceCapabilitiesManifest` Schema

The existing manifest is extended with one new top-level field: `providers`. All existing fields remain unchanged.

```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "DeviceCapabilitiesManifest",
  "properties": {
    "id": "northstarida.xtapro.k8s.edge",
    "vendor": "Northstar Industrial Devices",
    "modelNumber": "332ANZE1-N1",
    "serialNumber": "PF45343-AA",
    "roles": ["standalone cluster"],
    "resources": {
      "cpu": [{"cores": 24, "architecture": "x86_64"}],
      "memory": "59Gi",
      "storage": "1862Gi",
      "peripherals": [{"type": "gpu", "manufacturer": "NVIDIA"}],
      "interfaces": [{"type": "ethernet"}, {"type": "wifi"}]
    },
    "providers": [
      {
        "provider": "capability.margo.org/network/port",
        "supported": true,
        "constraints": { },
        "currentState": { }
      },
      {
        "provider": "capability.myorganistion.com/hardware/vgpus",
        "schemaURL": "myorganisation.com/provider-schemas/hardware/vgpus-v1.yaml"
        "supported": true,
        "constraints": { },
        "currentState": { }
      },
      {
        "provider": "capability.myorganistion.com/k8s-networking",
        "schemaURL": "myorganisation.com/provider-schemas/k8s-networking-v1.yaml"
        "supported": true,
        "constraints": { },
        "currentState": { }
      }
    ]
  }
}
```

### `ProviderAdvertisement` Structure

Each entry in the `providers` array is a `ProviderAdvertisement` object:

| Field | Type | Required? | Description |
|---|---|---|---|
| `provider` | string | Y | The provider URI as defined in SUP 1. Must follow the `capability.<namespace>/<category>/<name>` convention. |
| `supported` | boolean | Y | Whether this device supports the provider. If `false`, the device is declaring awareness of the provider type but inability to fulfill it. The WFM MUST NOT dispatch deployments requiring this provider to this device. |
| `schemaURL` | string | N | Required for custom (non-`margo.org`) providers. URL where the WFM can fetch the `ProviderSchema` document. The WFM MUST fetch and cache this schema before validating deployments against it, or this can be side-loaded into the WFM. |
| `constraints` | object | N | Provider-specific static constraints describing what this device can offer. Structure defined by the individual provider SUP. |
| `currentState` | object | N | Provider-specific dynamic allocation state describing what is currently allocated. Structure defined by the individual provider SUP. |

**Note:** I was planning to propose another field like `reportInterval` in `ProviderAdvertisment` so that synchronization time of a capability can be controlled as per need. But it has been escaped due to the increasing complexity concerns.

### Illustrative Provider Advertisements

> ⚠️ The `constraints` and `currentState` field structures below are illustrative only. Normative field definitions are the responsibility of individual provider SUPs.

#### `capability.margo.org/network/port`

```json
{
  "provider": "capability.margo.org/network/port",
  "supported": true,
  "constraints": {
    "allowedPortRanges": [
      2345,
      4095,
      {"start": 8080, "end": 49150}
    ],
    "reservedPorts": [22, 80, 443, 9100]
  },
  "currentState": {
    "allocatedPorts": [
      {
        "port": 8080,
        "protocol": "TCP",
        "allocatedTo": "deployment-app-a-001"
      },
      {
        "port": 9090,
        "protocol": "TCP",
        "allocatedTo": "deployment-app-b-002"
      }
    ]
  }
}
```

#### `capability.margo.org/network/ingress`

```json
{
  "provider": "capability.margo.org/network/ingress",
  "supported": true,
  "constraints": {
    "ingressController": "nginx",
  },
  "currentState": {
    "allocatedRoutes": [
      {
        "hostname": "hello-world.device1.example.com",
        "route": "/",
        "allocatedTo": "deployment-other-app-001"
      }
    ]
  }
}
```

> Note: The `ingressController` field addresses the use case raised in issue #75 where an app vendor does not know if the route hostname or route can collide. Both the device and WFM can pre-validate this.

#### `capability.margo.org/hardware/gpu`

```json
{
  "provider": "capability.margo.org/hardware/gpu",
  "supported": true,
  "constraints": {
    "availableGPUs": [
      {
        "gpuId": "gpu-0",
        "manufacturer": "NVIDIA",
        "modelName": "A100",
        "devicePath": "/dev/nvidia0",
        "vramGiB": 40,
        "computeMode": "DEFAULT"
      },
      {
        "gpuId": "gpu-1",
        "manufacturer": "AMD",
        "modelName": "Unknown",
        "devicePath": "/dev/amd1",
        "vramGiB": 8,
        "computeMode": "DEFAULT"
      }
    ]
  },
  "currentState": {
    "allocatedGPUs": [
      {
        "gpuId": "gpu-1",
        "allocatedTo": "deployment-transcoder-001"
      }
    ]
  }
}
```

#### Custom Provider — CAN Bus (Vendor-Defined)

```json
{
  "provider": "capability.someorganisation.com/canbus/channel",
  "supported": true,
  "schemaURL": "https://someorganisation.com/margo/providers/canbus.v1.yaml",
  "constraints": {
    "availableChannels": [0, 1, 2, 3],
    "supportedBaudRates": [250000, 500000, 1000000]
  },
  "currentState": {
    "allocatedChannels": [
      {
        "channel": 0,
        "baudRate": 500000,
        "allocatedTo": "deployment-motor-controller-001"
      }
    ]
  }
}
```

> Note: `schemaURL` is required for all non-`margo.org` providers. The WFM MUST fetch and cache the schema from this URL before validating any deployment that references this provider URI. The schemas can be side loaded into the WFM as well.

> **⚠️ Illustrative Provider Advertisements — Not Normative**
>
> The provider advertisement examples in this document are illustrative only. The specific fields within each provider's `constraints` and `currentState` blocks will be defined by individual provider SUPs following ratification of SUP 1.

[<- Home](part1-providers.md#top)