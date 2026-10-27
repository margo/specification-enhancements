# Specification Update Proposal: Native deployment type for OS package and image updates

## Owner

- [@samuel-greising](https://github.com/samuel-greising)
- [@MayerFlorian](https://github.com/MayerFlorian)

## Summary

Today a Margo fleet manager can deploy containerized workloads to a device, but it cannot update the device's operating system. Operators patch devices by logging in to each one or with tools outside Margo.

This SUP adds a third deployment profile type, working name `native`, next to `helm` and `compose`. A `native` deployment profile lists operating system packages or OS images, each with a source URL and a version. The device vendor or operator publishes it as a versioned Application Description, and the fleet manager distributes it exactly like a workload. Native and workload deployments share the same deployment list in the desired state. The device's WFM client applies the listed components, for example with apt on Debian or an A/B image update on Yocto, and reports the result through the existing deployment status.

The proposal reuses the existing Application Description, desired state and deployment status structures. Apart from the new type, the only schema change is a component properties class that accepts non-OCI sources and non-SemVer versions.

## Reason for proposal

Keeping devices on patched OS versions is a basic operational requirement for industrial edge fleets. Security patches for libraries such as OpenSSL have to reach every affected device. The [OS/FW Lifecycle Management requirement](https://github.com/margo/product_management/issues/88) describes the need from the IT Service Provider's point of view.

The proposal deliberately starts small. A deployment of OS packages has the same shape as a workload deployment: a list of components with a source and a version. The fleet manager needs no new APIs, no new desired state document and no knowledge of package formats. This lets the community validate the approach in real deployments before designing a complete device management model.

## Requirements alignment acknowledgement

This SUP is associated with the [OS/FW Lifecycle Management requirement (product_management#88)](https://github.com/margo/product_management/issues/88) and addresses parts of these features:

| Feature | Coverage in this SUP |
|---------|----------------------|
| [#92 Fetch updates from the policy's designated source](https://github.com/margo/product_management/issues/92) | Covered for the MVP. The source comes from the component, not from a separate policy object. |
| [#102 Selective package and image update control](https://github.com/margo/product_management/issues/102) | Partly covered. Operators choose packages and pin versions by what they put into a `native` deployment profile. |
| [#106 Device OS version and update status reporting](https://github.com/margo/product_management/issues/106) | Partly covered. The outcome of each update is reported through the existing deployment status. Reporting installed OS and kernel versions is not covered. |
| [#69 Register a device vendor repository and device artifacts](https://github.com/margo/product_management/issues/69) | Covered through the existing mechanism for publishing Application Descriptions. |

Out of scope for this SUP, and left to later SUPs:

- major OS version upgrades, for example Debian 12 to Debian 13
- bare-metal installation and initial provisioning of a device
- host configuration such as DNS and NTP
- restricting a native deployment to devices running a matching operating system
- reboot control ([#103](https://github.com/margo/product_management/issues/103), [#104](https://github.com/margo/product_management/issues/104))
- removing a native deployment, and what happens to the installed software when a native deployment leaves the desired state
- policy-driven autonomous patching without a deployment from the fleet manager, which needs new policy resources in the desired state ([#91](https://github.com/margo/product_management/issues/91), [#99](https://github.com/margo/product_management/issues/99), [#90](https://github.com/margo/product_management/issues/90))
- managed update sources with stored credentials ([#89](https://github.com/margo/product_management/issues/89))
- inventory of installed packages, drift detection and update history ([#100](https://github.com/margo/product_management/issues/100), [#101](https://github.com/margo/product_management/issues/101))
- post-update health validation and rollback ([#105](https://github.com/margo/product_management/issues/105))
- dependency resolution and package management by Margo itself

**Relation to the deployment specification decision.** If the governance model for deployment specifications ([product_management#97](https://github.com/margo/product_management/issues/97), [#118](https://github.com/margo/product_management/issues/118), [#119](https://github.com/margo/product_management/issues/119)) is in place before this SUP is voted on, the SUP group will submit the technical proposal below as a Draft deployment specification with the identifier `native`. The technical content stays the same.

## Technical proposal

The key words MUST, MUST NOT, SHOULD and MAY in this section are to be interpreted as described in RFC 2119.

### Overview

```text
Device vendor or operator         Fleet manager                 WFM client on the device
-------------------------         -------------                 ------------------------
publishes Application     ---->   matches the native     ---->  applies the components
Description with a                profile against device         (apt, image update, ...)
native deployment profile         capabilities, adds it to       reports deployment status
                                  the desired state
```

The fleet manager handles a `native` deployment like any other deployment and does not interpret its components. The WFM client applies them.

### Affected specification files

| File | Change |
|------|--------|
| `src/specification/applications/application-description.linkml.yaml` | Add `native` to the `type` slot. Add `NativeDeploymentProfile`, `NativeComponent` and `NativeComponentProperties`. |
| `src/specification/margo-management-interface/desired-state.linkml.yaml` | Same additions as in the Application Description schema, which keeps its own copy of the deployment profile classes. |
| `src/specification/margo-management-interface/resources/examples/valid/` | Add a valid desired state example with a `native` deployment profile. |
| `system-design/specification/margo-management-interface/device-capabilities.md` | Add `native` to `SupportedDeploymentType`. |
| `system-design/specification/margo-devices/device-requirements.md` | Add a section "Native-enabled device requirements". |
| `system-design/specification/margo-management-interface/deployment-status.md` | Add the meaning of `installed` and `failed` for native components. |

### Schema changes

The `type` slot of `DeploymentProfile` gets the permitted value `native`:

```yaml
type:
  pattern: ^(helm|compose|native|custom)$
```

Description text to add to the `type` slot:

> `native` indicates that the deployment profile installs or updates software of the device's operating system, either as individual packages or as a complete OS image. When installing the application on a device supporting native deployments, the fleet manager provides all `native` components, and only `native` components, to the device. How a native component is applied is defined by the device's WFM client.

New classes, added to both the Application Description and the desired state schema:

```yaml
NativeDeploymentProfile:
  is_a: DeploymentProfile
  slot_usage:
    type:
      equals_string: "native"
    components:
      range: NativeComponent

NativeComponent:
  is_a: Component
  description: >-
    An operating system package or OS image to install on the device.
    `name` is an identifier that the WFM client interprets, for example a
    Debian package name or the name of an OS image.
  attributes:
    properties:
      range: NativeComponentProperties

NativeComponentProperties:
  description: >-
    Location and version of a native component. The WFM client interprets
    both values according to the device's operating system.
  attributes:
    source:
      description: >-
        URL of the component's source, for example a package repository,
        an OCI registry or a file on an HTTPS or SFTP server. The value MUST
        be a valid URL. The scheme is not restricted, and the WFM client
        interprets the value.
      range: uri
      required: true
    version:
      description: >-
        Version of the component to install, in the format used by the
        device's package manager or image tooling (for example
        "3.0.15-1~deb12u1" for a Debian package). The value is an opaque
        string to the fleet manager.
      range: string
      required: true
```

`NativeComponentProperties` is separate from `ComponentProperties` because `ComponentProperties.repository` requires the `oci://` scheme and `ComponentProperties.revision` requires SemVer. OS package versions such as Debian's do not follow SemVer.

### Device capabilities and matching

A new permitted value is added to `SupportedDeploymentType`:

| Permissible value | Description |
|-------------------|-------------|
| native | Device can receive and process native OS package or image deployments locally. |

Native deployments do not need a workload runtime, so this SUP adds no value to `SupportedRuntime`.

If the [schema-agnostic device profile SUP](https://github.com/margo/specification-enhancements/pull/98) is approved, the native deployment type describes its matching data as a characteristic, for example `margo.org/deployment/native`. This SUP does not standardize such a characteristic.

### Deployment status for native components

How the WFM client applies native components, resolves dependencies or handles errors is outside the scope of Margo. The WFM client reports the result through the existing deployment status. Two states need a definition for native components:

- `installed` means the component has been applied. If the change only takes effect after a reboot, for example an image written to the inactive slot, the component is `installed` once the change is staged. Triggering the reboot and reporting a pending reboot are outside the scope of this SUP.
- `failed` means the WFM client could not apply the component. The WFM client MUST report the error it received while applying the component in the component's `error` element.

### Native-enabled device requirements

Text to add to the device requirements page:

> Devices supporting the native deployment type MUST apply the components of the native deployments they receive and report their deployment status as defined for native components.

### Conflicting deployments

Two Compose deployments can run side by side. Two native deployments that ask for different versions of the same package or image cannot. Margo does not resolve conflicts between native deployments. If applying a component fails because of such a conflict, the WFM client reports the component as `failed` with the error it received.

### Granularity

Margo does not prescribe how device or OS vendors split packages across Application Descriptions. For example, a vendor can publish one Application Description per OS release, or a base release plus smaller ones for individual patches.

### Security

Authentication against package sources and verification of package signatures are not defined by this SUP. The [MIAF credential SUPs](https://github.com/margo/specification-enhancements/pull/95) and the [OCI artifact signing SUP](https://github.com/margo/specification-enhancements/pull/100) cover OCI sources. The SUP group will align with those SUPs and propose extensions where native sources need them.

### Example: security patch on a Debian-based device

A device vendor publishes an OpenSSL security patch for Debian 12.

#### Application Description

```yaml
apiVersion: v1
id: com-example-devices-os-security-2026-11
metadata:
  name: Debian 12 security update 2026-11
  description: OpenSSL security update for Debian 12
  version: "2026.11.0"
  catalog:
    organization:
      - name: Example Devices
        site: https://devices.example.com
deploymentProfiles:
  - type: native
    id: com-example-devices-os-security-2026-11-debian12
    components:
      - name: openssl
        properties:
          source: https://mirror.devices.example.com/debian/bookworm-security
          version: "3.0.15-1~deb12u1"
      - name: libssl3
        properties:
          source: https://mirror.devices.example.com/debian/bookworm-security
          version: "3.0.15-1~deb12u1"
```

#### Device capabilities

The relevant fields of the device's capabilities:

```json
{
    "properties": {
        "supportedDeploymentTypes": ["compose", "native"]
    }
}
```

The device supports `native`, so the fleet manager can add the deployment to its desired state.

### Example: image update on a Yocto-based device

An image-based device uses the same structure with a single component. In this example, a Yocto-based gateway uses [RAUC](https://rauc.io/) with an A/B partition layout. The vendor publishes a signed RAUC bundle on its update server:

```yaml
deploymentProfiles:
  - type: native
    id: com-example-gateway-os-image-5-2-0
    components:
      - name: example-gateway-os
        properties:
          source: https://updates.devices.example.com/gateway/example-gateway-os-5.2.0.raucb
          version: "5.2.0"
```

Here the WFM client passes the bundle to RAUC, which writes the image to the inactive slot. The client reports the component as `installed` once the image is staged. Devices using SWUpdate or Mender follow the same pattern with their own tooling.

### Use cases

To be added by the SUP group before the SUP is ready for review. Planned use cases:

1. A full OS update from the device vendor, applied to a device group
2. A single security patch rolled out to the affected devices only
3. An image-based device updated with an A/B partition scheme
4. A failed update because of an unresolvable dependency, and how the operator sees it

Each use case describes the Application Description, the expected fleet manager behaviour and the expected WFM client behaviour.

### Conformance

Conformance tests for native deployments cover the Margo interface: the device reports `native` in `supportedDeploymentTypes`, and the WFM client reports the deployment status of native components as defined above.

### Implementation

Before the SUP is ready for review, the SUP group will provide a proof of concept covering a fleet manager and a WFM client, and a branch of the [sandbox](https://github.com/margo/sandbox) that implements the WFM client side for a Debian-based device.

## Breaking Changes

None. Existing `helm`, `compose` and `custom` deployments are unchanged. Fleet managers and devices that do not know `native` ignore it, as the extensibility section of the specification already requires for unknown deployment types.

## Alternatives considered (optional)

**Use the existing `custom` deployment type.** Rejected. A `custom` profile only has meaning between suppliers who agree on it, so OS updates would not be interoperable across fleet managers and devices. `custom` also requires artifacts to be packaged as OCI blobs, but OS packages normally live in package repositories.

**A separate device management API and desired state document.** A complete device fleet management architecture with its own resources is the likely long-term direction, but agreeing on it would take much longer. This SUP covers the MVP with existing structures and does not block a later dedicated design.

## Rejection reason

Not applicable.
