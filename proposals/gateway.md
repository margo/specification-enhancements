# Specification Update Proposal

## Owner

Merrill Harriman, Julien Duquesnay

## Summary

We propose to enhance the Workload Fleet Manager to device interface to allow a client - a gateway service for example - to provide device capabilities, retrieve desired states, etc. on behalf of one or multiple target devices. 

The idea is to augment the Workload Fleet Manager interface to enable the connection of a "gateway service", while the communication between the "gateway service" and the target devices stays out of the scope of Margo (it could be a proprietary communication).

Additional services required by Margo to be provided by the device, e.g. OTEL collector, are not in the scope of this SUP.

We envision three types of gateway service:

* **Transparent gateway** - The WFM sees the devices behind the gateway as directly connected to it, the Margo client is moved from the device to the gateway. This is out of scope of the SUP as it does not require changes to the Margo API or the Margo artifacts as currently defined.
* **Opaque gateway** - the WFM sees the gateway as a single device with the combined capabilities of the devices behind the gateway. It is not aware of the devices behind the gateway. This is in the scope of this SUP.
* **See-thru gateway** - the WFM is aware of the devices and the gateway, it communicates with the devices via the gateway. This is in the scope of the gateway.

![Types of gateway (svg)](res/gateway/types-of-gateway.drawio.svg)

While the opaque gateway and see-thru gateway are quite different we believe the changes to the API and artifacts needed to enable them will be similar and propose to manage them in this SUP. If we find a strong divergence we can split the work into two SUP.

## Reason for proposal

These enhancements will allow devices that can host Margo applications but do not implement the Margo interface to be managed by a Margo compliant WFM, and thus decrease the barrier to entry for the device vendors. 

They could also allow managing devices that required a higher level of network isolation - no direct connection with the outside world for example.

These enhancements may also prepare the way for supporting constrained devices.

## Requirements alignment acknowledgement

This SUP is related to the following technical feature:

- https://github.com/margo/specification/issues/137

It is not intended to be part of PR1 as it introduces new functionalities. We expect that the changes it will introduce will be localized and will not challenge what has already been defined.

## Technical proposal

> The SUP's technical details. There must be enough technical details that someone can take the information in this section and implement it on their own.
> 
> Complete as part of Phase 3: SUP Technical Development

A Margo Gateway allows a Margo Workload Fleet Manager (WFM) to connect and deploy workloads to non-margo devices. It translates the Margo WFM's interfaces to the non-margo devices' interfaces.

A gateway and a compound device share concerns to manage sub-device and the proposed solution could work for both. We define a compound device as a device made of several independent sub-devices, e.g, 

* a device with two independent CPU (an Arm A core and an Arm M core for example) each with their own OS and memory.
* a modular or rack based device which can be extended with compute modules.

In terms of deployments, a gateway could have three modes of operation: 

* **autonomous** where the gateway decides on its own which sub-devices to use for the deployments provided by the WFM.
* **directed** where the WFM dictates the sub-device to use for each deployment.
* **mixed** where the WFM dictates the placement of some deployments and let the gateway decide the placement of the other deployments. 

An **opaque** gateway is a gateway that does not provide visibility on the sub-devices it manages, it simply provide the sum of the capabilities of the sub-devices to the WFM. It can only support the autonomous mode of operation.

A **see-thru** gateway ia gateway that provides visibility on the sub-devices it manages, providing the capabilities of each sub-devices to he WFM. It could support all three types of modes of operation.

![opaque gateway(svg)](./res/gateway/opaque-gateway.drawio.svg)
![See-through-gateway (svg)](./res/gateway/see-thru-gateway.drawio.svg)

In both cases, opaque or see-thru, the gateway is responsible for onboarding and verifying the identity of the sub-devices it manages.

The gateway interfaces with the WFM with 4 APIs:

* when **Onboarding** with the WFM
* when providing its **Capabilities**
* when retrieving its **Desired state** and deployments
* when providing **Deployment status**


