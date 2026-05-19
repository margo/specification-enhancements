# Specification Update Proposal

## Owner

> List the name(s) of the person driving the SUP to completion.

Arne Broering, Siemens AG

> Complete as part of Phase 2: Proposal Creation

## Summary

> Provide a summary (in layman's terms) explaining the changes the SUP is proposing
> 
> Complete as part of Phase 2: Proposal Creation

Today, Margo already lets an app say things like:

*I need a Kubernetes/Helm device* or *I need a Compose device*, and
*I need a certain amount of CPU, memory, storage, or a GPU/network interface.* 

However, it does not yet let an app say:

*I only work on devices from vendor X*, or
*I need vendor-specific runtime Y already installed on the device*.

The proposed change would add exactly this missing capability.

Therefore, the ``ApplicationDescription`` file is extended so the app developer can state that their app needs a specific vendor runtime, or a specific runtime capability in addition to the existing generic Margo roles and resource requirements. 

To support this, devices need to report those special runtimes and their capabilities. Today, devices already report their vendor, model, role, CPU, memory, storage, peripherals, and interfaces to the WFM (WFM) during onboarding and when capabilities change. This new feature would extend the ``DeviceCapabilityManifest`` so a device could also say which vendor-specific runtimes it has, and which special runtime capabilities those runtimes provide. 

In consequence, the WFM is then required to check whether the target device really has the requested vendor runtime and rquired runtime capabilities and, if not, refuse deployment. 

## Reason for proposal

> Explain why this SUP is needed and how it improves on what we have in the specification
> 
> Complete as part of Phase 2: Proposal Creation

This SUP addresses: https://github.com/margo/specification/issues/93

This makes Margo more practical for real-world industrial ecosystems where some apps are not fully generic, but depend on a vendor-specific software environment or runtime functionality.
Without this feature, an app may look deployable because the device has enough CPU/memory and the right general role, even though it is still missing the vendor runtime the app actually needs. 
With this new feature, the app developer can describe these requirements in a standard, machine-readable way, and the WFM can enforce them automatically.


## Requirements alignment acknowledgment

> An acknowledgment that the SUP meets minimum requirements and doesn't introduce any requirements that are out of Margo's scope or vision. This section must have link(s) to applicable features and a statement about any requirements that were agreed to be out of scope for the SUP.
> 
> Complete as part of Phase 2: Proposal Creation

This SUP addresses the identified and agreed upon feature gap to enable application developers to target device specific runtime environments: https://github.com/margo/specification/issues/93


## Technical proposal

> The SUP's technical details. There must be enough technical details that someone can take the information in this section and implement it on their own.
> 
> Complete as part of Phase 3: SUP Technical Development

In the current Margo spec, the `deploymentProfiles` section is already the place where the application developer expresses how the application is deployed (helm.v3 or compose) and what that deployment requires in terms of CPU, memory, storage, peripherals, and interfaces. That makes it the most natural place to add requirements for a specific vendor runtime environment, because those requirements are deployment-profile specific in exactly the same way as existing platform and resource constraints. 

This is also consistent with Margo’s current architecture: the device reports its characteristics to the WFM through the ``DeviceCapabilitiesManifest`` during onboarding and updates, and the WFM is the component that decides which workloads can be assigned to which devices. Therefore, the correct pattern is: ``ApplicationDescription`` declares requirement; ``DeviceCapabilitiesManifest`` advertise support; WFM enforces match.

### Proposed new field in ApplicationDescription

Adding a new optional field under each ``deploymentProfiles[]`` entry:

```yaml
runtimeRequirements:
  targets:                     # required, non-empty
    - runtimeRef: string       # optional 
      version: string          # optional 
      runtimeCapabilities:     # optional
        - string
      deviceSelector:          # optional
        vendorId: string       # optional
        modelNumberIds:              # optional
          - string
```

* ``runtimeRequirements`` - This field is an optional element specifying one or more device-specific runtime environments (via multiple target definitions) and/or device runtime capability sets required by the deployment profile.
If runtimeRequirements is present, the WFM MUST NOT make this deployment profile eligible for deployment to a device unless the device’s `DeviceCapabilitiesManifest` contains a matching `runtimeEnvironment` and all required `runtimeCapabilities`. If no deployment profile of an application is eligible for a device, the WFM MUST block deployment of that application to that device and SHOULD expose the reason to the user.
* ``targets[]`` - Each target defines one acceptable runtime environment.
* ``runtimeRef`` - Stable identifier from the shared Margo/vendor runtime dictionary used to require a specific runtime.
* ``version`` - Version constraint for the runtime referenced by runtimeRef.
* ``runtimeCapabilities[]`` - Fine-grained functions that must be present within that runtime environment. These should reference capability IDs from the shared dictionary / device capability description feature.
* ``deviceSelector.vendorId`` - Stable vendor identifier from a shared dictionary used to require devices from a specific vendor. Reverse-domain / FQDN-style namespace as global unique identifier for vendor (e.g., com.northstar-ida).
* ``deviceSelector.modelNumberIds[]`` - Stable model number identifier from a shared dictionary used to require a device model.


#### Example ApplicationDescription:

```yaml

apiVersion: apps.margo.org/v1alpha1
kind: ApplicationDescription

metadata:
  id: com.northstar-ida.industry.app.camera-ai
  name: Northstar-IDA's app to do AI-based visual processing.
  description: >
    Example AI vision application intended for device runtimes that have also the 'Camera-Connector' app.
  version: 2.0.0
  catalog:
    application:
      descriptionFile: https://northstar-ida.com/apps/camera-ai/README.md
      icon: https://northstar-ida.com/apps/camera-ai/icon.png
      site: https://northstar-ida.com/apps/camera-ai
      tagline: GPU-accelerated visual inspection for Northstar IDA
      tags:
        - ai
        - vision
    organization:
      - name: Northstar-IDA Inc.
        site: https://northstar-ida.com

deploymentProfiles:
  - id: industry.edge-os
    type: compose
    requiredResources:
      cpu:
        cores: 2
      memory: 2Gi
    
    # --- Proposed extension -
    runtimeRequirements:
      targets:
        - runtimeRef: com.northstar-ida.industry.edge-os
          version: ">=3.1.0 <4.0.0"
          runtimeCapabilities:
            - com.northstar-ida.industry.app.camera-connector # app to enable connectivity to a connected camera
          deviceSelector:
            vendorId: com.northstar-ida
            modelNumberIds:
              - com.northstar-ida.edge-devices.EdgeServer-A
              - com.northstar-ida.edge-devices.EdgeServer-B
              - com.northstar-ida.edge-devices.EdgeServer-C
              - com.northstar-ida.edge-devices.EdgeServer-D 

  components:
    - name: camera-ai
      properties:
        packageLocation: https://registry.northstar-ida.com/camera-ai/compose-bundle-2.0.0.tar.gz
        wait: true
        timeout: "10m0s"

parameters:
  cameraStreamUrl:
    name: cameraStreamUrl
    value: "rtsp://camera.local:554/stream1"
    targets:
      - pointer: CAMERA_STREAM_URL
        components:
          - vision-inference

  confidenceThreshold:
    name: confidenceThreshold
    value: 0.8
    targets:
      - pointer: CONFIDENCE_THRESHOLD
        components:
          - vision-inference

configuration:
  sections:
    - name: Video input
      settings:
        - parameter: cameraStreamUrl
          name: Camera stream URL
          description: RTSP endpoint of the inspection camera
          immutable: false
          schema: cameraStreamUrlSchema
    - name: Inference
      settings:
        - parameter: confidenceThreshold
          name: Confidence threshold
          description: Minimum confidence for reporting detections
          immutable: false
          schema: confidenceThresholdSchema

  schema:
    - name: cameraStreamUrlSchema
      dataType: string
      allowEmpty: false
      minLength: 10
    - name: confidenceThresholdSchema
      dataType: double
      allowEmpty: false
      minValue: 0.1
      maxValue: 1.0
      minPrecision: 1
      maxPrecision: 3

```

### Proposed new field in DeviceCapabilitiesManifest

To make the application-side requirement enforceable, the device capability model needs a matching advertisement structure. Right now, ``DeviceCapabilitiesManifest`` already include the device vendor/model/serial, roles, resources, peripherals, and interfaces, but they do not yet include vendor runtime environments or runtime capability identifiers.

Adding a new optional field `runtimeEnvironments`:

```json
{
  "runtimeEnvironments": [
    {
      "runtimeRef": "string",        // required
      "vendor": "string",            // optional
      "vendorId": "string",          // optional
      "modelNumberId": "string",     // optional
      "version": "string",           // optional
      "runtimeCapabilities": [       // optional
        "string"
      ],
      "properties": {                // optional
        "<string>": "<string | number | boolean | array>" // optional
      }
    }
  ]
}

```


* ``runtimeEnvironments`` - optional element specifying the vendor-specific runtime environments available on the device for Margo-managed workloads. 
* ``runtimeRef`` - Each runtime environment MUST include a stable identifier in this field.
* ``vendor`` - The ``vendor`` field MAY be used to provide a human-readable vendor name. 
* ``vendorId`` - The ``vendorId`` field SHOULD be used to provide a stable machine-readable vendor namespace identifier. Vendor-defined runtime and capability identifiers SHOULD be scoped underneath the corresponding ``vendorId``.
* `modelNumberId` - This field defines a stable model number identifier from a shared dictionary.
* ``version`` - The ``version`` field MAY be used to report the installed version of the runtime environment. If an application declares a ``version`` constraint for a referenced runtime, the WFM MUST evaluate that constraint against the runtime environment version reported by the device.
* ``runtimeCapabilities`` - This field MAY be used to report runtime-specific capability identifiers available on the device. If an application declares required ``runtimeCapabilities``, the WFM MUST compare those capability identifiers with the runtimeCapabilities reported for the runtime environment.
* ``properties`` - An optional extensibility map for runtime-specific metadata that may be useful for operators, diagnostics, or policy logic, but that is NOT part of the core matching model. This allows vendors to advertise additional structured information without overloading the core schema. 

If a ``runtimeEnvironment``, their ``version``, or their ``runtimeCapabilities`` change, the device client MUST update the ``DeviceCapabilitiesManifest`` reported to the WFM.

#### Example DeviceCapabilitiesManifest:

```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "DeviceCapabilitiesManifest",

  "properties": {
    "id": "ie-device-bx59a-001",
    "vendor": "Northstar-IDA Inc.",
    "modelNumber": "EdgeServer-A",
    "serialNumber": "ABC-EXAMPLE-001",
    "roles": [
      "standalone device",
    ],

    "resources": {
      "cpu": [
        {
          "cores": 24,
          "architecture": "x86_64",
        },
      ],
      "memory": "30Gi",
      "storage": "900Gi",
      "peripherals": [
        {
          "type": "gpu",
          "manufacturer": "NVIDIA",
          "model": "L4",
        },
      ],
      "interfaces": [
        {
          "type": "ethernet",
        },
      ],
    },

    // --- Proposed extension ---
    "runtimeEnvironments": [
      {
        "id": "com.northstar-ida.industry.edge-os",
        "vendor": "Northstar-IDA Inc.",
        "vendorId": "com.northstar-ida",
        "modelNumberId": "com.northstar-ida.edge-devices.EdgeServer-A",
        "version": "3.1.1-3-x86-64",
        "capabilities": [
          "com.northstar-ida.industry.app.camera-connector", // app to enable connectivity to a connected camera
          "com.northstar-ida.industry.app.opcua-connetor",   // app to access OPC UA servers
        ],
        "properties": {
          "iedkVersion": "1.24.2-1",
          "dockerVersion": "20.10.24",
          "operatingSystem": "Debian 12 (Bookworm)",
          "nvidiaDriverVersion": "525.85.12-1",
          "cudaDriverVersion": "12.0",
        },
      },
    ],
  },
}
```

## Alternatives considered (optional)

> List any alternative solutions considered while working on the SUP and the reason for not choosing them. If the SUP owner knows that there are alternative SUPs being worked on, this section can be used to highlight potential advantages this SUP has over the alternatives.
> 
> Complete as part of Phase 3: SUP Technical Development


### Referenced External Documents to Describe Requirements & Capabilities 

Instead of listing ``runtimeRequirments`` in ``ApplicationDescription`` and the ``runtimeEnvironment`` in ``DeviceCapabilitiesManifest``, another approach could be to link to external documents for such device runtime descriptions. 

External documents are attractive when the device/runtime description becomes too rich, too vendor-specific, or too reusable to fit comfortably in a small core schema. This is especially true if  just a runtime ID and version, but also interfaces, protocols, endpoints, semantic metadata, conformance profiles, or model-level descriptions shared across many devices or applications should be described. Standards like [W3C WoT Thing Description](http://w3c.org/wot) explicitly support being hosted externally. A similar pattern exists in the [Asset Administration Shell](http://) (AAS) world: the AAS is the digital representation of an asset, it is built from submodels, and those submodels can describe an asset’s features, characteristics, properties, statuses, parameters, measurement data, and capabilities. The AAS ecosystem also supports multiple interaction patterns including file exchange, not only live API access, which makes it conceptually compatible with external descriptor documents.

The main drawback of such an approach is operational: current Margo matching is local and direct. The ApplicationDescription already contains deployment profiles and required resources, while the DeviceCapabilitiesManifest contains the device’s identity, roles, and resources used for matching. If decisive information is moved entirely into external documents, the WFM cannot make a deployment decision until it has fetched and parsed them. That adds latency, caching concerns, signature/integrity handling, failure modes, and more complex onboarding behavior.
There is also a specification-alignment issue: Margo’s application package flow is already OCI-registry-based, and the current device onboarding/capabilities flow expects a concrete capability document at the WFM endpoint. So an external-document design is very feasible, but it would require a deliberate spec change to define how references are transported, validated, cached, and normalized.


## Rejection reason

> If a SUP is rejected, indicate the reason why it was rejected.
> 
> Complete if SUP is rejected at Phase 2: Proposal Creation or Phase 4: Final Decision 
