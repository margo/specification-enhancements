# Specification Update Proposal: Deployment type for native workloads

## Owner

- [@samuel-greising](https://github.com/samuel-greising)
- [@MayerFlorian](https://github.com/MayerFlorian)

## Summary

Today a Margo fleet manager can deploy Compose and Helm workloads to a device. Native workloads, software that the device's operating system installs and runs directly without a container runtime, are outside Margo. Operators install and patch them by logging in to each device or with tools outside Margo.

This SUP is a narrow extension of workload deployment, not a device management solution. It proposes a new deployment specification, working name `native`, next to the `helm` and `compose` deployment technologies. It is submitted as a Draft deployment specification. A `native` deployment profile lists native workload components, each with a source URL and a version. The vendor or operator publishes it as a versioned Application Description, and the fleet manager distributes it exactly like any other workload. Native, Compose and Helm deployments share the same deployment list in the desired state. The device's WFM client applies the listed components with the device's own tooling, for example a package manager or system extension images on an immutable OS, and reports the result through the existing deployment status.

The only change to the core specification is the new value `native` for `SupportedDeploymentType`. Everything else lives in the deployment specification. It reuses the existing Application Description, desired state and deployment status structures and adds a component properties class that accepts non-OCI sources and non-SemVer versions.

## Reason for proposal

Industrial edge devices run software outside container runtimes: system services, drivers, libraries and vendor agents. Operators need to keep this software at approved versions across a fleet. A security patch for a library such as OpenSSL has to reach every affected device. The [OS/FW Lifecycle Management requirement](https://github.com/margo/product_management/issues/88) describes the need from the IT Service Provider's point of view.

From the end user's point of view, a native workload is another kind of application to deploy. A deployment of native workloads has the same shape as a Compose or Helm deployment: a list of components with a source and a version. The fleet manager needs no new APIs, no new desired state document and no knowledge of package formats. This lets the community validate the approach in real deployments with a small change.

## Requirements alignment acknowledgement

This SUP is associated with the [OS/FW Lifecycle Management requirement (product_management#88)](https://github.com/margo/product_management/issues/88) and addresses parts of these features:

| Feature | Coverage in this SUP |
|---------|----------------------|
| [#92 Fetch updates from the policy's designated source](https://github.com/margo/product_management/issues/92) | Covered for native workloads. The source comes from the component, not from a separate policy object. |
| [#102 Selective package and image update control](https://github.com/margo/product_management/issues/102) | Partly covered. Operators choose native workloads and pin their versions in a `native` deployment profile. Image updates are not covered. |
| [#106 Device OS version and update status reporting](https://github.com/margo/product_management/issues/106) | Partly covered. The existing deployment status reports the outcome of each native deployment. Reporting installed OS and kernel versions is not covered. |
| [#69 Register a device vendor repository and device artifacts](https://github.com/margo/product_management/issues/69) | Covered through the existing mechanism for publishing Application Descriptions. |

Out of scope for this SUP, and left to later SUPs:

- major OS version upgrades, for example Debian 12 to Debian 13
- OS image updates
- bare-metal installation and initial provisioning of a device
- updating Kubernetes itself
- host configuration such as DNS and NTP
- restricting a native deployment to devices running a matching operating system
- native workloads that need a restart to take effect, and reboot control ([#103](https://github.com/margo/product_management/issues/103), [#104](https://github.com/margo/product_management/issues/104))
- removing a native deployment, and what happens to the installed software when a native deployment leaves the desired state
- policy-driven autonomous patching without a deployment from the fleet manager, which needs new policy resources in the desired state ([#91](https://github.com/margo/product_management/issues/91), [#99](https://github.com/margo/product_management/issues/99), [#90](https://github.com/margo/product_management/issues/90))
- managed update sources with stored credentials ([#89](https://github.com/margo/product_management/issues/89))
- inventory of installed software, drift detection and update history ([#100](https://github.com/margo/product_management/issues/100), [#101](https://github.com/margo/product_management/issues/101))
- post-update health validation and rollback ([#105](https://github.com/margo/product_management/issues/105))
- dependency resolution and package management by Margo itself

**Relation to the deployment specification decision.** The Steering Committee has approved splitting deployment technologies into separate deployment specifications ([product_management#97](https://github.com/margo/product_management/issues/97)). Following that decision, this SUP proposes `native` as a Draft deployment specification with the identifier `native`, not as a change to the core specification. The core change is limited to adding `native` to `SupportedDeploymentType`. If the [schema-agnostic device profile SUP](https://github.com/margo/specification-enhancements/pull/98) is approved, a characteristic replaces that value. Lifecycle and governance of the deployment specification follow [#118](https://github.com/margo/product_management/issues/118) and [#119](https://github.com/margo/product_management/issues/119).
## Technical proposal

The key words MUST, MUST NOT, SHOULD and MAY in this section are to be interpreted as described in RFC 2119.

### Overview

```text
Vendor or operator                Fleet manager                 WFM client on the device
------------------                -------------                 ------------------------
publishes Application     ---->   matches the native     ---->  applies the components with
Description with a                profile against device         the device's own tooling,
native deployment profile         capabilities, adds it to       reports deployment status
                                  the desired state
```

The fleet manager handles a `native` deployment like any other deployment and does not interpret its components. The WFM client applies them.

### Changes to the core specification

| File | Change |
|------|--------|
| `system-design/specification/margo-management-interface/device-capabilities.md` | Add `native` to `SupportedDeploymentType`, or define a characteristic instead if #98 is approved. |

### Content of the `native` deployment specification

The Draft deployment specification defines:

- the `native` value of the deployment profile `type` and the classes `NativeDeploymentProfile`, `NativeComponent` and `NativeComponentProperties`, for both the Application Description and the desired state
- a valid desired state example with a `native` deployment profile
- the meaning of `installed` and `failed` for native components
- the requirements for devices that support native deployments

Where these definitions live in the repository and how they are referenced from the core schemas follows the structure for deployment specifications set up under #97.

### Schema changes

The `type` slot of `DeploymentProfile` gets the permitted value `native`:

```yaml
type:
  pattern: ^(helm|compose|native|custom)$
```

Description text to add to the `type` slot:

> `native` indicates that the deployment profile deploys native workloads: software that the device's operating system installs and runs directly, without a container runtime. When installing the application on a device supporting native deployments, the fleet manager provides all `native` components, and only `native` components, to the device. How a native component is applied is defined by the device's WFM client.

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
    A native workload to install on the device. `name` is an identifier
    that the WFM client interprets, for example a Debian package name.
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
        device's installation tooling (for example "3.0.15-1~deb12u1" for
        a Debian package). The value is an opaque string to the fleet
        manager.
      range: string
      required: true
```

`NativeComponentProperties` is separate from `ComponentProperties` because `ComponentProperties.repository` requires the `oci://` scheme and `ComponentProperties.revision` requires SemVer. Versions of native workloads, such as Debian package versions, do not follow SemVer.

### Device capabilities and matching

A new permitted value is added to `SupportedDeploymentType`:

| Permissible value | Description |
|-------------------|-------------|
| native | Device can receive and process native workload deployments locally. |

Native deployments do not need a workload runtime, so this SUP adds no value to `SupportedRuntime`.

If the [schema-agnostic device profile SUP](https://github.com/margo/specification-enhancements/pull/98) is approved, the native deployment type describes its matching data as a characteristic, for example `margo.org/deployment/native`. This SUP does not standardize such a characteristic.

### Deployment status for native components

How the WFM client applies native components, resolves dependencies or handles errors is outside the scope of Margo. The WFM client reports the result through the existing deployment status. Two states need a definition for native components:

- `installed` means the component has been applied.
- `failed` means the WFM client could not apply the component. The WFM client MUST report the error it received while applying the component in the component's `error` element.

### Native-enabled device requirements

Text for the device requirements of the deployment specification:

> Devices supporting the native deployment type MUST apply the components of the native deployments they receive and report their deployment status as defined for native components.

### Conflicting deployments

Two Compose deployments can run side by side. Two native deployments that ask for different versions of the same native workload cannot. Margo does not resolve conflicts between native deployments. If applying a component fails because of such a conflict, the WFM client reports the component as `failed` with the error it received.

### Granularity

Margo does not prescribe how vendors split native workloads across Application Descriptions. For example, a vendor can publish one Application Description per release of its software stack, or a base release plus smaller ones for individual patches.

### Security

This SUP does not define authentication against sources of native workloads or verification of their signatures. The [MIAF credential SUPs](https://github.com/margo/specification-enhancements/pull/95) and the [OCI artifact signing SUP](https://github.com/margo/specification-enhancements/pull/100) cover OCI sources. The SUP group will align with those SUPs and propose extensions where native sources need them.

### Example: security patch on a Debian-based device

A device vendor publishes an OpenSSL security patch for Debian 12.

#### Application Description

```yaml
apiVersion: v1
id: com-example-devices-openssl-2026-11
metadata:
  name: OpenSSL security update 2026-11
  description: OpenSSL 3.0.15 security update for Debian 12
  version: "2026.11.0"
  catalog:
    organization:
      - name: Example Devices
        site: https://devices.example.com
deploymentProfiles:
  - type: native
    id: com-example-devices-openssl-2026-11-debian12
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

### Example: native workload on an immutable OS

Devices with a read-only root file system, for example Yocto-based images or EVE-OS, cannot install packages into the OS. They use the same structure with a different source. In this example, a Yocto-based gateway adds software as [systemd system extension](https://www.freedesktop.org/software/systemd/man/latest/systemd-sysext.html) images, which the vendor publishes in an OCI registry:

```yaml
deploymentProfiles:
  - type: native
    id: com-example-gateway-monitoring-agent-1-4-0
    components:
      - name: example-monitoring-agent
        properties:
          source: oci://registry.devices.example.com/gateway/example-monitoring-agent
          version: "1.4.0"
```

The WFM client pulls the extension image and activates it with `systemd-sysext refresh`, without a restart. Other immutable systems use their own mechanism for adding software.

### Use cases

Today, edge devices are typically provisioned by installing the operating system and required runtime components. Ongoing maintenance, such as security patches, system updates and runtime updates, still require direct access to individual devices.

The proposed fleet management approach separates initial provisioning from ongoing maintenance. Initial device setup remains outside the scope. Once a device is onboarded, supported native workload lifecycle operations can be managed centrally through the fleet manager.

Instead of maintaining each device individually, operators define the desired native workloads and versions for selected devices. The device applies the requested changes and reports the resulting status and version back to the fleet manager.

The following [use cases](./use-cases.md) describe the required lifecycle operations.

### Conformance

Conformance tests for native deployments cover the Margo interface: the device reports `native` in `supportedDeploymentTypes`, and the WFM client reports the deployment status of native components as defined above.

### Implementation

Before the SUP is ready for review, the SUP group will provide a proof of concept covering a fleet manager and a WFM client, and a branch of the [sandbox](https://github.com/margo/sandbox) that implements the WFM client side for a Debian-based device.

## Breaking Changes

None. Existing `helm`, `compose` and `custom` deployments are unchanged. Fleet managers and devices that do not know `native` ignore it, as the extensibility section of the specification already requires for unknown deployment types.

## Alternatives considered (optional)

**Use the existing `custom` deployment type.** Rejected. A `custom` profile only has meaning between suppliers who agree on it, so native deployments would not be interoperable across fleet managers and devices. `custom` also requires artifacts to be packaged as OCI blobs, but native workloads normally live in package repositories.

**A separate API and desired state document for native workloads.** Rejected. For the end user, a native workload is another application to deploy, and a second desired state adds no value for them.

## Rejection reason

Not applicable.
