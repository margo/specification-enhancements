# Specification Update Proposal: Schema-agnostic device characteristic matching

## Owner

[@phil-abb](https://github.com/phil-abb)

## Summary

As part of the desired to separate the core specification from independent deployment specifications, the PM group has indicated there is a desire to move to a schema-agnostic approach for matching applications to supported devices. This SUP proposes replacing the fixed device capability model with a schema-agnostic device profile model. A device reports an array of named characteristics, each containing an opaque property bag. Application deployment profiles refer to those characteristics by globally unique keys and use a small, standard matching language to express eligibility requirements.

The proposal also recommends an approach to use dynamic mapping for specific cases where the workload fleet manger must understand the meaning behind specific properties.

## Reason for proposal

Margo's current device capability contract hardcodes resources, peripherals, interfaces, runtimes, and deployment type information into a defined schema. The current [Device Capabilities](https://docs.margo.org/specification/margo-management-interface/device-capabilities) page and the [Application Description](https://docs.margo.org/specification/applications/application-description#deploymentprofile-attributes) page consequently require the workload fleet manager implementations to understand this schema in order to perform compatibility matching.

This conflicts with the new direction for defaulting to a schema-agnostic, passthrough by default, approach for matching applications to devices. This is a new direction the PM group has agreed to, to decouple device characteristic matching from the core specification so new/updated characteristics can be added without requiring the workload fleet manager to implement new code to support it. The driving factor for this change in direction was a proposal to separate deployment types from the core specification but the same reasoning applies to all device characteristics, not just what deployments devices support.

The proposal is associated with [Issue TBD]() and draws from ideas originally proposed in the [Device Manifest Bisection SUP](https://github.com/margo/specification-enhancements/blob/sup/bisect-device-manifest/proposals/sup_bisect_device_manifest.md). The Bisection SUP introduces some good ideas but they are tied heavily to having schemas defined that the workload fleet manager and device MUST use and understand which conflicts with this new direction. This proposal keeps the useful profile concept but removes the requirement that the workload fleet manger register, validate, or interpret every profile schema. It also simplifies how each device characteristic is represented as a opaque property bag. The ideas proposed here allow the workload fleet manger to evaluate compatibility using only the generic matching operators refined in this SUP that were originally introduced with the [Custom Runtime SUP](https://github.com/margo/specification-enhancements/blob/main/completed/sup_device_specific_runtime_affinity_matching.md#2-device-constraints-model).

This change improves the specification by:

1. Helps move in the direction of separating the stable core contracts from device-specific contracts so the core specification and deployment specifications can evolve independently from each other.
2. Allows independent deployment specifications to add characteristics without requiring workload fleet managers to change code.
3. Makes matching schema-agnostic, deterministic and compliance-testable through JSON Pointer and explicitly defined operators.
4. Suggests a dynamic discovery mechanism for properties workload fleet manager can understand if required; though recommends keeping this to an absolute minimum.

## Requirements alignment acknowledgement

This proposal is related to [Issue TBD]().

This proposal supports the existing Margo goals and surfaces for:

* [Application Description](https://docs.margo.org/specification/applications/application-description), which determines whether an application is compatible with a device.
* [Desired State](https://docs.margo.org/specification/margo-management-interface/desired-state), which carries the selected deployment profile to the device.
* [Device Capabilities](https://docs.margo.org/specification/margo-management-interface/device-capabilities), which is the current device-to-WFM capability exchange.
* [Device Requirements](https://docs.margo.org/specification/margo-devices/device-requirements), which requires workload-hosting devices to expose supported deployment and runtime capabilities.
* [Software Composition](https://docs.margo.org/personas-and-definitions/software-composition), which distinguishes application packaging from deployment and permits multiple deployment types.

It does not introduce a new workload runtime, package format, registry, scheduling policy, or observability protocol. It does not define the common characteristics but it gives examples for how such characteristics are named, transported, matched, and evolved. It does not define any deployment specifications but gives examples how for how deployment characteristics can be described. The deployment specifications remain responsible for defining the meaning and schema of its own characteristics.

This proposal defines the direction and approach for moving to a schema-agnostic, passthrough by default, approach. If this SUP is approved than deployment specifications for Helm and Compose can be started based on this approach. If this SUP is approved the common device characteristics can start to be defined as part of the common deployment specification guidelines.

## Technical proposal

### 1. Core and deployment specification boundary

The Margo core specification SHOULD contain only:

* the device identity and relationship model;
* the characteristic property-bag envelope;
* non-deployment-specific device parameters
* the profile and mapping endpoints;
* the matching grammar and evaluation semantics; and
* the vocabulary required by the workload fleet manager to perform its own management functions, if use cases agreed to.

The common deployment specification guidelines SHOULD contain only:

* requirements each deployment specification must meet;
* recommended common characteristics definitions;
* characteristic key naming guidelines;
* characteristic evolution guidelines;
* deployment specification governance model;

An independently versioned deployment specification MAY define:

* characteristic keys and their JSON Schemas;
* the semantics and units of characteristic properties;
* deployment-specific properties;
* deployment-specific device parameters;
* Application component OCI blob composition requirements;
* compatibility guidance for its runtime; and
* conformance tests for those characteristic definitions.

The core specification MUST NOT require a workload fleet manager to deserialize or semantically interpret a characteristic property bag merely to evaluate compatibility matching. The core specification MUST NOT explicitly prohibit a workload fleet manager from doing so if they choose with the understanding the workload fleet manager takes responsibility for the impact of their decision to tightly couple their implementation.

### 2. Device profile model

The current [Device Capabilities](https://docs.margo.org/specification/margo-management-interface/device-capabilities) concept is completely changed to a new device profile and characteristics model.

#### Profile endpoints

The current `/api/v1/capabilities/{deviceId}` contract is replaced by:

```text
POST   /api/v1/profile/{targetName}
PUT    /api/v1/profile/{targetName}
DELETE /api/v1/profile/{targetName}
```

`targetName` uses the existing hierarchical device identifier rules for gateways. `POST` creates a profile, `PUT` updates an existing profile characteristics, and `DELETE` removes profile characteristics. The POST and PUT request body MUST be the `deviceCharacteristics` object shown below. DELETE has no request body to removes all characteristics for the target, or a payload with the just the keys for individuals characteristics to remove. A gateway MUST submit the parent profile before a child profile, preserving the current see-through gateway ordering rule.

The endpoints inherit the Management Interface requirements for mTLS, authorization, HTTP/1.1, port 443, status codes, and [RFC 9457 Problem Details](https://docs.margo.org/specification/margo-management-interface/api-requirements-and-security#error-responses). A malformed envelope is a `400 Bad Request`; a semantically invalid characteristic payload is a `422 Unprocessable Content`.

The device profile payload is an array of device characteristics.

```json
{
    "deviceCharacteristics": 
    [
	    {
			"key": "string",
			"properties": {
                // property bag
			}
		}
    ]
}
```

Device Profile Example:

```json
{
	"deviceCharacteristics": [
		{
			"key": "margo.org/resource/cpu",
			"properties": {
				"cpus": [
					{
						"cores": 24,
						"architecture": "amd64"
					}
				]
			}
		},
		{
			"key": "margo.org/peripherals/gpu",
			"properties": {
				"gpus": [
					{
						"manufacturer": "NVIDIA",
						"model": "A100",
						"allocatable": true,
						"vram": 10,
						"architecture": "Ampere"
					}
				]
			}
		}
	]
}
```

Each device characteristic is represented by a key and an object-valued property bag:

```json
{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"$id": "https://margo.org/schemas/device-characteristic.schema.json",
	"title": "DeviceCharacteristic",
	"type": "object",
	"additionalProperties": false,
	"required": ["key", "properties"],
	"properties": {
		"key": {
			"type": "string",
			"minLength": 1,
			"pattern": "^[A-Za-z0-9][A-Za-z0-9._-]*(/[A-Za-z0-9._~-]+)+$"
		},
		"properties": {
			"type": "object",
			"additionalProperties": true
		}
	}
}
```

The following rules apply:

* `deviceCharacteristics` MUST be an array. Each key MUST occur at most once in a payload.
* A characteristic key MUST be globally unique within the Margo ecosystem. Reverse-domain names such as `margo.org/resource/cpu` or `example.com/interface/canbus` are RECOMMENDED.
* `properties` MUST be a JSON object. Its contents are opaque to the core specification and MAY contain nested objects and arrays.
* A device MAY omit a characteristic when it is unavailable. Omission is different from an empty property bag.
* A device MUST send all known characteristics to create the initial device profile using POST.
* A device MAY add or update one or more individual characteristics once the device profile has been created using PUT.
* A device MAY remove one or more individual characteristic once the device profile has been created using DELETE.
* profile describes only resources and characteristics exposed to Margo workloads, as required by the current device capability guidance.

### 3. Characteristic definitions and evolution

The supplier that owns a characteristic key MUST publish and human-readable definition for that key and MAY publish a JSON Schema. The characteristic definition SHOULD be published, or referenced, as part of the applicable deployment specification. Margo's common deployment specification guidelines documentation MAY define a set of common characteristics deployment specifications MAY references. The schema is informative to a workload fleet manger operating in passthrough mode, but is normative for a device suppliers claiming conformance to the characteristic.

Characteristics are not versioned by adding a version field to each instance. An owner MAY make only these compatible changes under the same key:

* add an optional property;
* add an enum option; or
* change a required property to optional.

The owner MUST assign a new key for any other change, including removing or renaming a property, changing a property data type, removing an enum option, or making an optional property required. The new key MAY use a new path or another collision-resistant identifier.

In order to reduce compatibility concerns, modifying a characteristic is such as way as to required a new unique key is NOT RECOMMENDED and should only be done when there are no other options available.

Characteristics SHOULD strive to keep the required properties to an absolute minimum to prevent application suppliers from having to define a lot of matching expressions that are unnecessary for all conditions.

#### Common Device Characteristic

The Margo community MAY want to define a set of common device characteristics a deployment specification can reference for common things like memory, CPU, storage, and certain peripherals or interfaces. These common characteristics should be documented as part of the common device specification guidelines documentation.

A deployment specification MAY choose to reference these common device characteristics, or define their own more applicable characteristics based on the unique qualities of each deployment specification.

Where units are important to the property, each characteristics MUST define a single unit the value is expected to be in so no unit conversion logic is required.

The following are examples for some common characteristics the Margo community MAY want to define.

1. **Nameplate characteristic**

    Example schema:

    ```json
    {
	    "$schema": "https://json-schema.org/draft/2020-12/schema",
	    "$id": "https://margo.org/schemas/nameplate.characteristic.schema.json",
	    "title": "NameplateCharacteristic",
	    "type": "object",
	    "additionalProperties": false,
	    "required": ["key", "properties"],
	    "properties": {
		    "key": { "const": "margo.org/nameplate" },
		    "properties": {
			    "type": "object",
			    "additionalProperties": false,
			    "required": ["vendor"],
			    "properties": {
				    "vendor": { "type": "string" },
				    "modelNumber": { "type": "string" },
				    "serialNumber": { "type": "string" }
			    }
		    }
	    }
    }
    ```

    Example instance:

    ```json
    {
        "key": "margo.org/nameplate",
        "properties": {
            "vendor": "NorthGate Assemblies",
            "modelNumber": "DA-2022123B",
            "serialNumber": "DA20A2349A-1",
        }
    }
    ```

2. **GPU characteristic:**

    Example schema:
    
    ```json
    {
    	"$schema": "https://json-schema.org/draft/2020-12/schema",
    	"$id": "https://margo.org/schemas/gpu.characteristic.schema.json",
    	"title": "GpuCharacteristic",
    	"type": "object",
    	"additionalProperties": false,
    	"required": ["key", "properties"],
    	"properties": {
    		"key": { "const": "margo.org/peripherals/gpu" },
    		"properties": {
    			"type": "object",
    			"additionalProperties": false,
    			"required": ["gpus"],
    			"properties": {
    				"gpus": {
    					"type": "array",
    					"minItems": 1,
    					"items": {
    						"type": "object",
    						"additionalProperties": false,
    						"required": ["manufacturer", "model", "allocatable"],
    						"properties": {
    							"manufacturer": { "type": "string" },
    							"model": { "type": "string" },
    							"allocatable": { "type": "boolean" },
    							"vram": { "type": "integer", "minimum": 0 },
    							"architecture": { "type": "string" }
    						}
    					}
    				}
    			}
    		}
    	}
    }
    ```
    
    Example instance
    ```json
    {
        "key": "margo.org/peripherals/gpu",
        "properties": {
            "gpus": [
                {
                    "manufacturer": "NVIDIA",
                    "model": "A100", 
                    "allocatable": true,
                    "vram": 10,
                    "architecture": "Ampere"
                }
            ],
        }
    }
    ```

3. **CPU characteristic:**

    Example schema:
    
    ```json
    {
    	"$schema": "https://json-schema.org/draft/2020-12/schema",
    	"$id": "https://margo.org/schemas/cpu.characteristic.schema.json",
    	"title": "CpuCharacteristic",
    	"type": "object",
    	"additionalProperties": false,
    	"required": ["key", "properties"],
    	"properties": {
    		"key": { "const": "margo.org/resource/cpu" },
    		"properties": {
    			"type": "object",
    			"additionalProperties": false,
    			"required": ["cpus"],
    			"properties": {
    				"cpus": {
    					"type": "array",
    					"minItems": 1,
    					"items": {
    						"type": "object",
    						"additionalProperties": false,
    						"required": ["cores", "architecture"],
    						"properties": {
    							"cores": { "type": "number", "exclusiveMinimum": 0 },
    							"architecture": { "enum": ["amd64", "arm64", "arm", "x86"] },
    							"model": { "type": "string" }
    						}
    					}
    				}
    			}
    		}
    	}
    }
    ```
    
    Example instance:
    
    ```json
    {
        "key": "margo.org/resource/cpu",
        "properties": {
            "cpus": [
                "cores": 24
                "architecture": "amd64",
            ]
        }
    }
    ```

4. **CAN Bus characteristic:**

    Example schema:
    
    ```json
    {
    	"$schema": "https://json-schema.org/draft/2020-12/schema",
    	"$id": "https://margo.org/schemas/canbus.characteristic.schema.json",
    	"title": "CanBusCharacteristic",
    	"type": "object",
    	"additionalProperties": false,
    	"required": ["key", "properties"],
    	"properties": {
    		"key": { "const": "margo.org/interface/canbus" },
    		"properties": {
    			"type": "object",
    			"additionalProperties": false,
    			"required": ["interfaces"],
    			"properties": {
    				"interfaces": {
    					"type": "array",
    					"minItems": 1,
    					"items": {
    						"type": "object",
    						"additionalProperties": false,
    						"required": ["channelId"],
    						"properties": {
    							"channelId": { "type": "string" },
    							"baudRate": { "type": "integer", "minimum": 1 },
    							"protocol": { "enum": ["CAN2.0A", "CAN2.0B", "CANFD"] }
    						}
    					}
    				}
    			}
    		}
    	}
    }
    ```
    
    Example instance:
    
    ```json
    {
        "key": "margo.org/interface/canbus",
        "properties": {
            "interfaces": [
                {
                    "channelId": "can0",
                    "baudRate": 500000, 
                    "protocol": "CAN2.0B" 
                }
           ] 
        }
    }
    ```

#### Deployment characteristic

Each deployment characteristic describes what is required for an application to target a device with the required profile. Each deployment characteristic can define what properties make sense for that specific deployment. There is no intention of trying to keep deployment characteristics common between deployment specifications.

> **NOTE:** The following deployment characteristic examples are entirely made up. Defining these deployment characteristics is not within the scope of this SUP and only serves as an example to demonstrate how this could be defined in the deployment specification.

1. **Helm/Kuberentes deployment characteristic**

    Example schema:

    ```json
    {
    	"$schema": "https://json-schema.org/draft/2020-12/schema",
    	"$id": "https://margo.org/schemas/helm-with-chart-api-2.characteristic.schema.json",
    	"title": "HelmWithChartApi2Characteristic",
    	"type": "object",
    	"additionalProperties": false,
    	"required": ["key", "properties"],
    	"properties": {
    		"key": { "const": "margo.org/deployment/helm-with-chart-api-2" },
    		"properties": {
    			"type": "object",
    			"additionalProperties": false,
    			"required": ["apis"],
    			"properties": {
    				"apis": { "type": "array", "minItems": 1, "items": { "type": "string" } },
    				"ingressController": { "type": "string" },
    				"distribution": { "type": "string" }
    			}
    		}
    	}
    }
    ```

    Example instance:

    ```json
    {
        "key": "margo.org/deployment/helm-with-chart-api-2",
        "properties": {
            "apis": [
                "v1",
                "apps/v1",
                "batch/v1",
                "autoscaling/v2",
                "networking.k8s.io/v1",
                "policy/v1",
                "rbac.authorization.k8s.io/v1",
                "storage.k8s.io/v1",
                "apiextensions.k8s.io/v1"
            ],
            "ingressController": "HAProxy",
            "distribution": "OpenShift"
        }
    }
    ```

2. **Compose deployment characteristic**

    Example schema:

    ```json
    {
    	"$schema": "https://json-schema.org/draft/2020-12/schema",
    	"$id": "https://margo.org/schemas/compose.characteristic.schema.json",
    	"title": "ComposeCharacteristic",
    	"type": "object",
    	"additionalProperties": false,
    	"required": ["key", "properties"],
    	"properties": {
    		"key": { "const": "margo.org/deployment/compose" },
    		"properties": {
    			"type": "object",
    			"additionalProperties": false,
    			"required": ["engine"],
    			"properties": { "engine": { "type": "string" } }
    		}
    	}
    }
    ```

    Example instance:

    ```json
    {
        "key": "margo.org/deployment/compose",
        "properties": {
            "engine": "Podman"
        }
    }
    ```

### 4. Application Description changes

The `deploymentProfiles` object was recently updated as part of the changes to support [Custom Runtimes](https://github.com/margo/specification-enhancements/blob/main/completed/sup_device_specific_runtime_affinity_matching.md). The proposal recommends the following changes to what was added to the specification for that SUP.

In each `deploymentProfiles[]` entry, replace `type` and `deviceConstraints` with `deviceCharacteristics`:

* Remove the required `type` property from the core deployment profile.
* Rename `deviceConstraints` to `deviceCharacteristics`.
* Remove `labelSelector` and `propertySelector`; both are replaced by one generic `matchExpressions` list.
* Remove `capacityRequirements`. Capacity requirements are expressed through characteristics expression matching.
* Introduce a `GtEq` and `LtEq` operator for "greater than or equal to", and "less than or equal to".
* Introduce a `property` field to indicate the name of the property to match on.

Each deployment specification defines which characteristics can be matched on. The workload fleet manager continue passing along the full deploymentProfile object to the device though the desired state.

The following is an example application description requiring multiple characteristics:

```yaml
deploymentProfiles:
  - id: com-northstarida-digitron-orchestrator-helm-a
    description: Helm deployment requiring a GPU and CAN Bus interface.
    components:
    - name: digitron-orchestrator
      properties:
        repository: oci://northstarida.azurecr.io/charts/digitron-orchestrator
        revision: 1.0.9
        wait: true
    deviceCharacteristics:
    - matchExpressions:
      - key: margo.org/deployment/helm-with-chart-api-2
        operator: Exists
      - key: margo.org/resource/cpu
        property: /cpus
        operator: ContainsAll
        itemSelector:
          matchExpressions:
          - key: /cores
            operator: GtEq
            values: [1.5]
          - key: /architecture
            operator: In
            values: [amd64]
      - key: margo.org/peripherals/gpu
        property: /gpus
        operator: ContainsAll
        itemSelector:
          matchExpressions:
          - key: /manufacturer
            operator: In
            values: [NVIDIA]
          - key: /allocatable
            operator: In
            values: [true]
      - key: margo.org/interface/canbus
        operator: Exists
```

At a minimum there MUST be one `deviceCharacteristics` indicating the deployment characteristic to match.

### 5. Matching grammar

The same [matching rules already defined](https://docs.margo.org/specification/applications/application-description#eligibility-rules) in the specification are applied with the following changes:

When `property` is omitted, the expression tests whether the characteristic identified by `key` exists. When `property` is present, it MUST be an RFC 6901 JSON Pointer evaluated relative to that characteristic's `properties` object. A missing pointer evaluates as absent.

The `Exist` operator behavior is adjusted:

* `Exists`: true when the characteristic key exists, or when the selected property exists if `property` is present.
* `DoesNotExist`: true when the characteristic key or selected property is absent.

The `property` must be set for the following operators

* `In`
* `NotIn`
* `Gt`
* `Lt`
* `ContainsAll`
* `ContainsAny`

The following operators are added:

* `GtEq` and `LtEq`: true when a selected numeric value is respectively greater than, equal to, or less than the single numeric value

### 6. Application Deployment changes

The selected `deviceCharacteristics` block MUST be copied unchanged from the Application Description into the corresponding `spec.deploymentProfile` in the `ApplicationDeployment`. The device MUST evaluate it before applying the deployment. The workload fleet manager MUST NOT rewrite property pointers, characteristic keys, operators, or values while creating Desired State.

This preserves the existing Desired State rule that an ApplicationDeployment is the self-contained deployment instruction retrieved by the client, while allowing clients to remain ignorant of deployment-specific property schemas. Unknown characteristic keys and unknown properties MUST be retained and MUST NOT cause failure solely because the client does not understand them.

### 7. Custom Runtime changes

The [Custom Runtime SUP](https://github.com/margo/specification-enhancements/blob/main/completed/sup_device_specific_runtime_affinity_matching.md) introduced several concepts for handling custom runtimes. The changes described in this SUP replaces how custom runtimes are supported. With this SUP custom runtimes are supported using the new characteristics model instead of the device capability labels added as part of that SUP.

Device vendors whishing to support custom runtimes would define their own deployment characteristics and share these characteristics with any application supplier needing to target their runtime. Device vendors may choose to create their own deployment specification for their custom runtime if they choose, but this would not be an official deployment specification in the Margo registry.

This SUP continues to make use of several concepts introduced with the custom runtime SUP to enable the schema-agnostic matching.

### 9. Vendor Extensions

With the changes to device capabilities, the former [vendor extension](https://docs.margo.org/specification/margo-management-interface/device-capabilities#specification-extensions) mechanism for device capabilities is changed to use the new characteristics model.

If there is a need for collaborations between a device and workload fleet manager supplier the suppliers can define their own set of device characteristics that are sent to the workload fleet manger.

The vendor extension mechanism defined for the [Application Description](https://docs.margo.org/specification/applications/application-description#specification-extensions) and [Desired State](https://docs.margo.org/specification/margo-management-interface/desired-state#specification-extensions) remain unchanged.

### 10. Dynamic WFM mappings

Under certain use cases it may be necessary for the workload fleet manager to understand what a characteristic property means.

The core specification MUST only define the vocabulary needed for the workload fleet manager behavior. For each vocabulary item, a device reports a mapping with the characteristic key and JSONPath expression that supplies the value.

Defining what this required vocabulary is, is not part of the SUP. The SUP only defines how to handle this in a dynamic way.

> **Note:** My opinion on this is it should be kept to an absolute minimum and only added with agreement from the PM group based on an actual real-world use case and not supposition.

The mapping is an open object. The workload fleet manager MUST treat it as a dynamic set and MUST NOT assume that a particular entry exists. A mapping value MUST contain a characteristic `key` and a `property` JSONPath expression as defined by [RFC 9535](https://www.rfc-editor.org/rfc/rfc9535). The JSONPath expression MUST be evaluated relative to the characteristic's `properties` object and MUST resolve to the value used for the mapped vocabulary. The WFM MAY use a mapping entry only when it knows the corresponding vocabulary and the resolved value has the neccesary information (e.g, type,  unit, etc.).

For example, assume Margo has agreed the workload fleet manager must be able to determine the memory and storage totals and CPU cores. The mapping would be defined as follows:

```json
{
	"memory": {
		"key": "margo.org/resource/memory",
		"property": "$.total",
        "type": "decimal",
        "unit": "Mi"
	},
	"storage": {
		"key": "margo.org/resource/storage",
		"property": "$.total",
        "type": "decimal",
        "unit": "Gi"
	},
	"cpuCores": {
		"key": "margo.org/resource/cpu",
		"property": "$.cpus[*].cores",
        "type": "integer"
	}
}
```

The mapping endpoints are:

```text
POST   /api/v1/profile/mapping/{targetName}
PUT    /api/v1/profile/mapping/{targetName}
DELETE /api/v1/profile/mapping/{targetName}
```

POST creates, PUT replaces, and DELETE removes the complete mapping for the target. Mapping updates MUST be atomic. The profile and mapping endpoints inherit the same authentication, authorization, gateway hierarchy, and error handling requirements.

## Breaking changes

This SUP introduces several breaking changes:

* The device capability document shape changes from fixed fields to an array of characteristics.
* The device capability endpoint and HTTP contract are replaced by profile endpoints.
* `type` is removed from deployment profiles.
* `deviceConstraints` is renamed and its child structure changes for the Application Description and Desired State manifests.
* Capacity, property-selector, and label-selector semantics are replaced by characteristic matching.
* Custom runtime support changes to the new model.
* Vendor extensions for reported device capabilities changes to the new model.

No new breaking change is introduced to OCI application or component distribution, Desired State artifact retrieval, deployment status reporting, mTLS, or OpenTelemetry requirements.

## Alternatives considered (optional)

### Keep the fixed capability schema

This preserves compatibility but requires the core specification and workload fleet manager code to change for every new resource, peripheral, interface, runtime, or deployment mechanism. It does not meet the extensibility goal.

### Use supplier labels only

The current `labels` and label-selector mechanism is useful for small scalar metadata, but it cannot describe structured resources such as a collection of GPUs or CAN Bus interfaces without inventing conventions that the WFM still needs to understand. Property bags support structured data while the generic matcher remains small.

### Register every profile schema with the workload fleet manager

The earlier Device Manifest Bisection SUP used registered `ProfileDefinition` documents. Registration provides stronger validation, but it couples profile publication to workload fleet manager lifecycle and prevents a true passthrough-by-default implementation. This proposal leaves schema validation to the characteristic owner and device supplier, while retaining stable published schemas for tooling and conformance.

### Encode all requirements as specification extensions

Specification extensions are intentionally supplier-specific and may be ignored by unrelated implementations. They are not suitable for a common, compliance-testable placement contract. The characteristic matcher provides a minimal interoperable core, while extensions remain available for non-placement data.

## Rejection reason

N/A. This is a draft proposal for working-group review.
