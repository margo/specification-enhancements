# Specification Update Proposal: Specification extensions support in the Margo Specification

## Owner

[@phil-abb](https://github.com/phil-abb)

## Summary

This SUP proposes a standardized mechanism for enabling specification extensions in the Margo specification. It introduces a convention for extension fields using the format `x-<company>-extensions:` (with a recommended naming convention and regex), allowing any valid YAML or JSON content. The proposal targets the [Application Description YAML](https://docs.margo.org/specification/applications/application-description), [Application Deployment YAML](https://docs.margo.org/specification/margo-management-interface/desired-state#applicationdeployment-yaml-definition), and [Device Capabilities JSON](https://docs.margo.org/specification/margo-management-interface/device-capabilities) documents to support specification extensions for application,  Workload Fleet Manager (WFM) and device suppliers. This proposal provides schema and implementation guidance to ensure robust interoperability.

## Reason for proposal

As Margo adoption grows, suppliers require a way to add custom metadata, configuration, or integration hooks to add custom functionality to Margo artifacts that go beyond what is defined in the Margo specification. These extensions may be needed for:

- Collaboration between application suppliers and WFM suppliers (e.g., for WFM-specific deployment logic, cataloging, or analytics)
- Collaboration between application suppliers and device suppliers (e.g., for device-specific configuration, hardware integration, or custom runtime features)
- Collaboration between WFM suppliers and device suppliers (e.g., for WFM-specific deployment logic, custom configuration, extended WFM client functionality)

A standardized extension mechanism ensures that such customizations do not break interoperability or conformance for other suppliers, that unknown extensions are safely ignored, and that the Margo specification can evolve without breaking supplier-specific customizations.

## Requirements alignment acknowledgement

This proposal addresses [feature #144](https://github.com/margo/specification/issues/144), which calls for a flexible, forward-compatible extension mechanism in the Margo specification. It aligns with Margo’s vision of open, interoperable orchestration at the edge, enabling innovation while maintaining a stable core.

## Technical proposal

### Extension Types and Locations

There are four types of specification extensions:

1. **Application supplier ↔ WFM supplier specification Extensions**

    - Placed at the root of the Application Description YAML.
    - Used for information exchanged between the application supplier and the WFM supplier.
    - **MUST NOT** be propagated to the device in the Application Deployment YAML.

2. **Application supplier ↔ Device supplier specification Extensions**
  
    - Placed in the Application Description YAML **within** the `deploymentProfiles` array, either:
      - At the root of a `DeploymentProfile` object (for deployment-profile-wide extensions)
      - At the root of a `DeploymentProfile.components` object (for component-specific extensions)
    - These extensions **MUST** be propagated to the Application Deployment YAML, where they appear:
      - At the root of the `deploymentProfile` object (for deployment-profile-wide extensions)
      - At the root of a `deploymentProfile.components` object (for component-specific extensions)

3. **WFM supplier ↔ Device supplier specification Extensions**

    - Placed in the Application Deployment YAML at the root of the `spec` object
    - Used for information exchanges between the WMF supplier and the device supplier.
    - WFM suppliers may also add their own extensions to the `deploymentProfiles` or `deploymentProfiles.components` roots as well.
  
4. **Device supplier ↔ WFM supplier specification Extensions**

    - Placed at the root of the Device Capabilities JSON
    - Used for information exchanges between the device supplier and the WFM supplier.

### Extension Field Format

- Extension fields **MUST** use the format: `x-<company>-extensions:`
- `<company>` **MUST** match the regex: `/^[a-z][a-z0-9-]*$/`
  - Must start with a lowercase letter
  - May contain lowercase letters, digits, and dashes
  - No spaces or special characters
- Any valid YAML or JSON (depending on the document format) structure is allowed under the extension key

**Examples of valid extension keys:**

- `x-acme-extensions:`
- `x-northstar-industrial-extensions:`
- `x-foo123-extensions:`

### YAML Examples

#### 1. Application Description YAML (application ↔ WFM specification extension)

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDescription
metadata:
  id: com-acme-hello-world
  name: Hello World  
  version: "1.0"
x-acme-extensions:
  wfm-catalog-metadata:
  region: us-west
  compliance: true
```

#### 2. Application Description YAML (application ↔ device specification extension at DeploymentProfile root)

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDescription
metadata:
  id: com-northstarida-digitron-orchestrator
  name: Digitron orchestrator
  version: "1.2.1"
deploymentProfiles:
  - type: helm.v3
    id: com-northstarida-digitron-orchestrator-helm.v3-a
    x-northstar-industrial-extensions:
      device-features:
        enable-gpu: true
        custom-init: /opt/northstar/init.sh
    components:
    - name: digitron-orchestrator
      properties:
      repository: oci://northstarida.azurecr.io/charts/northstarida-digitron-orchestrator
      revision: 1.0.9
```

#### 3. Application Description YAML (application ↔ device specification extension at DeploymentProfile.Component root)

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDescription
metadata:
  id: com-northstarida-digitron-orchestrator
  name: Digitron orchestrator
  version: "1.2.1"
deploymentProfiles:
  - type: helm.v3
    id: com-northstarida-digitron-orchestrator-helm.v3-a
    components:
    - name: digitron-orchestrator
      properties:
        repository: oci://northstarida.azurecr.io/charts/northstarida-digitron-orchestrator
        revision: 1.0.9
      x-northstar-industrial-extensions:
        device-features:
          enable-gpu: true
          custom-init: /opt/northstar/init.sh
```

#### 4. Application Deployment YAML (application ↔ device specification extension at deploymentProfile root)

```yaml
apiVersion: application.margo.org/v1alpha1
kind: ApplicationDeployment
metadata:
  annotations:
    applicationId: com-northstarida-digitron-orchestrator
    id: a3e2f5dc-912e-494f-8395-52cf3769bc06
    name: com-northstarida-digitron-orchestrator-deployment
    namespace: margo-poc
spec:
  deploymentProfile:
    type: helm.v3
    components:
    - name: digitron-orchestrator
      properties:
      repository: oci://northstarida.azurecr.io/charts/northstarida-digitron-orchestrator
      revision: 1.0.9
    x-northstar-industrial-extensions:
      device-features:
        enable-gpu: true
        custom-init: /opt/northstar/init.sh
```

#### 5. Application Deployment YAML (application ↔ device specification extension at deploymentProfile.components root)

```yaml
spec:
  deploymentProfile:
    type: helm.v3
    components:
    - name: digitron-orchestrator
      properties:
        repository: oci://northstarida.azurecr.io/charts/northstarida-digitron-orchestrator
        revision: 1.0.9
      x-northstar-industrial-extensions:
        device-features:
          enable-gpu: true
          custom-init: /opt/northstar/init.sh
```

#### 6. Application Deployment YAML (WFM ↔ device specification extension at spec root)

```yaml
spec:
  deploymentProfile:
    type: helm.v3
    components:
    - name: digitron-orchestrator
      properties:
        repository: oci://northstarida.azurecr.io/charts/northstarida-digitron-orchestrator
        revision: 1.0.9
  x-cloudfm-extensions:
    cm-store-id: ABBC123JELZ
```

#### 7. Device Capabilities JSON (device ↔ WFM specification extension at root)

```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "DeviceCapabilitiesManifest",
  "properties": {...},
  "x-southstar-inc-extensions": {
    "mode": "hybrid",
    "security": {
      "storage": "vault",
      "rootOfTrust": "tpm" 
    }
  }
}
```

### Schema Update Guidance

To support unknown extension fields, the Margo schemas (YAML/JSON Schema) should be updated as follows:

- Use `patternProperties` (JSON Schema) or equivalent to allow keys matching `^x-[a-z][a-z0-9-]*-extensions$`
- Set `additionalProperties: true` at the relevant object levels
- Do not require the schema to know the structure of the extension value

**Example JSON Schema snippet:**

```json
{
  "type": "object",
  "properties": {
    // ... core spec properties ...
  },
  "patternProperties": {
    "^x-[a-z][a-z0-9-]*-extensions$": { "type": "object" }
  },
  "additionalProperties": false
}
```

### Implementation Guidance

- **WFM and device suppliers**:  
  - MUST ignore unknown `x-<company>-extensions:` fields.
  - MUST NOT fail validation or runtime processing due to unknown extensions.
  - SHOULD apply implement fields if they have an agreement with the extension author.

- **Application suppliers**:  
  - SHOULD only use extensions when collaborating with a specific WFM or device supplier.
  - MUST NOT rely on extensions being processed by suppliers who have not agreed to support them.

### Conformance

- Adding extensions using this mechanism MUST NOT break conformance for other suppliers.
- Unknown extensions MUST be ignored by default.

## Alternatives considered (optional)

- Strict schema with no extension fields (rejected: too inflexible)
- Out-of-band extension mechanism (rejected: less transparent, harder to validate, risk of conflicts)

## Rejection reason (optional)

N/A
