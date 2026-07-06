# Schema Changes

This section defines the changes required to existing Margo schemas to support
the Capability Definition Framework. These changes are additive — existing fields
are unchanged unless explicitly stated.

---
## ApplicationDescription Changes

No structural changes are required to `ApplicationDescription`. It remains
platform-agnostic — it declares what parameters the application accepts and
where they flow inside the component, but takes no position on how those
parameters are sourced on any specific platform.

The `parameters` block is the interface contract between the application vendor
and the deployment. The `value` field carries the default — the operator or
platform may override it at deployment time. The `targets` field defines where
the value flows inside the component.

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDescription
id: com.example.ml-inference-app
metadata:
  name: ML Inference Application
  version: 1.0.0

parameters:
  gpuDevicePath:
    value: "/dev/gpu0"              # default set by application vendor
    targets:
      - pointer: env.GPU_DEVICE_PATH
        components: ["ml-inference"]
  gpuModel:
    value: "generic"
    targets:
      - pointer: env.GPU_MODEL
        components: ["ml-inference"]
  gpuVram:
    value: 8
    targets:
      - pointer: env.GPU_VRAM_GIB
        components: ["ml-inference"]

deploymentProfiles:
  - type: helm.v3
    components:
      - name: ml-inference
        properties:
          repository: oci://registry.example.com/charts/ml-inference
          revision: 2.1.0
```

The application vendor declares defaults. They do not know — and should not
care — whether those values will be overridden by an operator manually or by
the platform through capability discovery.

---

## ApplicationDeployment Changes

A new `discoverCapabilities` map is added to `spec`. It is the operator's
materialization of capability requirements for a specific deployment.

Parameters that are sourced from capability outputs use `valueFrom` instead
of `value`. `valueFrom` is a reference path into the `discoverCapabilities`
output — it is resolved at apply time by the resolving actor:

- **Fleet-scoped** capabilities: resolved by the WFM before dispatch.
  Resolved `value` is written into `parameters` before the deployment
  reaches the device.
- **Device-scoped** capabilities: resolved by the Device Agent at apply
  time against its own current `CapabilityState`. The `discoverCapabilities`
  block is preserved in the deployment sent to the device.

### Before Resolution

The operator authors this deployment. `valueFrom` references are unresolved:

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDeployment
id: deployment-ml-inference-01
metadata:
  deviceId: device-edge-001
  applicationId: com.example.ml-inference-app

spec:
  discoverCapabilities:
    inferenceGPU:                                   # unique key within this deployment
      id: capability.margo.org/hardware/gpu         # references a CapabilityDefinition URI
      request:                                      # conforms to discovery.requestSchema
        minVramGiB: 8
        architecture: ampere
        exclusive: true

  parameters:
    gpuDevicePath:
      valueFrom: discoverCapabilities.inferenceGPU.output.devicePath
      targets:
        - pointer: env.GPU_DEVICE_PATH
          components: ["ml-inference"]
    gpuModel:
      valueFrom: discoverCapabilities.inferenceGPU.output.model
      targets:
        - pointer: env.GPU_MODEL
          components: ["ml-inference"]
    gpuVram:
      valueFrom: discoverCapabilities.inferenceGPU.output.vramGiB
      targets:
        - pointer: env.GPU_VRAM_GIB
          components: ["ml-inference"]

  deploymentProfile:
    type: helm.v3
    components:
      - name: ml-inference
        properties:
          repository: oci://registry.example.com/charts/ml-inference
          revision: 2.1.0
```

### After Resolution

The Device Agent resolves `discoverCapabilities.inferenceGPU` against its
local `CapabilityState`, writes resolved values into `parameters`, and applies
the deployment. The `valueFrom` references are replaced with concrete `value`
entries:

```yaml
  parameters:
    gpuDevicePath:
      value: /dev/nvidia1           # resolved by Device Agent
      targets:
        - pointer: env.GPU_DEVICE_PATH
          components: ["ml-inference"]
    gpuModel:
      value: NVIDIA RTX 3080        # resolved by Device Agent
      targets:
        - pointer: env.GPU_MODEL
          components: ["ml-inference"]
    gpuVram:
      value: 10                     # resolved by Device Agent
      targets:
        - pointer: env.GPU_VRAM_GIB
          components: ["ml-inference"]
```

The existing `targets` mechanism then carries these resolved values into the
component — no changes to the component layer are required.

**Rules:**
- Each key in `discoverCapabilities` MUST be unique within a single
  `ApplicationDeployment`
- Each `id` in `discoverCapabilities` MUST reference a URI that exists in
  the Capability Registry
- `request` fields MUST conform to the referenced
  `CapabilityDefinition.discovery.requestSchema`
- `valueFrom` MUST reference a valid key in `discoverCapabilities` and a
  valid field in the referenced `CapabilityDefinition.discovery.outputSchema`
- A parameter MUST declare either `value` or `valueFrom` — not both
- Parameters using `valueFrom` MUST NOT be supplied manually by the operator
- For **fleet-scoped** capabilities: the WFM MUST resolve all `valueFrom`
  references and write concrete `value` entries into `parameters` before
  the deployment is included in the Desired State Manifest. The
  `discoverCapabilities` block MAY be stripped after resolution since the
  device does not need it.
- For **device-scoped** capabilities: the `discoverCapabilities` block MUST
  be preserved in the deployment sent to the device. The Device Agent
  resolves at apply time.
- The Device Agent MUST NOT apply a deployment if any device-scoped
  capability resolution fails. It MUST report a `CapabilityResolution`
  with the failure code back to the WFM via the status endpoint.
- The WFM MAY perform a pre-flight check using the last published
  `CapabilityState` before sending the deployment — but this is advisory
  and does not substitute for the Device Agent's authoritative resolution.