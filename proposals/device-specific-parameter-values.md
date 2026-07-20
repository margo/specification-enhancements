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

This SUP is needed because the current Margo specification already recognizes application parameters as a first-class concept, but it models them as values that are provided by the user at install time. Currently, configurable parameters are defined so the end user can specify values. 

Today, a parameter can target Helm values using dot notation or Compose environment variables, which is good because it lets an application developer say where the value must land during deployment. But the spec does not define a standard mechanism for saying that a value should come from the device itself rather than from manual user input. That means the spec handles parameter injection **but does not handle parameter resolution from device context**.

The device capabilities model already requires the device to report characteristics such as role, CPU, memory, storage, peripherals, and interfaces to the Workload Fleet Manager (WFM), and it says the device client must update this information if it changes. However, those reported capabilities are primarily for describing the device and matching workloads, not for supplying install-time values like a hostname or the storage class name to be used by devices. 

Examples of such device-supplied parameter values are related to its communication or computation specifics:
* The application needs to know which **hostname** or **IP address** the device uses. Then the application is able to use it as external endpoint (ingress) or within a TLS certificate. If Margo had a device-supplied parameter for the device's hostname (or IP), the device could resolve it once and use it for deployment. That would spare the user from having to discover and enter the value manually.
* The user deploying a Margo Helm-based application may not know which Kubernetes **storage class** (e.g., standard, fast-ssd) exists on a device, or which one the device vendor wants apps to use. That is why the device can often discover the storage class more reliably than the user.
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
```

**Proposed semantics**

Extension of `parameters`:

* `valueFrom` (optional): Indicates that the parameter value is to be resolved from a non-user source before installation or update.
* For GA1, the only standard source is `device`.
* `device.key` (required): Identifier of the device-supplied parameter to resolve.
* `device.required` (optional): If true, installation MUST fail if the key cannot be resolved. Default is true.
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

Before the device installs or updates an application, the device’s Workload Fleet Management Client (WFMC) resolves all parameters that use `valueFrom.device`. This is done prior to invoking the deployment provider (e.g., Helm or Compose). The device MUST resolve all `valueFrom.device` references to concrete values.

If a referenced device key cannot be resolved, then the `value` is used as a fallback, if it is defined, otherwise installation/update MUST fail with a clear validation/resolution error.


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
    "margo.cluster.ingressClass": {
      "type": "string",
      "value": "nginx",
      "description": "Default ingress class to use for applications that expose HTTP/HTTPS endpoints through Kubernetes ingress.",
      "mutable": true
    },
  ...
```


### Margo-reserved Keys

* `margo.cluster.hostname` - FQDN used for externally reachable application URLs and derived certification DNS names. This is the FQDN which the device (or the cluster to which the device belongs) uses as a basis to expose apps. App can either: create specific subdomains (e.g., host name = `apps.plant-a.example.com` and application can derive concrete hostname as subdomain = `factory-insights.apps.plant-a.example.com`) or use the base host name and create path-based routing to differentiate between applications (e.g., `apps.plant-a.example.com/factory-insights`).

* `margo.cluster.storageClass` - For PersistentVolume/PersistentVolumeClaim on Kubernetes devices.

* `margo.cluster.ingressClass` - The recommended ingress class to use by applications.

* `margo.device.dataPath` - Device-defined path to a directory where the app should store its data.

### Example 

In the `ApplicationDescription` and `DeviceCapabilitiesManifest` examples below, the device reports its capabilities, now including the proposed `installContext` dictionary. The current spec already has the device capability reporting flow.

Before installation, the device-side WFMC resolves:

* `ingressHost` from `margo.cluster.hostname`
* `storageClass` from `margo.cluster.storageClass`

The resolved values are then injected into:

* Helm values (ingress.host, certificate.dnsNames[0], persistence.storageClass)
or 
* Compose environment variables (APP_HOSTNAME)



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
        key: margo.cluster.ingressClass
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
        key: margo.cluster.storageClass
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
      "margo.device.dataPath": {
        "type": "string",
        "value": "/srv/margo/app-data/factory-insights",
        "description": "Device-defined path to a directory where the app should store its data.",
        "mutable": true
      },
      "margo.cluster.ingressClass": {
        "type": "string",
        "value": "nginx",
        "description": "Default ingress class to use for applications that expose HTTP/HTTPS endpoints through Kubernetes ingress.",
        "mutable": true
      },
      "margo.cluster.storageClass": {
        "type": "string",
        "value": "edge-fast-local",
        "description": "Kubernetes preferred StorageClass  when the application requests persistent storage.",
        "mutable": true
      }    
    }
  }
}
```

#### Compose File Example

This example assumes the application is a simple web service plus an NGINX reverse proxy. The important part is that the device-supplied values are consumed through environment variables. 

The ``APP_HOSTNAME`` is resolved from the `margo.cluster.hostname` as specified in DeviceCapabilitiesManifest.
The ``APP_DATA_HOST_PATH`` is resolved from the `margo.device.dataPath` as specified in DeviceCapabilitiesManifest. This is the directory inside the container where the application writes its persistent data. As this is modelled as a device supplied parameter, the WMFC decides where data lives on the device.

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
      - ${APP_DATA_HOST_PATH}:/var/lib/factory-insights
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
```

