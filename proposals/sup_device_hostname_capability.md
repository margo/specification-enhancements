# Specification Update Proposal

## Owner

@oomichi-melco

## Summary

This SUP proposes that edge devices report their network `hostname` as part of the `DeviceCapabilitiesManifest` during the onboarding process, alongside the other capabilities (CPU, memory, storage, peripherals, interfaces, etc.). Today a device's hostname is not part of its core identity data reported to the Workload Fleet Manager(WFM). This SUP adds it as a direct field of `properties`, reported at the same time and through the same route as the rest of the device's capabilities. Managing `hostname` alongside `id`, `vendor`, and other identity fields also makes it easier for operators to pinpoint and reach a specific edge device — for example when triaging a device failure in the field.

## Reason for proposal

The `DeviceCapabilitiesManifest` reported during onboarding (see [Device Roles to Capabilities SUP](https://github.com/margo/specification-enhancements/blob/main/completed/sup_device_roles_to_capabilities.md)) already includes identity and descriptive fields such as `id`, `vendor`, `modelNumber`, `serialNumber`, `cpu`, `memory`, `storage`, `peripherals`, and `interfaces`. A device's network hostname is the same kind of identity/network information, but it is currently missing from this manifest. Without it, the WFM (and operators/tools consuming the WFM's device inventory) has no standard way to know a device's hostname unless it is separately derived through an application-specific mechanism.

Reporting `hostname` alongside `id`, `vendor`, `modelNumber`, and `serialNumber` also gives operators a network-addressable identifier for each entry in the device inventory. When an edge device fails or becomes unreachable, this makes it faster to identify and locate the specific physical device on the network (e.g. to reach it directly, cross-reference it against DHCP/DNS records, or dispatch a technician), instead of having to correlate an opaque device `id` back to a network host through a separate, application-specific lookup.

This is different from the [Device-specific parameter values SUP](https://github.com/margo/specification-enhancements/blob/main/proposals/device-specific-parameter-values.md), which proposes a `margo.cluster.hostname` key under `installContext` in the `DeviceCapabilitiesManifest`. That mechanism exists to resolve `ApplicationDescription` parameters (`valueFrom.device`) at install/update time, and is only populated when an application declares a dependency on a device-supplied value. The field proposed here is unconditional: it is reported at onboarding as core device identity, independent of whether any application requests it. The two proposals can coexist — a future revision of the device-specific-parameter-values SUP could source `margo.cluster.hostname`'s value from `properties.hostname` instead of, or in addition to, maintaining it independently under `installContext`, but that integration is left for the two SUP groups to coordinate and is out of scope here.

## Requirements alignment acknowledgement

This SUP recommends adding a property `hostname` to the `DeviceCapabilitiesManifest` introduced by the original device capabilities TWG feature: [margo/specification#97](https://github.com/margo/specification/issues/97).

Out of scope for this SUP:
- Application-level parameter resolution/substitution using the device's hostname (covered separately by the [Device-specific parameter values SUP](https://github.com/margo/specification-enhancements/blob/main/proposals/device-specific-parameter-values.md)).
- DNS registration/resolution mechanics for the reported hostname.

## Technical proposal

This proposal extends the `properties` object of the `DeviceCapabilitiesManifest`, as defined by the [Device Roles to Capabilities SUP](https://github.com/margo/specification-enhancements/blob/main/completed/sup_device_roles_to_capabilities.md), with a new optional field: `hostname`. The field is optional, rather than required, so that existing device clients implementing the current schema remain compliant without modification, and so that devices whose hostname is not yet finalized at the time of initial onboarding (e.g. it is assigned later by DHCP/DNS or by an operator) can still report a valid `DeviceCapabilitiesManifest` and add `hostname` in a subsequent update once it is known.

### Route and HTTP Methods (UNCHANGED)

```https
POST /api/v1/clients/{clientId}/capabilities
PUT /api/v1/clients/{clientId}/capabilities
```

### Properties Attributes (new field)

| Field                    | Type              | Required? | Description |
|--------------------------|-------------------|-----------|-------------|
| hostname                 | string            | N         | The device's network hostname (not fully qualified — a single RFC 1123-compliant DNS label, without any domain suffix), if known. Used by the WFM and operator tooling to identify the device on the network. |

If the device's hostname changes, the device client MUST update the `DeviceCapabilitiesManifest` reported to the WFM, consistent with the existing update requirement for other `properties` fields.

### Example Device Capabilities Payload

```json
{
    "apiVersion": "device.margo.org/v1alpha1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
        "id": "northstarida.xtapro.k8s.edge",
        "vendor": "Northstar Industrial Devices",
        "modelNumber": "332ANZE1-N1",
        "serialNumber": "PF45343-AA",
        "hostname": "northstar-edge01",
        "cpu": [
            {
                "cores": 24,
                "architecture": "amd64"
            }
        ],
        "memory": "59 Gi",
        "storage": "1862 Gi",
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
```

## Breaking Changes

None. `hostname` is added as an optional field, so existing device clients implementing the current schema from the [Device Roles to Capabilities SUP](https://github.com/margo/specification-enhancements/blob/main/completed/sup_device_roles_to_capabilities.md) remain schema-compliant without modification, whether or not they report it.

## Alternatives considered (optional)

- **Add `hostname` to each entry of the `interfaces` array** instead of a single `properties.hostname` field, to support multi-homed devices with a distinct hostname per network interface. Not chosen for this proposal to keep the initial design simple and consistent with the single-value `id`/`vendor`/`serialNumber` fields; this can be revisited in a follow-up SUP if multi-hostname use cases are identified.
- **Rely solely on the `margo.cluster.hostname` key in `installContext`**, as proposed by the [Device-specific parameter values SUP](https://github.com/margo/specification-enhancements/blob/main/proposals/device-specific-parameter-values.md). Not chosen because that mechanism only surfaces a hostname when an application explicitly declares a `valueFrom.device` dependency on it, whereas this SUP treats the hostname as core device identity that the WFM can learn directly at onboarding, independent of any application's requirements, whenever the device is able to report it.

## Rejection reason

N/A
