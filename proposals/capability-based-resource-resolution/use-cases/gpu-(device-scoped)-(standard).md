# Use Case — GPU Capability

> **Status:** Informative (Non-Normative)

## Problem

An AI inference workload requires a GPU with sufficient VRAM. Device paths,
models, and allocation state are only known at deployment time — the
application vendor cannot hardcode them.

## Without the Capability Framework

The application hardcodes a device path. Deployment succeeds. The workload
fails at runtime when the expected device does not exist or is already in
use by another deployment.

## With the Capability Framework

The Device Agent evaluates the GPU request against its current
`CapabilityState` before applying the deployment. A suitable GPU is
selected and its details injected into the application. If no suitable
GPU is free, the deployment is rejected before any component is installed.

---

## CapabilityDefinition

```yaml
apiVersion: margo.org/v1alpha1
kind: CapabilityDefinition

metadata:
  id: capability.margo.org/hardware/gpu

spec:
  scope: device

  description: |
    Represents a GPU available on a device. Enables applications to request
    GPU discovery with minimum VRAM, model, and architecture preferences.

  sourceState:
    schema:
      type: object
      properties:
        gpus:
          type: array
          items:
            type: object
            properties:
              devicePath:
                type: string
                description: Kernel device path e.g. /dev/nvidia0
              model:
                type: string
              architecture:
                type: string
                description: e.g. ampere, ada
              vramGiB:
                type: number
              allocatedTo:
                type: string
                description: Deployment ID currently holding this GPU

  discovery:
    requestSchema:
      type: object
      properties:
        minVramGiB:
          type: number
        model:
          type: string
          description: Optional — any model selected if omitted
        architecture:
          type: string
          description: Optional

    outputSchema:
      type: object
      properties:
        devicePath:
          type: string
        model:
          type: string
        architecture:
          type: string

    failureCodes:
      - NoGPUAvailable
      - InsufficientVRAM
      - GPUAlreadyAllocated
      - ModelNotFound
```

---

## How It Works

The application vendor declares GPU-related parameters in
`ApplicationDescription` with no defaults — they cannot know the values
at authoring time. The operator wires capability outputs into those
parameters using `valueFrom` in `ApplicationDeployment`.

```yaml
# ApplicationDeployment (relevant sections only)
spec:
  discoverCapabilities:
    inferenceGPU:
      id: capability.margo.org/hardware/gpu
      request:
        minVramGiB: 8
        architecture: ampere

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
```

The Device Agent evaluates the request against its local `CapabilityState`:

| GPU | Architecture | VRAM | Allocated | Passes? |
|---|---|---|---|---|
| `/dev/nvidia0` | ada | 24 GiB | ✅ yes | ❌ GPUAlreadyAllocated |
| `/dev/nvidia1` | ampere | 10 GiB | ❌ no | ✅ Passes all checks |

`/dev/nvidia1` is selected. The Device Agent resolves `valueFrom` references
and writes concrete values into `parameters` before applying:

```yaml
  parameters:
    gpuDevicePath:
      value: /dev/nvidia1           # resolved by Device Agent
      targets:
        - pointer: env.GPU_DEVICE_PATH
          components: ["ml-inference"]
    gpuModel:
      value: NVIDIA RTX 3080
      targets:
        - pointer: env.GPU_MODEL
          components: ["ml-inference"]
```

If no suitable GPU is free, the Device Agent rejects the deployment without
applying it and reports the failure via `DeploymentStatus` using one of the
`failureCodes` declared in the `CapabilityDefinition` — for example
`GPUAlreadyAllocated` or `InsufficientVRAM`.

---

## What This Demonstrates

| Concern | How it is addressed |
|---|---|
| Device path unknown at authoring time | `valueFrom` defers discovery to resolution time |
| GPU conflict between deployments | Device Agent checks `allocated` before assigning |
| Conflict caught before deployment | Rejects on `GPUAlreadyAllocated` |
| Resolved values flow into component | Existing `targets` mechanism — no component layer changes |
| Device state stays current | Device Agent republishes `CapabilityState` after allocation |

---

## Key Architectural Point — Generator Capabilities

This is a **generator capability** — the platform produces values the
application could not know at authoring time. This is distinct from guard
capabilities like ingress hostname reservation, where the application
already knows the value and only needs conflict validation.

Generator capabilities are appropriate when:
- The value is device-specific and unknown at authoring time
- The platform must select from multiple candidates based on requirements
- The selected value must be injected into the application configuration