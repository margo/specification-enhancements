# Specification Update Proposal

## Owner

Armand Craig
@ajcraig
acraig@project.margo.org

## Summary

This SUP's intent is to replace device `roles` with a more flexible `capabilities` framework to enable expanded use-cases and ensure we are not over prescribing "devices" within the specification. 

## Reason for proposal
The goal of this SUP is to improve the specification sections revolving around Margo devices. The original idea for devices was to assign a [`device role`](https://docs.margo.org/specification/margo-devices/device-requirements) to a particular device. This enabled us to quickly frame out the first two device roles, `Standalone cluster` and `Standalone device`. However, as the community expands use cases related to devices, the roles concept is limiting the flexibility of the architecture. 

Groups of functionality and/or configuration of the device will enable a unique capability that is reported to the WFM during device onboarding. This will enable the WFM to understand what types of manifests/workloads/configurations are compatible with the newly onboarded device. 

Relevant concepts that drove this SUP:
- Approval of the Gateway SUP
    - https://github.com/margo/specification-enhancements/pull/36
- Concept of multi functional device
    - i.e. device that supports one to many application manifests
- Enable constrained device types
    - WASM / leaf devices
- Enable freedom for device and platform vendors to create unique offerings in the field while not restricting management capabilities via Margo.

 including gateway functionality, multi functional devices, 

## Requirements alignment acknowledgement

This SUP aims to close out the following TWG Feature:
- [Replace device roles with capabilities within Margo](https://github.com/margo/specification/issues/136)

## Technical proposal
This section provides technical details for the proposal to replace device roles with a flexible capabilities framework in the Margo specification. It incorporates and aligns with concurrent proposals and reference materials, including extended device capabilities, gateway, and device-specific runtime requirements.
> Future integration might be needed following the device specific parameter values. 
    - See [here](https://github.com/margo/specification-enhancements/pull/54)

This proposal builds on the current device capability reporting specification. See: [Device Capabilities Spec Section](https://docs.margo.org/specification/margo-devices/device-requirements) along with the [device capability management interface function](https://docs.margo.org/specification/margo-management-interface/device-capabilities)

---

## Device Model: Resources vs. Capabilities

Devices MUST provide the Workload Fleet Management service with both their resources and their capabilities. Resources are quantitative attributes (CPU, memory, storage) used for workload scheduling. Capabilities describe what the device can do — which runtimes it supports, and which manifest types it can process.

This proposal replaces the `roles` field in the current `DeviceCapabilitiesManifest` with a `capabilities` object, while keeping the `resources` object unchanged.

## Requirements Changes

The `roles` field in the current `DeviceCapabilitiesManifest` `properties` schema is replaced by a `capabilities` object. All references to static device roles (Standalone Cluster, Cluster Leader, Standalone Device) in the specification will be updated following this SUP.

All previously hard-coded device requirements (WFM client, OCI runtime, OTEL collector) are now expressed as capabilities within the `capabilities` object. The WFM client and OTEL collector are reported as boolean flags, while OCI runtime support is captured through `supportedRuntimes`. A device MUST report at least one entry across `supportedRuntimes`, and MUST report at least one `supportedDeploymentTypes`.

If any element within the device capabilities change, the device client MUST update the `DeviceCapabilitiesManifest` reported to the WFM.

---

## Device-Specific Runtimes (active CURRENT SUP)

> Note: this SUP is related to the following SUP, however, we will reinject the integration at a later point if/once the [Device specific runtimes SUP](https://github.com/margo/specification-enhancements/pull/55) is approved. 

---

## Gateway and Compound Devices(APPROVED SUP)

>Note: The Gateway SUP was approved, and is now in the stage of being injected into the specification. If this SUP, roles to capabilities, gets approved, we will need to update the capabilities the Gateway reports. For both Opaque and See-thru configurations. 

### Opaque gateway function

Opaque Gateway devices MUST report their capabilities to the WFM as a single `DeviceCapabilitiesManifest`, reflecting the aggregate capabilities of all sub-devices they manage. Gateway operational modes:

* **Autonomous** — the gateway decides which sub-device handles each deployment.
* **Directed** — the WFM dictates the sub-device for each deployment.
* **Mixed** — the WFM directs some placements and the gateway decides the rest.

An opaque gateway presents the combined capabilities of its sub-devices and supports only the autonomous mode.

### See-thru gateway function

See-thru gateway devices must report their capabilities individually, as outlined in this proposal. 

Located [here](https://github.com/margo/specification-enhancements/blob/main/proposals/gateway.md#see-thru-gateway) is the approved SUP that describes how the see-thru devies report their capabilities.


---

### Route and HTTP Methods (UNCHANGED)

```https
POST /api/v1/clients/{clientId}/capabilities
PUT /api/v1/clients/{clientId}/capabilities
```

### Request Body Attributes (UNCHANGED)

| Field       | Type       | Required? | Description |
|-------------|------------|-----------|-------------|
| apiVersion  | string     | Y         | Identifier of the version the API resource follows. |
| kind        | string     | Y         | Must be `DeviceCapabilitiesManifest`. |
| properties  | Properties | Y         | Element that defines characteristics about the device. See the [Properties Attributes](#properties-attributes) section below. |

### Properties Attributes

| Field        | Type           | Required? | Description |
|--------------|----------------|-----------|-------------|
| id           | string         | Y         | Unique deviceID assigned to the device via the Device Owner. |
| vendor       | string         | Y         | Defines the device vendor. |
| modelNumber  | string         | Y         | Defines the model number of the device. |
| serialNumber | string         | Y         | Defines the serial number of the device. |
| resources    | Resource       | Y         | Element that defines the device's resources available to applications. See the [Resources Attributes](#resources-attributes) section below. Unchanged from current specification. |
| capabilities | Capabilities   | Y         | **Proposed addition.** Replaces `roles`. Defines what the device can do. See the [Capabilities Attributes](#capabilities-attributes) section below. |

### Capabilities Attributes

**Proposed addition.** Replaces the `roles` field. Describes what the device can do and what types of workloads it can receive and process.

#### New Capability items in proposal

**`supportedRuntimes`** — for well-known, Margo-standard OCI runtimes (docker, podman, kubernetes).
- A device MUST report at least one `supportedRuntimes`.

**`supportedDeploymentTypes`** — for well-known, Margo-standard manifests (helm, compose).
- A device MUST report at least one `supportedDeploymentTypes`.


| Attribute         | Type               | Required? | Description |
|-------------------|--------------------|-----------|-------------|
| wfmClient         | boolean         | Y         | Reports the WFM client present on the device. |
| otelCollector     | boolean     | Y         | Reports the OTEL collector present on the device. |
| supportedRuntimes | array | Y        | Standard Margo OCI runtimes available on the device. See [SupportedRuntimes](#supportedruntime). |
| supportedDeploymentTypess| array | Y    | Manifest/deployment formats the device can receive and process locally. See [supportedDeploymentTypes](#supportedDeploymentTypes). |

## Enumerations

> Note: These enumerations can be expanded in the future through the SUP process. This SUP aims to set the framework for future expansion. 

### CpuArchitectureType

| Permissible Values | Description |
|--------------------|-------------|
| amd64              | AMD 64-bit architecture. |
| x86_64             | x86 64-bit architecture. |
| arm64              | ARM 64-bit architecture. |
| arm                | ARM 32-bit architecture. |

### CommunicationInterfaceType

| Permissible Values | Description |
|--------------------|-------------|
| ethernet           | Ethernet interface. |
| wifi               | WiFi interface. |
| cellular           | Cellular (5G, LTE, 3G, 2G). |
| bluetooth          | Bluetooth or BLE interface. |
| usb                | USB interface. |
| canbus             | CANBus interface. |
| rs232              | RS232 interface. |

### PeripheralType

| Permissible Values | Description |
|--------------------|-------------|
| gpu                | Graphics Processing Unit (GPU). |
| display            | Display peripheral. |
| camera             | Camera peripheral. |
| microphone         | Microphone peripheral. |
| speaker            | Speaker peripheral. |

### SupportedRuntimes

| Permissible Values | Description |
|--------------------|-------------|
| oci            | OCI container runtime |

### supportedDeploymentTypess

| Permissible Values | Description |
|--------------------|-------------|
| helm               | Device can receive and process Kubernetes Helm chart deployments locally. |
| compose            | Device can receive and process Docker Compose service deployments locally. |

---

## Example Device Capabilities Payload

```json
{
    "apiVersion": "device.margo.org/v1alpha1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
        "id": "northstarida.xtapro.k8s.edge",
        "vendor": "Northstar Industrial Devices",
        "modelNumber": "332ANZE1-N1",
        "serialNumber": "PF45343-AA",
        "resources": {
            "cpu": [
                {
                    "cores": 24,
                    "architecture": "x86_64"
                }
            ],
            "memory": "59 Gi",
            "storage": "1862 Gi",
            "peripherals": [
                {
                    "type": "gpu",
                    "manufacturer": "NVIDIA"
                }
            ],
            "interfaces": [
                {
                    "type": "ethernet"
                },
                {
                    "type": "wifi"
                }
            ]
        },
        "capabilities": {
            "wfmClient": true,
            "otelCollector": true,
            "supportedRuntimes": [
                "oci"
            ],
            "supportedDeploymentTypes": [
                "helm",
                "compose"
            ]
        }
    }
}
```

## Alternatives considered (optional)

N/A

## Rejection reason