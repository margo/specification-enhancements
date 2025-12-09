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

While the opaque gateway and see-thru gateway are quiet different we believe the changes to the API and artifacts needed to enable them will be similar and propose the manage them in this SUP. If we find a strong divergence we can split the work into two SUP.

## Reason for proposal

These enhancements will allow devices that can host Margo applications but do not implement the Margo interface to be managed by a Margo compliant WFM, and thus decrease the barrier to entry for the device vendors. 

They could also allow managing devices that required a higher level of network isolation - no direct connection with the outside world for example.

These enhancements may also prepare the way for supporting constrained devices.

## Requirements alignment acknowledgement

This SUP is related to the following technical features:

- https://github.com/margo/specification/issues/99
- https://github.com/margo/specification/issues/101

It is not intended to be part of PR1 as it introduce new functionalities. We expect that the changes it will introduce will be localized and will not challenge what has already been defined.

## Technical proposal

> The SUP's technical details. There must be enough technical details that someone can take the information in this section and implement it on their own.
> 
> Complete as part of Phase 3: SUP Technical Development

A Margo Gateway allows a Margo Workload Fleet Manager (WFM) to connect and deploy workloads to non-margo devices. It translates the Margo WFM's interfaces to the non-margo devices' interfaces.

The gateway could be viewed as a special type of compound device. Where a compound device is defined as a device made of several independent sub-devices, e.g, 

* a device with two independent CPU (an Arm A core and an Arm M core for example) each with their own OS and memory.
* a modular or rack based device which can be extended with compute modules.

In terms of deployments, a compound device could have three modes of operation: 

* **autonomous** where the device decide on its own which sub-device to use for the deployments provided by the WFM.
* **directed** where the WFM dictates the sub-device to use for each deployment.
* **mixed** where the WFM dictates the placement of some deployments and let the device decide where to place the others. 

An **opaque** gateway is a gateway that does not provide visibility on the sub-devices it manages, it simply provide the sum of the capabilities of the sub-devices to the WFM. It can only support the autonomous mode of operation.

A **see-thru** gateway ia gateway that provides visibility on the sub-devices it manages, providing the capabilities of each sub-devices to he WFM. It could support all three types of modes of operation.

![opaque gateway(svg)](./res/gateway/opaque-gateway.drawio.svg)
![See-through-gateway (svg)](./res/gateway/see-thru-gateway.drawio.svg)

In both cases, opaque or see-thru, the gateway is responsible for onboarding and verifying the identity of the sub-devices it manages.

The compound device/gateway interfaces with the WFM with 4 APIs:

* when **Onboarding** with the WFM
* when providing its **Capabilities**
* when retrieving its **Desired state** and deployments
* when providing **Deployment status**


### Onboarding

The compound device/gateway provides its own identity to the WFM.

No changes needed to the current definition of the onboarding API.


### Capabilities

#### Opaque gateway

No change necessary to the defined API. The gateway will simply reports the sum of the capabilities of all its sub-devices.

As a consequence, some deployments that may appear as acceptable from the combined capabilities may not work because the resources they require are spread among multiple sub-devices.

#### See-thru gateway

For the WFM to be able to assign deployment to specific sub-devices it must be made aware of all the available sub-devices and their capabilities (including roles to know if they can deploy compose file or helm chart, and resources).

We assign an id to each sub-device to differentiate them. This id should be assigned by the Device Management, but it could be assigned by the gateway.

A new optional array, `subDevices`, is added to the `properties` section to provide the roles and resources of each sub-device. This array is needed only in the case of a compound device. If this array is present then the `properties.roles` and `properties.resources` attributes should be omitted or left empty.

New attribute in `properties` section:

| Attribute	| Type | Required? | Description |
| :--- | :--- | :--- | :--- |
| subDevices | array | N |  |
| subDevices[].id | string | Y | Id of the sub-device. Assigned by the Device Management. |
| subDevices[].roles | []string | Y | Role(s) of the sub-device. MUST be selected from following: Standalone Cluster, Cluster Leader, or Standalone Device. |
| subDevices[].resources | Resource | Y | Element that defines the sub-device's resources available to the application deployed on the device. See the Resource Fields section below. |

Changes to existing attributes:

| Attribute	| Type | Required? | Description |
| :--- | :--- | :--- | :--- |
| roles | []string | **N** | Element that defines the device role it can provide to the Margo environment. MUST be one of the following: Standalone Cluster, Cluster Leader, or Standalone Device. **Required if `properties.subDevices` is not present, otherwise it should be ignored if present.** |
| resources | []Resource | **N** | Element that defines the device's resources available to the application deployed on the device. See the Resource Fields section below. **Required if `properties.subDevices` is not present, otherwise it should be ignored if present.** | 

> [!NOTE]
> In a separate SUP we should explore the idea of replacing the `roles` attribute with a `supportedDeployments` attribute that would list the types of deployment supported by the device, i.e., `compose` and `helm.v3`.