### Onboarding

The gateway provides its own identity to the WFM.

No changes needed to the current definition of the onboarding API.


### Capabilities

#### Opaque gateway

No change necessary to the defined API. The gateway will simply reports the sum of the capabilities of all its sub-devices.

As a consequence, some deployments that may appear as acceptable from the combined capabilities may not work because the resources they require are spread among multiple sub-devices.

*Example:* An opaque gateway has two "standalone device" sub-devices. Each sub-device has an ARM64 processor with 2 cores and 5 GB of memory. The gateway will report capabilities of 4 cores and 10 GB of memory and a role of "Standalone Device". If the WFM wants to deploy an application requiring 2 cores and 6 GB of memory, it will look possible based on the capabilities reported but will fails because the gateway does not have a sub-device with 2 cores and 6 GB of memory available.

*Example:* An opaque gateway has two sub-device. One is the equivalent to "Standalone Cluster" sub-device (supporting helm) and the other one is the equivalent to a "Standalone Device" (supporting compose). The Gateway will report both roles, Standalone Cluster and Standalone Device, in its capability payload. 

#### See-thru gateway

For the WFM to be able to assign deployment to specific sub-devices (or child devices) it must be made aware of all the available sub-devices and their capabilities (including roles to know if they can deploy compose file or helm chart, and resources).

