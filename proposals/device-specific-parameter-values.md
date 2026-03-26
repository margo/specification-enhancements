# Specification Update Proposal

## Owner

> List the name(s) of the person driving the SUP to completion.

Arne Broering, Siemens AG

> Complete as part of Phase 2: Proposal Creation

## Summary

> Provide a summary (in layman's terms) explaining the changes the SUP is proposing
> 
> Complete as part of Phase 2: Proposal Creation

Today, the Margo **Application Description** explicitly models parameters as values that the end user provides when installing/updating the application. A mechanism for **device-supplied parameter values** is currently missing. Without such a mechanism, the user has to discover these values manually and pass them as parameters, even though the device may already know them. 

The proposal is to add a new optional `valueFrom` field to `Parameter` in the  **Application Description** and instantiate the parameter value from what is defined under the new element `installContext` in the `DeviceCapabilitiesManifest`. Today `Parameter` has `name`, `value` (default value), and `targets`. The proposal is to add a `valueFrom` field that indicates the value is resolved from another source. 
The `valueFrom` mechanism is generic and can be in future applied to other sources, not only devices.
If both, `value` and `valueFrom`, are defined, `value` acts as the fallback and will be used if resolution fails.
A new element called `installContext` is added to the `DeviceCapabilitiesManifest`  and lists parameters supplied by the device.


## Reason for proposal

> Explain why this SUP is needed and how it improves on what we have in the specification
> 
> Complete as part of Phase 2: Proposal Creation

This SUP is needed because the current Margo specification already recognizes application parameters as a first-class concept, but it models them almost entirely as values that are provided by the user at install/update time. Currently, configurable parameters are defined so the end user can specify values. 

Today, a parameter can target Helm values using dot notation or Compose environment variables, which is good because it lets an application developer say where the value must land during deployment. But the spec does not define a standard mechanism for saying that a value should come from the device itself rather than from manual user input. That means the spec handles parameter injection **but does not handle parameter resolution from device context**.

The device capabilities model already requires the device to report characteristics such as role, CPU, memory, storage, peripherals, and interfaces to the Workload Fleet Manager (WFM), and it says the device client must update this information if it changes. However, those reported capabilities are primarily for describing the device and matching workloads, not for supplying install-time values like a hostname or the default storage class name. 

Examples of such device-supplied parameter values are related to its communication or computation specifics:
* The application needs to know which **hostname** or **IP address** the device uses. Then the application is able to use it as external endpoint (ingress) or within a TLS certificate. If Margo had a device-supplied parameter for the device's hostname (or IP), the device could resolve it once and use it for deployment. That would spare the user from having to discover and enter the value manually.
* The user deploying a Margo Helm-based application may not know which Kubernetes **storage class** (e.g., standard, fast-ssd) exists on a device, or which one is marked as the default. That is why the device can often discover the default storage class more reliably than the user.
* The user deploying a Margo Compose-based application may not know default **host path for app data**, or the default **named volume**, and those values could be device-supplied.



## Requirements alignment acknowledgment

> An acknowledgment that the SUP meets minimum requirements and doesn't introduce any requirements that are out of Margo's scope or vision. This section must have link(s) to applicable features and a statement about any requirements that were agreed to be out of scope for the SUP.
> 
> Complete as part of Phase 2: Proposal Creation

This SUP addresses the identified and agreed upon feature gap 'Define how device-specific parameter values can be provided': https://github.com/margo/specification/issues/141



## Technical proposal

> The SUP's technical details. There must be enough technical details that someone can take the information in this section and implement it on their own.
> 
> Complete as part of Phase 3: SUP Technical Development

### Adding a new optional `valueFrom` field to Parameter

Today Parameter has the attributes [link](https://docs.margo.org/specification/applications/application-description#defining-configurable-application-parameters):

* name
* value (default value)
* targets (to indicate which component the value should be applied to)

This proposal adds:

``` yaml
valueFrom:
  device:
    key: margo.cluster.hostname
    required: true
    fallback: null
```

**Proposed semantics**

Extension of `parameters`:

* `valueFrom` (optional): Indicates that the parameter value is to be resolved from a non-user source before installation or update.
* For GA1, the only standard source is `device`.
* `device.key` (required): Identifier of the device-supplied parameter to resolve.
* `device.required` (optional): If true, installation MUST fail if the key cannot be resolved. Default is true.
* The resolved `value` MUST still be validated against the parameter’s configured schema.
* If both, `value` and `valueFrom`, are defined, `value` acts as the fallback and will be used if resolution of `valueFrom` fails.

That keeps the existing parameter/configuration model intact and simply adds a second source of values.

Parameters that are expected to be supplied by the device are added to the `parameters` element of the **ApplicationDescription** with the `valueFrom` indicator specified as `device` as shown below:

```yaml
kind: ApplicationDescription
...
parameters:
  ingressHost:
    name: ingressHost
    # PROPOSED EXTENSION: tells Margo that the device supplies this value
    valueFrom:
      device:
        key: margo.cluster.hostname
        required: true
    targets:
      # Helm values.yaml target(s)
      - pointer: ingress.host
        components:
          - fi-web-helm
      - pointer: certificate.dnsNames[0]
        components:
          - fi-web-helm
      # Compose env var target(s)
      - pointer: APP_HOSTNAME
        components:
          - fi-web-compose
```


### Adding device supplied parameters to DeviceCapabilities

The `DeviceCapabilitiesManifest` is extended by adding an element called `installContext`. This element lists the device supplied parameters.

Before the device installs or updates an application, the device’s Workload Fleet Management Client resolves all parameters that use `valueFrom.device`. This is done prior to invoking the deployment provider (e.g., Helm or Compose). The device MUST resolve all `valueFrom.device` references to concrete values.

The resolved value MUST satisfy the schema referenced by the configuration setting exactly as if the value had been provided by the user. This keeps the current validation model unchanged. 

If a referenced device key cannot be resolved: if a `fallback` is defined, use it; otherwise installation/update MUST fail with a clear validation/resolution error.


```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "DeviceCapabilitiesManifest",
  ...
  "installContext": {
    "margo.cluster.hostname": {
      "type": "string",
      "value": "edge01.plant-a.example.com",
      "description": "Hostname/FQDN that applications should use for ingress, public URL generation, and certificate DNS names.",
      "mutable": true
    },
    "margo.cluster.defaultIngressClass": {
      "type": "string",
      "value": "nginx",
      "description": "Default ingress class to use for applications that expose HTTP/HTTPS endpoints through Kubernetes ingress.",
      "mutable": true
    },
  ...
```


### Margo-reserved Keys

* `margo.cluster.hostname` - For ingress hosts, cert DNS names, externally reachable application URLs.

* `margo.cluster.defaultStorageClass` - For PersistentVolume/PersistentVolumeClaim defaults on Kubernetes devices.



### Example 

In the `ApplicationDescription` and `DeviceCapabilitiesManifest` examples below, the WFM has through the `ApplicationDescription` the parameters that use `valueFrom.device.key`. The current specification already defines the overall deployment model in which applications are described via `ApplicationDescription` and then deployed with chosen parameters.
The device has already reported its capabilities, now including the proposed `installContext` dictionary. The current spec already has the device capability reporting flow.

Before installation, the device-side WFMC resolves:

* `ingressHost` from `margo.cluster.hostname`
* `storageClass` from `margo.cluster.defaultStorageClass`

The resolved values are then injected into:

* Helm values (ingress.host, certificate.dnsNames[0], persistence.storageClass)
or 
* Compose environment variables (APP_HOSTNAME, DEVICE_ID)



#### Application Description Example

```yaml
apiVersion: app.margo.org/v1alpha1
kind: ApplicationDescription

metadata:
  id: factory-insights
  name: Factory Insights
  description: >
    Edge application that exposes a web UI and API for viewing local production KPIs,
    alarms, and device health data.
  version: 1.2.0
  catalog:
    application:
      tagline: Real-time local production insights
      descriptionFile: https://example.vendor.com/factory-insights/README.md
      icon: https://example.vendor.com/factory-insights/icon.png
      licenseFile: https://example.vendor.com/factory-insights/LICENSE.txt
      releaseNotes: https://example.vendor.com/factory-insights/RELEASE_NOTES.md
      site: https://example.vendor.com/factory-insights
      tags:
        - analytics
        - dashboard
        - edge
        - manufacturing
    organization:
      - name: Example Vendor GmbH
        site: https://example.vendor.com
    author:
      - name: Example Vendor Edge Apps Team
        email: edge-apps@example.vendor.com

deploymentProfiles:
  - type: helm.v3
    id: k8s-standard
    description: >
      Kubernetes deployment profile for Margo standalone cluster devices.
      Exposes the application via ingress and persists data in a PVC.
    requiredResources:
      cpu:
        cores: 2
        architectures:
          - amd64
          - arm64
      memory: 2Gi
      storage: 10Gi
      peripherals:
        - type: gpu
          manufacturer: NVIDIA
      interfaces:
        - type: ethernet
    components:
      - name: fi-web-helm
        properties:
          repository: oci://registry.example.vendor.com/charts/factory-insights
          revision: 1.2.0
          wait: true
          timeout: 10m0s

  - type: compose
    id: compose-standard
    description: >
      Compose deployment profile for Margo standalone devices.
      Uses environment variables for runtime configuration and a Docker-managed
      named volume or device-provided bind mount on the target device.
    requiredResources:
      cpu:
        cores: 1
        architectures:
          - amd64
          - arm64
      memory: 1Gi
      storage: 5Gi
      interfaces:
        - type: ethernet
    components:
      - name: fi-web-compose
        properties:
          packageLocation: https://downloads.example.vendor.com/factory-insights/factory-insights-compose-1.2.0.tar.gz
          keyLocation: https://downloads.example.vendor.com/factory-insights/signing-key.asc
          wait: true
          timeout: 5m0s

parameters:
  ingressHost:
    name: ingressHost
    # PROPOSED EXTENSION: tells Margo that the device supplies this value
    valueFrom:
      device:
        key: margo.cluster.hostname
        required: true
    targets:
      # Helm values.yaml target(s)
      - pointer: ingress.host
        components:
          - fi-web-helm
      - pointer: certificate.dnsNames[0]
        components:
          - fi-web-helm
      # Compose env var target(s)
      - pointer: APP_HOSTNAME
        components:
          - fi-web-compose

  ingressClass:
    name: ingressClass
    # PROPOSED EXTENSION
    valueFrom:
      device:
        key: margo.cluster.defaultIngressClass
        required: false
        fallback: nginx
    targets:
      - pointer: ingress.ingressClassName
        components:
          - fi-web-helm

  storageClass:
    name: storageClass
    # PROPOSED EXTENSION
    valueFrom:
      device:
        key: margo.cluster.defaultStorageClass
        required: false
    targets:
      - pointer: persistence.storageClass
        components:
          - fi-web-helm

  supportEmail:
    name: supportEmail
    # CURRENT-SPEC STYLE: normal user-supplied parameter with default value
    value: support@example.vendor.com
    targets:
      - pointer: app.supportEmail
        components:
          - fi-web-helm
      - pointer: SUPPORT_EMAIL
        components:
          - fi-web-compose

configuration:
  sections:
    - name: General
      settings:
        - parameter: supportEmail
          name: Support email
          description: >
            Contact email shown in the application's help/about panel.
          immutable: false
          schema: emailText

  schema:
    - name: emailText
      dataType: string
      allowEmpty: false
      minLength: 6
      maxLength: 254
      regexMatch: '^[^@\s]+@[^@\s]+\.[^@\s]+$'
```



#### DeviceCapabilitiesManifest Example

Inclusion of new `installContext` element, as a dictionary of device-supplied parameter values available for application installation and update workflows.


```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "DeviceCapabilitiesManifest",
  "properties": {
    "id": "northstarida.xtapro.k8s.edge",
    "vendor": "Northstar Industrial Devices",
    "modelNumber": "332ANZE1-N1",
    "serialNumber": "PF45343-AA",
    "roles": [
      "standalone cluster"
    ],
    "resources": {
      "cpu": [
        {
          "cores": 24,
          "architecture": "x86_64"
        }
      ],
      "memory": "59Gi",
      "storage": "1862Gi",
      "peripherals": [
        {
          "type": "gpu",
          "manufacturer": "NVIDIA",
          "model": "RTX A4000"
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
    "installContext": {
      "margo.cluster.hostname": {
        "type": "string",
        "value": "edge01.plant-a.example.com",
        "description": "Hostname/FQDN that applications should use for ingress, public URL generation, and certificate DNS names.",
        "mutable": true
      },
      "margo.cluster.defaultIngressClass": {
        "type": "string",
        "value": "nginx",
        "description": "Default ingress class to use for applications that expose HTTP/HTTPS endpoints through Kubernetes ingress.",
        "mutable": true
      },
      "margo.cluster.defaultStorageClass": {
        "type": "string",
        "value": "standard",
        "description": "Default Kubernetes StorageClass to use for application PVCs when the application requests persistent storage.",
        "mutable": true
      }
    }
  }
}
```

#### Compose File Example

This example assumes the application is a simple web service plus an NGINX reverse proxy. The important part is that the device-supplied values are consumed through environment variables. Compose supports defining persistent storage through named volumes or bind mounts, and unlike Kubernetes there is no real StorageClass equivalent in Compose, so this example uses a named volume for persistence.

```yaml
services:
  fi-app:
    image: registry.example.vendor.com/factory-insights:1.2.0
    container_name: factory-insights-app
    restart: unless-stopped
    environment:
      APP_HOSTNAME: ${APP_HOSTNAME}
      APP_DATA_DIR: /var/lib/factory-insights
    volumes:
      - fi-data:/var/lib/factory-insights
    expose:
      - "8080"

  fi-proxy:
    image: nginx:1.27-alpine
    container_name: factory-insights-proxy
    restart: unless-stopped
    depends_on:
      - fi-app
    ports:
      - "80:80"
    environment:
      APP_HOSTNAME: ${APP_HOSTNAME}
    volumes:
      - ./nginx/default.conf.template:/etc/nginx/templates/default.conf.template:ro

volumes:
  fi-data:
```



## Alternatives considered (optional)

> List any alternative solutions considered while working on the SUP and the reason for not choosing them. If the SUP owner knows that there are alternative SUPs being worked on, this section can be used to highlight potential advantages this SUP has over the alternatives.
> 
> Complete as part of Phase 3: SUP Technical Development

## Rejection reason

> If a SUP is rejected, indicate the reason why it was rejected.
> 
> Complete if SUP is rejected at Phase 2: Proposal Creation or Phase 4: Final Decision 