```json
{
    "apiVersion": "device.margo/v1",
    "kind": "DeviceCapabilities",
    "properties": {
        "id": "device.c",
        "vendor": "Northstar Industrial devices",
        "modelNumber": "332ANZE1-N1",
        "serialNumber": "PF45343-AA",
        "roles": [],
        "subDevices": [
            {
                "id": "001",
                "roles": [
                    "standalone Cluster",
                    "Cluster Leader"
                ],
                "properties": {
                  ...
                }
            },
            {
                "id": "002",
                "roles": [
                    "standalone device"
                ],
                "properties": {
                  ...
                }            
            } 
        ]
    }
}
```


### Desired state

For the **directed** and **mixed** operating modes the WFM needs to associate a deployment with a specific sub-device.

We propose to do that by adding a new optional attribute to the `ApplicationDeployment` yaml: `target`.

New `deploymentProfile` attribute:

| Attribute	| Type | Required? | Description |
| :--- | :--- | :--- | :--- |
| target | string | N | the sub-device id to which the deployment is assigned if compound device. |

```yaml
apiVersion: application.margo.org/v1alpha1
kind: ApplicationDeployment
metadata:
  annotations:
    id: 
    applicationId: 
  name: 
  namespace: 
spec:
    deploymentProfile:
        target:
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

We propose to add a new optional attribute to the deployment status API request body: `subDeviceId`.

While not necessary to report the status of a deployment since each deployment has its own unique ID, it allows for a see-thru gateway in autonomous or mixed mode to inform about the sub-device it has selected for the deployment. 

New attribute: 

| Fields | Type | Required? | Description |
| :--- | :--- | :--- | :--- |
| `subDeviceId` | string | N | sub-device hosting the deployment |

```json
{
    "apiVersion": "deployment.margo/v1",
    "kind": "DeploymentStatus",
    "subDeviceId": "001",
    "deploymentId": "a3e2f5dc-912e-494f-8395-52cf3769bc06",
    "status": {
        "state": "pending",
        "error": {
            "code": "",
            "message": ""
        }
    },
    "components": [
        {
            "name": "digitron-orchestrator",
            "state": "pending",
            "error": {
                "code":"",
                "message":""
            }
        },
        {
            "name": "database-services",
            "state": "pending",
            "error": {
                "code": "",
                "message ": ""
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

### Desired state

#### Alternative Option A - use sub-device id in endpoint

**Endpoints**:

* **Manifest**: `GET /api/v1/devices/{deviceId}-{subDeviceId}/deployments`
* **Individual deployment**: `GET /api/v1/devices/{deviceId}-{subDeviceId}/deployments/{deploymentId}`
* **Bundle**: `GET /api/v1/devices/{deviceId}-{subDeviceId}/bundles/{bundleDigest}`

e.g.:

```
GET /api/v1/devices/device.c-001/deployments
GET /api/v1/devices/device.c-002/deployments
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
    "url": "/api/v1/devices/northstarida.xtapro.k8s.edge/bundles/sha256:b5c6d7e8f9..."
  },
  "deployments": [
    {
      "deploymentId": "a3e2f5dc-912e-494f-8395-52cf3769bc06",
      "applicationId": "com-northstartida-digitron-orchestrator",
      "version": "2.1.1",
      "digest": "sha256:a4e01b2c3d...",
      "url": "/api/v1/devices/northstarida.xtapro.k8s.edge/deployments/a3e2f5dc-912e-494f-8395-52cf3769bc06"
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
| `subDevices[].deployment[].url` | string | Y | |

#### Alternative Option C - add sub-device to deployment info in manifest 

```json
{
  "manifestVersion": 101,

  "bundle": {
    "mediaType": "application/vnd.margo.bundle.v1+tar+gzip",
    "digest": "sha256:b5c6d7e8f9...",
    "url": "/api/v1/devices/northstarida.xtapro.k8s.edge/bundles/sha256:b5c6d7e8f9..."
  },
  "deployments": [
    {
      "deploymentId": "a3e2f5dc-912e-494f-8395-52cf3769bc06",
      "applicationId": "com-northstartida-digitron-orchestrator",
      "version": "2.1.1",
      "digest": "sha256:a4e01b2c3d...",
      "url": "/api/v1/devices/northstarida.xtapro.k8s.edge/deployments/a3e2f5dc-912e-494f-8395-52cf3769bc06",
      "subDeviceId": "001"
    }
  ]
}
```

New attribute: 

| Field | Type | Required? | Description |
| :--- | :--- | :--- | :--- |
| `deployment[].subDeviceId` | string | N | ID of the sub-device to which this deployment is assigned in case of compound device |

### Deployment status

#### Alternative option A: no change

Since the deployment ID is unique for each deployment on a device, no change is required to the defined interface for the compound device/gateway to report the status a  deployment.

#### Alternative option B - use sub-device id in endpoint

**Endpoint**: `POST /api/v1/device/{deviceId}-{subDeviceId}/deployment/{deploymentId}/status`

While not really necessary, this option makes sense if the endpoints used for the desired state also uses the sub-device id. This allows the WFM to treat each sub-device like they are regular device and no other changes is necessary.


## Rejection reason

> If a SUP is rejected, indicate the reason why it was rejected.
> 
> Complete if SUP is rejected at Phase 2: Proposal Creation or Phase 4: Final Decision 
