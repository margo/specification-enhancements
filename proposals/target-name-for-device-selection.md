# Specification Update Proposal: Rename deviceId targeting fields to targetName

* [Specification Update Proposal: Rename deviceId targeting fields to targetName](#specification-update-proposal-rename-deviceid-targeting-fields-to-targetname)
  * [Owner](#owner)
  * [Summary](#summary)
  * [Reason for proposal](#reason-for-proposal)
  * [Requirements alignment acknowledgement](#requirements-alignment-acknowledgement)
  * [Technical proposal](#technical-proposal)
    * [1. Terminology](#1-terminology)
    * [2. Device capabilities](#2-device-capabilities)
    * [3. Deployment status](#3-deployment-status)
    * [4. Desired state](#4-desired-state)
  * [Alternatives considered](#alternatives-considered)
    * [Keep deviceId for routes and body fields](#keep-deviceid-for-routes-and-body-fields)
  * [Rejection reason](#rejection-reason)

## Owner

Armand Craig - @ajcraig

## Summary

This SUP proposes renaming Management Interface fields and route parameters that currently use `deviceId` for deployment targeting to `targetName`.

The current name implies a globally unique, system-assigned device identifier. `targetName` better describes that role without consuming the term `deviceId`, which should remain available for universally unique identifiers assigned by a Device Fleet Manager or another authoritative inventory system.

The proposal updates the Device Capabilities, Desired State, and Deployment Status documentation so the same target naming model is used consistently across reporting, assignment, and status.

## Reason for proposal

The current Management Interface uses `deviceId` in places where the value is not necessarily a unique device identifier. For example, a see-thru gateway reports hierarchical values such as `gateway1/deviceA` or `gateway1/zone1/sensorB`.

Those values are better understood as names in a WFM Client-reported target namespace. They are stable and meaningful for placement, status, and gateway routing, but they are not guaranteed to be globally unique device identifiers. Calling them `deviceId` risks creating ambiguity later if Margo adds or integrates a true device identity assigned by a Device Fleet Manager.

This SUP defines the terminology explicitly:

* `targetName` identifies the deployment target as known to the WFM Client and WFM.
* `targetName` may be a single segment, a hierarchical gateway path, or a gateway wildcard where the specification allows wildcard placement.
* `deviceId` is reserved for a future or external universally unique device identifier.

## Requirements alignment acknowledgement

This SUP addresses a terminology and interoperability gap in the Margo Management Interface. It does not change the core WFM Client identity model or introduce a new device identity system.

Out of scope:

* Defining a globally unique `deviceId` format or assignment authority.
* Defining Device Fleet Manager behavior.
* Changing WFM Client authentication or authorization.
* Changing the semantics of opaque gateways.

## Technical proposal

### 1. Terminology

Add a common definition for `targetName` to the Management Interface documentation:

`targetName` is the WFM Client-reported name of a deployment target. It identifies the device, gateway, or child device path that a WFM uses for capability reporting, desired state assignment, and deployment status correlation.

The value:

* MUST be stable for the lifetime of the target relationship.
* MUST consist only of RFC 3986 unreserved characters in each path segment.
* MAY contain `/` separators to represent a see-thru gateway hierarchy.
* MAY use `*` as the final path segment only where the specification explicitly allows gateway-selected placement.
* MUST be treated as opaque by the WFM except where the specification defines gateway path interpretation.

The term `deviceId` should be removed from the Management Interface targeting surface unless the field truly refers to a globally unique device identifier.

### 2. Device capabilities

Update the Device Capabilities endpoint and payload documentation to use `targetName` instead of `deviceId`.

Proposed routes:

```https
PUT /api/v1/capabilities/{targetName}
DELETE /api/v1/capabilities/{targetName}
```

Proposed route parameter:

| Parameter | Type | Required? | Description |
|-----------|------|-----------|-------------|
| `{targetName}` | string | Y | The name of the target whose capabilities are being reported or deleted. It uses the format `{name}[/{name}[/{name}...]]`. The top-level segment identifies the reporting gateway or device. Additional segments identify child devices behind a see-thru gateway. Each segment MUST include only unreserved characters from RFC 3986. |

Proposed properties field:

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| targetName | string | Y | The name of the target whose capabilities are described. It MUST match the `{targetName}` route parameter. For a child device behind a see-thru gateway, the value is a path containing the gateway name, any intermediate target names, and the child target name, for example `gateway1/zone1/sensorB`. |

Call examples for see-thru gateway section:

```https
PUT /api/v1/capabilities/gateway1
```

```json
{
    "properties": {
        "targetName": "gateway1",
        "vendor": "Gateway Vendor",
        "modelNumber": "GW-1000",
        "serialNumber": "GW12345678"
    }
}
```

```https
PUT /api/v1/capabilities/gateway1/deviceA
```

```json
{
    "properties": {
        "targetName": "gateway1/deviceA",
        "vendor": "Sensor Vendor",
        "modelNumber": "S-200",
        "serialNumber": "S200-A",
        "cpus": [
            {
                "cores": 2,
                "architecture": "arm64"
            }
        ],
        "memory": "2 Gi",
        "storage": "16 Gi",
        "peripherals": [],
        "interfaces": [
            {
                "type": "ethernet"
            }
        ],
        "otelCollector": true,
        "supportedRuntimes": [
            "oci"
        ],
        "supportedDeploymentTypes": [
            "compose"
        ]
    }
}
```

### 3. Deployment status

Rename the Deployment Status body field from `deviceId` to `targetName` and make it required.

The report MUST always name the workload target. If a see-thru gateway reports status for a child device, `targetName` identifies the child target using the full gateway path.

Proposed request body field:

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| targetName | string | Y | Name of the target hosting the workload. Includes the full gateway hierarchy if applicable. |

Update the example payloads so required fields and examples are consistent. A hosting WFM Client reporting status for its own workload target includes `targetName`:

```json
{
    "deploymentId": "a3e2f5dc-912e-494f-8395-52cf3769bc06",
  "targetName": "gateway1",
    "status": {
        "state": "pending"
    },
    "components": [
        {
            "name": "digitron-orchestrator",
            "state": "pending"
        },
        {
            "name": "database-services",
            "state": "pending"
        }
    ]
}
```

A see-thru gateway reporting status for a child target includes `targetName`:

```json
{
    "deploymentId": "a3e2f5dc-912e-494f-8395-52cf3769bc06",
    "targetName": "gateway1/deviceA",
    "status": {
        "state": "installed"
    },
    "components": [
        {
            "name": "digitron-orchestrator",
            "state": "installed"
        }
    ]
}
```

The existing `Error.source` field in Deployment Status should use the new terminology. For reserved gateway-generated errors, `source` identifies the gateway target that generated the error, including hierarchy if applicable.

### 4. Desired state

Rename `metadata.deviceId` on `ApplicationDeployment` to `metadata.targetName`.

Proposed metadata field:

| Attribute | Type | Required? | Description |
|-----------|------|-----------|-------------|
| targetName | string | Y | The target name to which the deployment is assigned. To reference a child device behind a see-thru gateway, use `{target-name}[/{target-name}[/...]]`. To request the gateway to choose the child device, use `*` as the last segment, for example `gateway1/*` or `gateway1/zone1/*`. If the gateway cannot autonomously select a child device, it MUST report deployment status with error `103 - Autonomous placement not supported`. |

Add desired state examples for see-thru gateway deployments.

Gateway target:

```yaml
apiVersion: deployment.margo.org/v1alpha1
kind: ApplicationDeployment
id: a3e2f5dc-912e-494f-8395-52cf3769bc06
metadata:
  name: digitron-gateway
  namespace: digitron
  targetName: gateway1
spec:
  applicationId: org.margo-example.digitron
  deploymentProfile:
    type: compose
    components:
      - name: digitron-agent
        properties:
          repository: ghcr.io/margo/digitron-agent
          revision: sha256:1111111111111111111111111111111111111111111111111111111111111111
  parameters: {}
```

Child target behind a see-thru gateway:

```yaml
apiVersion: deployment.margo.org/v1alpha1
kind: ApplicationDeployment
id: b4f3a6ed-102f-405f-9a86-63df487abc17
metadata:
  name: digitron-sensor
  namespace: digitron
  targetName: gateway1/deviceA
spec:
  applicationId: org.margo-example.digitron
  deploymentProfile:
    type: compose
    components:
      - name: digitron-sensor
        properties:
          repository: ghcr.io/margo/digitron-sensor
          revision: sha256:2222222222222222222222222222222222222222222222222222222222222222
  parameters: {}
```

Gateway-selected wildcard target:

```yaml
apiVersion: deployment.margo.org/v1alpha1
kind: ApplicationDeployment
id: c5a4b7fe-2130-4160-ab97-74e0598bcd28
metadata:
  name: digitron-any-sensor
  namespace: digitron
  targetName: gateway1/*
spec:
  applicationId: org.margo-example.digitron
  deploymentProfile:
    type: compose
    components:
      - name: digitron-sensor
        properties:
          repository: ghcr.io/margo/digitron-sensor
          revision: sha256:3333333333333333333333333333333333333333333333333333333333333333
  parameters: {}
```

## Alternatives considered

### Keep deviceId for routes and body fields

Keeping `deviceId` avoids a rename, but it preserves the ambiguity that prompted this SUP. The value is often a user-assigned target path rather than a globally unique device identifier, and the mismatch becomes more visible in gateway scenarios.

## Rejection reason

Not applicable.