We assign an id to each child device to differentiate them. How this id is assigned is outside of the scope of Margo, it could be assigned by the Device Management or by the gateway directly. The only requirements are that this id must be unique for a given gateway and can include only Unreserved Characters as specified in [RFC3986](https://www.rfc-editor.org/rfc/rfc3986#section-2.3) plus the path separator (i.e. '/').

To allow for a client to report the capabilities (add) of multiple devices, parents and children, and remove devices from the client we propose to modify the endpoint to include the top-level device id as a mandatory path parameter and children device ids as optional path parameters. 

```
POST /api/v1/clients/{clientId}/capabilities/{device-id}[/{device-id}[/{device-id}...]]
PUT /api/v1/clients/{clientId}/capabilities/{device-id}[/{device-id}[/{device-id}...]]
DELETE /api/v1/clients/{clientId}/capabilities/{device-id}[/{device-id}[/{device-id}...]]
```

The top-level `device-id` must be unique for a given `clientId` and must include only Unreserved Characters as specified in [RFC3986](https://www.rfc-editor.org/rfc/rfc3986#section-2.3).  
The children `device-id` must be unique for a given parent `device-id` and they must include only Unreserved Characters as specified in [RFC3986](https://www.rfc-editor.org/rfc/rfc3986#section-2.3) plus the path separator (i.e. '/').

Note: using multiple device ids in the endpoint does not register multiple devices in a single request, but indicates a hierarchy of devices, with a parent/child relationship.

We propose the following script for the reporting of the sub-devices and their capabilities by the gateway to the WFM:

1. The gateway reports its own capabilities, indicating it is a gateway by using the new `Gateway` value in the roles attribute.

    ```
    POST /api/v1/clients/{clientId}/capabilities/{device-id}
    PUT /api/v1/clients/{clientId}/capabilities/{device-id}
    ```

    ```json
    {
        "apiVersion": "device.margo.org/v1alpha1",
        "kind": "DeviceCapabilitiesManifest",
        "properties": {
            "id": "gateway01",
            "vendor": "Northstar Industrial devices",
            "modelNumber": "332ANZE1-N1",
            "serialNumber": "PF45343-AA",
            "roles": ["Gateway"]
        }
    }
    ```

    If the gateway has a single role, "Gateway", then the `resources` array of the `properties` section can be omitted.  
    If the gateway has additional roles (e.g. Standalone Cluster, Standalone Device, or Cluster Leader) it will add them to the `roles` array and will need to provide the resources available for those roles as well in the `resources` array.

2. The gateway reports the capabilities of all its sub-devices, one at a time.

    The sub-device `device-id` is appended to the endpoint URL to indicate that the capability being reported is for a sub-device managed by the gateway. It is also included in the `properties.id` attribute of the request body.
    
    ```
    POST /api/v1/clients/{clientId}/capabilities/{device-id}/{device-id}
    PUT /api/v1/clients/{clientId}/capabilities/{device-id}/{device-id}
    ```

    ```json
    {
        "apiVersion": "device.margo.org/v1alpha1",
        "kind": "DeviceCapabilitiesManifest",
        "properties": {
              "id": "gateway01/dev01",
              "vendor": "ACME Devices",
              "modelNumber": "11AD01",
              "serialNumber": "11AD012026010001",
              "roles": [
                  "Standalone Device"
              ],
              "resources": {
                  ...
              }
        }
    }
    ```

3. To remove a sub-device from the WFM, the gateway will use the DELETE method on the same endpoint.

    ```
    DELETE /api/v1/clients/{clientId}/capabilities/{device-id}/{device-id}
    ```

The request body structure for the PUT/POST methods does not change, but some small changes are needed for some of the `Properties` attributes:

| Field | Type | Required? | Description |
| --- | --- | :---: | --- | 
| id | string | Y | **Unique deviceID assigned to the device via the Device Owner. It must include only Unreserved Characters as specified in [RFC3986](https://www.rfc-editor.org/rfc/rfc3986#section-2.3) plus the path separator (i.e. '/'). In case of a device behind a gateway, it takes the form of a path with the id of the parent gateway and the id of the child device, i.e., "{device-id}/{device-id}". The top-level {device-id} must be unique for a given {clientId}, and the children {device-id} must be unique for a given parent {device-id}.** |
| vendor | string | Y | Defines the device vendor. |
| modelNumber | string | Y | Defines the model number of the device. |
| serialNumber | string | Y | Defines the serial number of the device. |
| roles | []string | Y | **Element that defines the device role it can provide to the Margo environment. MUST be one of the following: Standalone Cluster, Cluster Leader, Standalone Device, or Gateway** |
| resources | []Resource | **N** | **Element that defines the device's resources available to the application deployed on the device. See the Resource Fields section below.<br> The element is required if the device has any of the following roles: Standalone Cluster, Cluster Leader, Standalone Device** |


### Desired state

To allow the WFM to assign deployments to specific devices or sub-devices, we propose to augment the desired state payload to include the device id for each deployment. We propose to add `deviceId` as a mandatory attribute to the `metadata` section of the `ApplicationDeployment` yaml. This attribute will contain the id of the device, with hierarchy if applicable, to which the deployment is assigned (format `{device-id}[/{device-id}[/...]]`).

If the gateway itself is capable of hosting deployments, then the `deviceId` attribute will contain just the gateway id for the deployments targeted for the gateway.

If the gateway is capable of autonomously placing deployments on its sub-devices (autonomous mode), the WFM can request the gateway to do so by using `*` for the last child device id (e.g. `gateway01/*`) in the `deviceId` attribute. The `*` value is only valid for the last device id in the hierarchy.

New `metadata` attribute:

| Attribute	| Type | Required? | Description |
| :--- | :--- | :--- | :--- |
| `deviceId` | string | Y | the device id to which the deployment is assigned. To reference a sub-device the format is `{device-id}[/{device-id}[/...]]`. To request the gateway to choose the sub-device, use `*` for the last device id in the hierarchy (i.e. `{device-id}/[{device-id}/.../]*`). |

```yaml
apiVersion: application.margo.org/v1alpha1
kind: ApplicationDeployment
metadata:
  annotations:
    id: 
    applicationId: 
  name: 
  namespace: 
  deviceId:
spec:
    deploymentProfile:
        type: 
        components:
            - name: 
              properties:
    parameters:
        param:
            value: 
            targets:
                - pointer: 
                  components:[]
```

The assumption is that the gateway will need to convert the content of the `ApplicationDeployment` yaml file into something understandable by the sub-device. So putting this information into the yaml file does not really inconvenience the gateway. 


### Deployment Status

#### deviceId attribute

We propose to add a new attribute to the deployment status API request body: `deviceId`.

While not necessary to report the status of a deployment since each deployment has its own unique ID, it allows for a see-thru gateway in autonomous or mixed mode to inform about the sub-device it has selected for the deployment. 

If the gateway was requested to deploy on itself, then the `deviceId` attribute will contain just the id of the gateway.

If the gateway was requested to autonomously place the deployment on a sub-device (using `*` in the `deviceId` attribute of the desired state), then the `deviceId` attribute contains the id of the sub-device selected by the gateway for the deployment. If the gateway could not place the deployment because no sub-device was available, then the `deviceId` attribute matches the one in the desired state (`{device-id}[/...]/*`). If the gateway decides to move te deployment to another sub-device later, it will send a new deployment status report with the new sub-device id.

New attribute: 

| Fields | Type | Required? | Description |
| :--- | :--- | :--- | :--- |
| `deviceId` | string | Y | Device hosting the deployment. Includes the full device hierarchy if applicable. |

#### Error codes/messages for sub-device not available

We propose to define two error codes/messages for the `status` attribute to handle cases when the deployment fails because the sub-device is not available. 

| Use case | Error code | Error message |
| --- | --- | --- |
| Child device ID is not known by the gateway | 101 | Unknown child device ID | 
| Child device is not reachable by the gateway | 102 | Child device unreachable |

To avoid error number collision we propose to enhance the error structure with a `source` attribute. 

* When the error is generated by the gateway, the source is set to the device id of the gateway (with its full hierarchy if applicable). 
* When the error is not generated by the gateway, 
    * the source for the `status.error` attribute is set to the device id of the child device (with its full hierarchy) as used in the `deviceId` attribute.
    * the source for the `components.error` attribute is set to the component name as used in `components[].name` for that component.

New attribute in `error` structure:

| Field | Type | Required? | Description |
| :--- | :--- | :--- | :--- |
| `source` | string | Y | Identifies the source of the error. It is set to the device id, with its full hierarchy if applicable, of the device generating the error, or to the component name of the component generating the error. |


#### Examples

##### Child device unreachable error

```json
{
    "apiVersion": "deployment.margo.org/v1alpha1",
    "kind": "DeploymentStatusManifest",
    "deviceId": "gateway01/subdevice01",
    "deploymentId": "a3e2f5dc-912e-494f-8395-52cf3769bc06",
    "status": {
        "state": "failed",
        "error": {
            "code": "102",
            "source": "gateway01",
            "message": "Child device unreachable"
        }
    },
    "components": [
        {
            "name": "digitron-orchestrator",
            "state": "failed",
            "error": {
                "code":"102",
                "source":"gateway01",
                "message":"Child device unreachable"
            }
        },
        {
            "name": "database-services",
            "state": "failed",
            "error": {
                "code": "102",
                "source":"gateway01",
                "message":"Child device unreachable"
            }
        }
    ]
}
```

##### Component deployment error

```json
{
    "apiVersion": "deployment.margo.org/v1alpha1",
    "kind": "DeploymentStatusManifest",
    "deviceId": "gateway01/subdevice01",
    "deploymentId": "a3e2f5dc-912e-494f-8395-52cf3769bc06",
    "status": {
        "state": "failed",
        "error": {
            "code": "1",
            "source": "gateway01/subdevice01",
            "message": "Not enough resources"
        }
    },
    "components": [
        {
            "name": "digitron-orchestrator",
            "state": "pending"
        },
        {
            "name": "database-services",
            "state": "failed",
            "error": {
                "code": "1",
                "source":"database-services",
                "message":"Not enough resources"
            }
        }
    ]
}
```

## Alternatives considered (optional)

> List any alternative solutions considered while working on the SUP and the reason for not choosing them. If the SUP owner knows that there is a risk of a competing SUP, this section can be used to make their case ahead of any potential votes on why their solution is better.
> 
> Complete as part of Phase 3: SUP Technical Development

A few alternatives options were explored. The option we selected appeared to be the most elegant to us.

### Capabilities

#### Alternative Option A - add array of sub-devices

A new optional array, `subDevices`, is added to the `properties` section to provide the roles and resources of each sub-device. This array is needed only in the case of a gateway. If this array is present then the `properties.roles` and `properties.resources` attributes should be omitted or left empty.

New attribute in `properties` section:

| Attribute	| Type | Required? | Description |
| :--- | :--- | :--- | :--- |
| subDevices | array | N |  |
| subDevices[].id | string | Y | Id of the sub-device. Assigned by the Device Management. |
| subDevices[].vendor | string | Y | Defines the device vendor. |
| subDevices[].modelNumber | string | Y | Defines the model number of the device. |
| subDevices[].serialNumber | string | Y | Defines the serial number of the device. |
| subDevices[].roles | []string | Y | Role(s) of the sub-device. MUST be selected from following: Standalone Cluster, Cluster Leader, or Standalone Device. |
| subDevices[].resources | Resource | Y | Element that defines the sub-device's resources available to the application deployed on the device. See the Resource Fields section below. |

Changes to existing attributes of the `properties` section:

| Attribute	| Type | Required? | Description |
| :--- | :--- | :--- | :--- |
| roles | []string | **N** | Element that defines the device role it can provide to the Margo environment. MUST be one of the following: Standalone Cluster, Cluster Leader, or Standalone Device. **Required if `properties.subDevices` is not present, otherwise it should be ignored if present.** |
| resources | []Resource | **N** | Element that defines the device's resources available to the application deployed on the device. See the Resource Fields section below. **Required if `properties.subDevices` is not present, otherwise it should be ignored if present.** | 

> [!NOTE]
> In a separate SUP we should explore the idea of replacing the `roles` attribute with a `supportedDeployments` attribute that would list the types of deployment supported by the device, i.e., `compose` and `helm.v3`.

```json
{
    "apiVersion": "device.margo.org/v1alpha1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
        "id": "device.c",
        "vendor": "Northstar Industrial devices",
        "modelNumber": "332ANZE1-N1",
        "serialNumber": "PF45343-AA",
        "roles": [],
        "subDevices": [
            {
                "id": "001",
                "vendor": "ACME Devices",
                "ModelNumber": "11AD01",
                "SerialNumber": "11AD012026010001",
                "roles": [
                    "standalone Cluster",
                    "Cluster Leader"
                ],
                "resources": {
                  ...
                }
            },
            {
                "id": "002",
                "vendor": "ACME Devices",
                "ModelNumber": "01AD55",
                "SerialNumber": "01AD5520255200100",
                "roles": [
                    "standalone device"
                ],
                "resources": {
                  ...
                }            
            } 
        ]
    }
}
```

### Desired state

#### Alternative Option A - use sub-device id in endpoint

**Endpoints**:

* **Manifest**: `GET /api/v1/clients/{clientId}-{subDeviceId}/deployments`
* **Individual deployment**: `GET /api/v1/clients/{clientsId}-{subDeviceId}/deployments/{deploymentId}/{digest}`
* **Bundle**: `GET /api/v1/clients/{clientId}-{subDeviceId}/bundles/{digest}`

e.g.:

```
GET /api/v1/clients/client.c-001/deployments
GET /api/v1/clients/client.c-002/deployments
```

No change to the request bodies and response bodies of the different API.

#### Alternative Option B - organize deployment by sub-device in manifest

```json
{
  "manifestVersion": 101,
  "subDevices": [
    {
      "subDeviceId": "001"
      "bundle": {
        ...
      },
      "deployments": {
        ...
      }
    }
  ]
  "bundle": {
    "mediaType": "application/vnd.margo.bundle.v1+tar+gzip",
    "digest": "sha256:b5c6d7e8f9...",
    "url": "/api/v1/clients/northstarida.xtapro.k8s.edge/bundles/sha256:b5c6d7e8f9..."
  },
  "deployments": [
    {
      "deploymentId": "a3e2f5dc-912e-494f-8395-52cf3769bc06",
      "applicationId": "com-northstartida-digitron-orchestrator",
      "version": "2.1.1",
      "digest": "sha256:a4e01b2c3d...",
      "url": "/api/v1/clients/northstarida.xtapro.k8s.edge/deployments/a3e2f5dc-912e-494f-8395-52cf3769bc06"
    }
  ]
}
```
New attribute:

| Field | Type | Required? | Description |
| :--- | :--- | :--- | :--- |
| `subDevices` | array | N | list of sub-devices |
| `subDevices[].subDeviceId` | string | Y | |
| `subDevices[].bundle` | object | N | |
| `subDevices[].bundle.mediaType`| string | Y | The format of the bundle. For `application/vnd.margo.bundle.v1+tar+gzip`, the archive **MUST** contain the individual `ApplicationDeployment` YAML files in its root folder |
| `subDevices[].bundle.digest` | string | Y | The [digest](#digest-specification) of the bundle archive for integrity verification |
| `subDevices[].bundle.url` | string | Y | The endpoint to retrieve the bundle |
| `subDevices[].deployments` | array | Y | |
| `subDevices[].deployments[].deploymentId`| string | Y | The unique UUID from the `ApplicationDeployment`'s [`metadata.annotations.id`](https://specification.margo.org/margo-api-reference/workload-api/desired-state-api/desired-state/#annotations-attributes) |
| `subDevices[].deployments[].applicationId`| string | Y | An identifier from the associated [`ApplicationDescription`](https://specification.margo.org/margo-api-reference/workload-api/application-package-api/application-description/) for context |
| `subDevices[].deployments[].version`| string | Y | An identifier from the associated [`ApplicationDescription`](https://specification.margo.org/margo-api-reference/workload-api/application-package-api/application-description/) for context |
| `subDevices[].deployments[].digest` | string | Y | The [digest](#digest-specification) of the individual `ApplicationDeployment` YAML file |
| `subDevices[].deployments[].url` | string | Y | |

#### Alternative Option C - add sub-device to deployment info in manifest 

```json
{
  "manifestVersion": 101,

  "bundle": {
    "mediaType": "application/vnd.margo.bundle.v1+tar+gzip",
    "digest": "sha256:b5c6d7e8f9...",
    "url": "/api/v1/clients/northstarida.xtapro.k8s.edge/bundles/sha256:b5c6d7e8f9..."
  },
  "deployments": [
    {
      "deploymentId": "a3e2f5dc-912e-494f-8395-52cf3769bc06",
      "applicationId": "com-northstartida-digitron-orchestrator",
      "version": "2.1.1",
      "digest": "sha256:a4e01b2c3d...",
      "url": "/api/v1/clients/northstarida.xtapro.k8s.edge/deployments/a3e2f5dc-912e-494f-8395-52cf3769bc06",
      "subDeviceId": "001"
    }
  ]
}
```

New attribute: 

| Field | Type | Required? | Description |
| :--- | :--- | :--- | :--- |
| `deployments[].subDeviceId` | string | N | ID of the sub-device to which this deployment is assigned in case of gateway |

### Deployment status

#### Alternative option A: no change

Since the deployment ID is unique for each deployment on a device, no change is required to the defined interface for the gateway to report the status a deployment.

#### Alternative option B - use sub-device id in endpoint

**Endpoint**: `POST /api/v1/clients/{clientId}-{subDeviceId}/deployments/{deploymentId}/status`

While not really necessary, this option makes sense if the endpoints used for the desired state also uses the sub-device id. This allows the WFM to treat each sub-device like they are regular device and no other changes is necessary.


## Rejection reason

> If a SUP is rejected, indicate the reason why it was rejected.
> 
> Complete if SUP is rejected at Phase 2: Proposal Creation or Phase 4: Final Decision 
