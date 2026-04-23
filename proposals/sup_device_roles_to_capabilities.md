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

Devices MUST provide the Workload Fleet Management service with both their resources and their capabilities. Resources are quantitative attributes (CPU, memory, storage) used for workload scheduling. Capabilities describe what the device can do — which runtimes it supports, which manifest types it can process, and any vendor-specific or hardware-accelerated features it provides.

This proposal replaces the `roles` field in the current `DeviceCapabilitiesManifest` with a `capabilities` object, while keeping the `resources` object unchanged.

## Requirements Changes

The `roles` field in the current `DeviceCapabilitiesManifest` `properties` schema is replaced by a `capabilities` object. All references to static device roles (Standalone Cluster, Cluster Leader, Standalone Device) in the specification will be updated following this SUP.

All previously hard-coded device requirements (WFM client, OCI runtime, OTEL collector) are now expressed as capabilities within the `capabilities` object. The WFM client and OTEL collector are reported as boolean flags, while OCI runtime support is captured through `supportedRuntimes` or `vendorRuntimes`. A device MUST report at least one entry across `supportedRuntimes` or `vendorRuntimes`, and MUST report at least one `supportedManifest`.

---

## Device-Specific Runtimes (active CURRENT SUP)

Devices MUST report vendor-specific runtimes and their named capabilities using the `vendorRuntimes` field in the `capabilities` object. Application description files may specify required vendor runtimes and capabilities. The WFM MUST validate that a device has the required vendor runtime and capabilities before dispatching a deployment.

---

## Gateway and Compound Devices(APPROVED SUP)

Gateway devices MUST report their capabilities to the WFM as a single `DeviceCapabilitiesManifest`, reflecting the aggregate capabilities of all sub-devices they manage. Gateway operational modes:

* **Autonomous** — the gateway decides which sub-device handles each deployment.
* **Directed** — the WFM dictates the sub-device for each deployment.
* **Mixed** — the WFM directs some placements and the gateway decides the rest.

An opaque gateway presents the combined capabilities of its sub-devices and supports only the autonomous mode.

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

A device reports its runtimes using one of two mechanisms:

* **`supportedRuntimes`** — for well-known, Margo-standard OCI runtimes (docker, podman, kubernetes).
* **`vendorRuntimes`** — for vendor-specific runtime environments that extend or replace the standard runtimes. A device MAY report only `vendorRuntimes` if none of the standard `SupportedRuntimeType` values apply.

A device MUST report at least one entry across `supportedRuntimes` or `vendorRuntimes`.

| Attribute         | Type               | Required? | Description |
|-------------------|--------------------|-----------|-------------|
| wfmClient         | WfmClient          | Y         | Reports the WFM client present on the device. See [WfmClient Attributes](#wfmclient-attributes). |
| otelCollector     | OtelCollector      | Y         | Reports the OTEL collector present on the device. See [OtelCollector Attributes](#otelcollector-attributes). |
| supportedRuntimes | []SupportedRuntime | N*        | Standard Margo OCI runtimes available on the device. See [SupportedRuntime Attributes](#supportedruntime-attributes). Required if no `vendorRuntimes` are reported. |
| supportedManifests| []SupportedManifestType | Y    | Manifest/deployment formats the device can receive and process locally. See [SupportedManifestType](#supportedmanifesttype). |
| vendorRuntimes    | []VendorRuntime    | N*        | Vendor-specific runtimes, used as an alternative or supplement to `supportedRuntimes` when standard runtime types do not apply. See [VendorRuntime Attributes](#vendorruntime-attributes). Required if no `supportedRuntimes` are reported. |

> \* At least one of `supportedRuntimes` or `vendorRuntimes` MUST be present.

### WfmClient Attributes

| Attribute | Type   | Required? | Description |
|-----------|--------|-----------|-------------|
| present   | boolean | Y        | Indicates whether a Margo-compliant WFM client is installed and running on the device. |

### OtelCollector Attributes

| Attribute | Type    | Required? | Description |
|-----------|---------|-----------|-------------|
| present   | boolean | Y         | Indicates whether an OTEL collector is installed and running on the device. |

### SupportedRuntime Attributes

Use `supportedRuntimes` when the device runtime is one of the standard Margo-defined OCI runtime types. For runtimes not covered by `SupportedRuntimeType`, use `vendorRuntimes` instead.

| Attribute | Type              | Required? | Description |
|-----------|-------------------|-----------|-------------|
| type      | SupportedRuntimeType | Y      | Example: The standard OCI-compatible runtime available on the device. See [SupportedRuntimeType](#supportedruntimetype). |

### VendorRuntime Attributes

Use `vendorRuntimes` when the device provides a runtime environment not covered by the standard `SupportedRuntimeType` values, or when additional vendor-specific metadata and capabilities must be advertised alongside the runtime. This is the extension point for proprietary, platform-specific, or industrial runtime environments.

This section aligns with the device-specific runtime SUP. See ongoing SUP details [here](https://github.com/margo/specification-enhancements/pull/55).

| Attribute            | Type                   | Required? | Description |
|----------------------|------------------------|-----------|-------------|
| runtimeRef           | string                 | Y         | Stable identifier for the vendor-specific runtime environment (e.g., reverse-domain scoped, such as `com.northstar-ida.industry.edge-os`). |
| vendor               | string                 | N         | Human-readable vendor name. |
| vendorId             | string                 | N         | Stable machine-readable vendor namespace identifier. Runtime and capability identifiers SHOULD be scoped under this value (e.g., `com.northstar-ida`). |
| modelNumberId        | string                 | N         | Stable model number identifier from a shared dictionary. |
| version              | string                 | N         | Installed version of the runtime environment. If an application declares a version constraint, the WFM MUST evaluate it against this value. |
| runtimeCapabilities  | []string               | N         | List of runtime-specific capability identifiers available on the device. If an application declares required `runtimeCapabilities`, the WFM MUST compare them against this list. |
| properties           | map[string]any         | N         | Optional extensibility map for runtime-specific metadata (e.g., OS version, driver versions). Not part of the core matching model. |

If a `runtimeEnvironment`, its `version`, or its `runtimeCapabilities` change, the device client MUST update the `DeviceCapabilitiesManifest` reported to the WFM.

---

## Enumerations

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

### SupportedRuntimeType

| Permissible Values | Description |
|--------------------|-------------|
| docker             | Docker OCI container runtime. |
| podman             | Podman OCI container runtime. |
| kubernetes         | Kubernetes container orchestration runtime. |

### SupportedManifestType

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
            "wfmClient": {
                "present": true
            },
            "otelCollector": {
                "present": true
            },
            "supportedRuntimes": [
                { "type": "kubernetes" },
                { "type": "podman" }
            ],
            "supportedManifests": [
                "helm",
                "compose"
            ],
            "vendorRuntimes": [
                {
                    "runtimeRef": "com.acme.special-rt",
                    "vendor": "Acme Corp",
                    "vendorId": "com.acme",
                    "modelNumberId": "com.acme.edge-devices.X1000",
                    "version": "3.1.0",
                    "runtimeCapabilities": [
                        "foo",
                        "bar"
                    ],
                    "properties": {
                        "osVersion": "Debian 12"
                    }
                }
            ]
        }
    }
}
```

## Alternatives considered (optional)

N/A

## Rejection reason