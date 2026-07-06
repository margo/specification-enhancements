# Use Case — Ingress Hostname Reservation

> **Status:** Informative (Non-Normative)

## Problem

Two applications deployed to the same device both default to `app.example.com`.
Only one can own the route. The conflict is invisible until traffic breaks.

## Without the Capability Framework

The deployment succeeds. The conflict surfaces at runtime — through
platform-specific ingress controller errors, not through the Margo
management plane.

## With the Capability Framework - as a Guard Capability

The Device Agent validates the requested hostname against current allocations
before applying the deployment. If the hostname is taken, the deployment is
rejected before any component is installed.

This capability produces no output. It is a **guard** — pass or fail only.

---

## CapabilityDefinition

```yaml
apiVersion: margo.org/v1alpha1
kind: CapabilityDefinition

metadata:
  id: capability.margo.org/network/ingress

spec:
  scope: device

  description: |
    Validates ingress hostname requests against currently allocated hostnames
    on the device. Produces no output — conflict guard only.

  sourceState:
    schema:
      type: object
      properties:
        domain:
          type: string
        allocatedHostnames:
          type: array
          items:
            type: object
            properties:
              hostname:
                type: string
              allocatedTo:
                type: string
              tlsEnabled:
                type: boolean

  discovery:
    requestSchema:
      type: object
      required: [hostname]
      properties:
        hostname:
          type: string
        tlsRequired:
          type: boolean
          default: true

    outputSchema: {}              # no output — guard only

    failureCodes:
      - HostnameAlreadyAllocated
      - InvalidHostnameFormat
```

---

## How It Works

The operator supplies the hostname as a plain `value` — no `valueFrom`
needed since nothing is being generated. The `discoverCapabilities` entry
validates that hostname before the deployment is applied.

```yaml
# ApplicationDeployment (relevant sections only)
spec:
  discoverCapabilities:
    storefrontIngress:
      id: capability.margo.org/network/ingress
      request:
        hostname: shop.example.com
        tlsRequired: true

  parameters:
    publicHostname:
      value: shop.example.com           # operator-supplied — matches request.hostname
      targets:
        - pointer: env.PUBLIC_HOSTNAME
          components: ["storefront"]
```

The Device Agent evaluates `shop.example.com` against its local
`CapabilityState`. If free, the deployment proceeds. If taken:

```yaml
capabilityResolution:
  status: Failure
  failureCode: HostnameAlreadyAllocated
  message: >
    'shop.example.com' is already allocated to deployment-retail-frontend-007.
```

After a successful deployment, the Device Agent adds the hostname to its
local state and republishes `CapabilityState` to the WFM — so subsequent
deployments see the updated allocations.

---

## What This Demonstrates

| Concern | How it is addressed |
|---|---|
| Hostname conflict | Device Agent checks allocations before applying |
| Conflict caught before deployment | Rejects on `HostnameAlreadyAllocated` |
| No value injection | `outputSchema` is empty — guard, not generator |
| WFM pre-flight | Checks last published `CapabilityState` before dispatch |

---

## Key Architectural Point — Guard Capabilities

This is a **guard capability** — it validates a known value rather than
generating an unknown one. No `valueFrom`, no output injection.

Guard capabilities are appropriate when:
- The application already knows the value it wants
- The platform only needs to confirm it is safe to use

The same pattern applies to port conflict detection, route prefix
uniqueness, and device-local DNS name reservation.
` ` `

Half the length, same information. The gpu.md depth is justified because GPU has a complex before/after resolution story. Ingress doesn't — so it shouldn't pretend to.