The values for these environment variables are resolved before Compose starts the container.
Below is an example of an .env file that the WMFC would generate after resolving the device-supplied parameters:

```
APP_HOSTNAME=edge01.plant-a.example.com
APP_DATA_HOST_PATH=/srv/margo/app-data/factory-insights
```

#### Helm Chart Example

The following is a minimal Helm chart example showing how to use the echanics of this SUP in an application deployment using Margo Helm, the ``ApplicationDescription`` example shown above, and the device-supplied parameters (e.g. hostname, storageClassName, etc.).

``` 
factory-insights/
  Chart.yaml
  values.yaml
  templates/
    deployment.yaml
    service.yaml
    ingress.yaml
    pvc.yaml
```

##### 1. Chart.yaml

This is the entry point to the Helm specified application.

```yaml
apiVersion: v2
name: factory-insights
description: Helm chart for Factory Insights edge application
type: application
version: 1.2.0
appVersion: "1.2.0"
```

##### 2. values.yaml

This is the key part: after resolving ``valueFrom.device`` keys in the ``ApplicationDescription``, concrete values are resolved as inidcated in the comments of the example below.

```yaml
replicaCount: 1

image:
  repository: registry.example.vendor.com/factory-insights
  tag: "1.2.0"
  pullPolicy: IfNotPresent

service:
  port: 8080

app:
  supportEmail: "support@example.vendor.com"

ingress:
  enabled: true
  ingressClassName: "" # after resolving: "nginx"
  host: "" # after resolving: "edge01.plant-a.example.com"
  path: /
  pathType: Prefix

persistence:
  enabled: true
  size: 10Gi
  storageClassName: "" # after resolving: "edge-fast-local"
```

In summary, the mapping between device-specified parameter values and Helm Chart values is as such:

| Device-specified parameter | Helm Chart value |
| --------------- | ---------- |
| margo.cluster.storageClass | persistence.storageClassName |
| margo.cluster.ingressClass | ingress.ingressClassName |
| margo.cluster.hostname | ingress.host | 


##### 3. templates/deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}
  labels:
    app.kubernetes.io/name: factory-insights
    app.kubernetes.io/instance: {{ .Release.Name }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app.kubernetes.io/name: factory-insights
      app.kubernetes.io/instance: {{ .Release.Name }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: factory-insights
        app.kubernetes.io/instance: {{ .Release.Name }}
    spec:
      containers:
        - name: factory-insights
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - containerPort: {{ .Values.service.port }}

          env:
            - name: SUPPORT_EMAIL
              value: {{ .Values.app.supportEmail | quote }}

          volumeMounts:
            - name: app-data
              mountPath: /var/lib/factory-insights

      volumes:
        - name: app-data
          persistentVolumeClaim:
            claimName: {{ .Release.Name }}-data
```

##### 4.  templates/service.yaml

```yaml
apiVersion: v1
kind: Service
metadata:
  name: {{ .Release.Name }}
spec:
  selector:
    app.kubernetes.io/name: factory-insights
    app.kubernetes.io/instance: {{ .Release.Name }}
  ports:
    - port: 80
      targetPort: {{ .Values.service.port }}
      protocol: TCP
```

##### 5. templates/service.yaml

```yaml
{{- if .Values.ingress.enabled }}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .Release.Name }}
spec:
  {{- if .Values.ingress.ingressClassName }}
  ingressClassName: {{ .Values.ingress.ingressClassName | quote }}
  {{- end }}

  rules:
    - host: {{ .Values.ingress.host | quote }}
      http:
        paths:
          - path: {{ .Values.ingress.path | quote }}
            pathType: {{ .Values.ingress.pathType }}
            backend:
              service:
                name: {{ .Release.Name }}
                port:
                  number: 80
{{- end }}
```

##### 6. templates/pvc.yaml

```yaml
{{- if .Values.persistence.enabled }}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ .Release.Name }}-data
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: {{ .Values.persistence.size }}

  {{- if .Values.persistence.storageClassName }}
  storageClassName: {{ .Values.persistence.storageClassName | quote }}
  {{- end }}
{{- end }}
```

## Alternatives considered (optional)

> List any alternative solutions considered while working on the SUP and the reason for not choosing them. If the SUP owner knows that there are alternative SUPs being worked on, this section can be used to highlight potential advantages this SUP has over the alternatives.
> 
> Complete as part of Phase 3: SUP Technical Development

## Rejection reason

> If a SUP is rejected, indicate the reason why it was rejected.
> 
> Complete if SUP is rejected at Phase 2: Proposal Creation or Phase 4: Final Decision 
