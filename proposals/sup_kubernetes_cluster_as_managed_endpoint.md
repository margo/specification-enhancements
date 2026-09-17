# Specification Update Proposal

## Owner(s)

- Josh Swanson
@jjaswanson4
Red Hat

- Tim Mirth
@mirthguitar
Red Hat

- Luca Ferrari
@lucamaf
Red Hat

## Summary

This SUP overhauls how the Margo specification describes Kubernetes clusters as managed endpoints. Today, the specification models a Kubernetes cluster as a "device" and forces device-centric vocabulary onto it: clusters must report hardware properties like `vendor`, `modelNumber`, and `serialNumber`; clusters were historically assigned roles like "Standalone Cluster" and "Cluster Leader"; and the capabilities model reports physical machine resources rather than what the Kubernetes API actually exposes. This framing conflates the physical infrastructure hosting the cluster with the logical workload endpoint the cluster provides, and it forces Margo to care about cluster architecture details (single-node vs. multi-node, leader election, worker topology) that are irrelevant to workload deployment.

This SUP proposes that Margo treat a Kubernetes cluster as a **managed endpoint** — an opaque workload target that is compliant with the Kubernetes API specification and reachable via a Workload Fleet Manager (WFM) or Device Fleet Manager (DFM). Margo should not prescribe or inspect the internal architecture of the cluster. Instead, the Margo agent running on the cluster reports:

1. **Kubernetes API surface**: the API groups, API versions, and Custom Resource Definitions (CRDs) available on the cluster
2. **Cluster capabilities**: supported deployment types, ingress classes, storage classes, runtime classes, and other workload-relevant features
3. **Available resources**: allocatable CPU, memory, GPUs, and other resources as reported by the Kubernetes API — aggregated across the cluster, not per-node

These capabilities are then exposed in the WFM or DFM as properties of the managed cluster endpoint, enabling workload placement decisions without requiring Margo to understand or standardize the cluster's internal topology.

## Reason for proposal

The current specification has several structural problems when applied to Kubernetes clusters:

### 1. Device-centric vocabulary does not fit Kubernetes clusters

The `DeviceCapabilitiesManifest` requires fields like `vendor`, `modelNumber`, and `serialNumber` [[spec: device-capabilities.md, Properties Attributes table][1]]. These are meaningful for a physical edge device but do not have natural equivalents for a Kubernetes cluster. A cluster can span hardware from multiple vendors, has no single model number, and no serial number. Forcing these fields leads to meaningless placeholder values and confuses the purpose of the capabilities report.

### 2. Cluster architecture is not Margo's concern

The historical device roles ("Standalone Cluster", "Cluster Leader") [[spec-enhancements: completed/gateway.md, roles field][2]] and the MIAF multi-holder identity model for multi-node clusters [[spec-enhancements: completed/miaf-pr3-inputs/miaf-multi-holder-identities-and-cluster-topology.md][3]] both force Margo to understand the internal topology of the cluster. This creates unnecessary coupling. Whether a cluster is single-node, multi-node, uses etcd or an external datastore, runs on bare metal or virtual machines, or implements leader election via Raft or lease-based mechanisms — none of this affects whether the cluster can accept a Helm chart deployment. Margo should be agnostic to these details.

### 3. Physical resource reporting misrepresents cluster capacity

The current capabilities model reports CPU cores, memory, and storage as flat values on a "device" [[spec: device-capabilities.md, CPU/memory/storage attributes][1]]. The spec notes that these represent "only the subset of device resources delegated exclusively to Margo for workload placement and reconciliation" [[spec: device-capabilities.md, line 5][1]], but the schema still models them as single-device quantities. For a Kubernetes cluster, the relevant resource quantities are what the Kubernetes API reports as **allocatable** across the cluster's nodes [[k8s: Node `.status.allocatable`][4]] — not the raw hardware specs of any individual machine. The current model cannot express concepts like resource requests vs. limits, namespace quotas, or the distinction between allocatable and capacity. It also cannot represent heterogeneous node pools (e.g., a cluster with both CPU-only and GPU nodes).

### 4. The Kubernetes API already provides a rich capabilities model

Kubernetes clusters natively expose exactly the information a WFM needs to make placement decisions [[k8s: API Discovery][5]]:
- **API discovery** (`/api`, `/apis`) [[k8s: API Discovery][5]]: which API groups and versions the cluster supports
- **CRDs** [[k8s: `apiextensions.k8s.io/v1`][6]]: which custom resource types are installed (e.g., Istio, Knative, GPU operators)
- **Node resources** [[k8s: Node `.status.allocatable`][4]]: allocatable CPU, memory, GPUs, and extended resources aggregated across the cluster
- **Storage classes** [[k8s: `storage.k8s.io/v1` StorageClass][7]]: what persistent storage backends are available
- **Ingress classes** [[k8s: `networking.k8s.io/v1` IngressClass][8]]: what ingress controllers are running
- **Runtime classes** [[k8s: `node.k8s.io/v1` RuntimeClass][9]]: what container runtimes are available (containerd, CRI-O, gVisor, Kata)
- **Namespace quotas** [[k8s: ResourceQuota][10]]: if the Margo agent operates within a namespace, the quota limits on that namespace

Margo should leverage this existing capability surface rather than reinventing a parallel, less expressive model.

### 5. Alignment with the broader edge and hybrid cloud ecosystem

Industry practice has converged on treating Kubernetes clusters as logical deployment targets, not as devices. Fleet management tools (Rancher, OpenShift ACM, Azure Arc, Google Anthos, Fleet) all manage clusters as first-class endpoints identified by their API server URL and credentials, not by hardware serial numbers. Aligning Margo with this model reduces friction for implementers and enables interoperability with existing tooling.

## Requirements alignment acknowledgement

This SUP addresses:
- The need to support Kubernetes clusters as managed workload targets without over-prescribing their internal architecture
- Alignment with [Replace device roles with capabilities within Margo](https://github.com/margo/specification/issues/136) — extending the roles-to-capabilities direction to its logical conclusion for Kubernetes clusters
- Alignment with the broader Margo goal of defining interoperable interfaces while avoiding unnecessary standardization of vendor-specific implementation details

The following items are out of scope for this SUP:
- Changes to how non-Kubernetes devices (compose-based standalone devices, WASM/leaf devices) report their capabilities — these continue to use the existing `DeviceCapabilitiesManifest` model
- Changes to the application packaging format (Helm charts, compose files, OCI artifacts)
- Changes to the desired state distribution protocol
- Multi-cluster federation or cross-cluster workload placement
- Kubernetes distribution certification or conformance testing (Margo relies on the CNCF conformance program for this)

### SUP dependencies

This SUP builds on and extends:
- [Replace device roles with capabilities](https://github.com/margo/specification-enhancements/pull/50) (approved) — this SUP takes the capabilities direction further for Kubernetes specifically
- [Device-specific runtime matching via device constraints](https://github.com/margo/specification-enhancements/blob/main/completed/sup_device_specific_runtime_affinity_matching.md) (approved) — the `deviceConstraints` / `eligibilityRules` model can be reused to match workloads against cluster-reported capabilities
- [Device specific parameter values](https://github.com/margo/specification-enhancements/blob/main/proposals/device-specific-parameter-values.md) (in progress) — the `installContext` / `valueFrom` model for device-supplied parameters (e.g., `margo.cluster.hostname`, `margo.cluster.storageClass`) applies directly to cluster endpoints

## Technical proposal

### Overview

This SUP introduces the following changes:

1. **New `ClusterCapabilitiesManifest` kind** alongside the existing `DeviceCapabilitiesManifest`, specifically for Kubernetes cluster endpoints
2. **Kubernetes-native capability reporting** that leverages the K8s API discovery and resource reporting mechanisms
3. **Removal of device-centric requirements** (vendor, modelNumber, serialNumber, leader/worker topology) for Kubernetes cluster endpoints
4. **Cluster-aggregated resource reporting** based on Kubernetes allocatable resources

---

### 1. ClusterCapabilitiesManifest

A new manifest kind, `ClusterCapabilitiesManifest`, is introduced for Kubernetes cluster endpoints. This manifest replaces `DeviceCapabilitiesManifest` when the managed endpoint is a Kubernetes cluster.

#### Schema Overview

```json
{
    "apiVersion": "cluster.margo.org/v1alpha1",
    "kind": "ClusterCapabilitiesManifest",
    "metadata": {
        "id": "plant-a-edge-cluster-01",
        "name": "Plant A Edge Cluster",
        "kubernetesVersion": "1.31.2",
        "platform": "k3s"
    },
    "apiSurface": {
        "apiGroups": [...],
        "customResourceDefinitions": [...]
    },
    "resources": {
        "allocatable": {
            "cpu": "48",
            "memory": "192Gi",
            "ephemeralStorage": "500Gi",
            "gpu": {
                "nvidia.com/gpu": 4
            },
            "extendedResources": {}
        },
        "nodeCount": 3,
        "nodeSummary": [...]
    },
    "capabilities": {
        "supportedDeploymentTypes": ["helm"],
        "storageClasses": [...],
        "ingressClasses": [...],
        "runtimeClasses": [...],
        "otelCollector": true
    },
    "installContext": {},
    "labels": {}
}
```

#### Request Body Attributes (Top-Level)

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| apiVersion | string | Y | MUST be `"cluster.margo.org/v1alpha1"`. Identifies the schema version for this manifest kind. |
| kind | string | Y | MUST be `"ClusterCapabilitiesManifest"`. Distinguishes this from a `DeviceCapabilitiesManifest`. |
| metadata | Metadata | Y | Identifies the cluster. See [Metadata Attributes](#metadata-attributes). |
| apiSurface | ApiSurface | Y | Kubernetes API groups and CRDs available on the cluster. See [API Surface Attributes](#api-surface-attributes). |
| resources | Resources | Y | Aggregate allocatable resources across the cluster. See [Resource Attributes](#resource-attributes). |
| capabilities | Capabilities | Y | Workload-relevant cluster features. See [Capabilities Attributes](#capabilities-attributes). |
| installContext | map[string]InstallContextEntry | N | Device-supplied parameter values for application installation. Same schema as defined in the Device Specific Parameter Values SUP [12]. See [installContext and labels](#installcontext-and-labels). |
| labels | map[string]interface{} | N | Supplier-defined key/value metadata for placement matching. Same schema as defined in the Device-Specific Runtime Matching SUP [13]. See [installContext and labels](#installcontext-and-labels). |
| x-&lt;unique-name&gt;-extensions | map[string]interface{} | N | Vendor-specific extensions, following the same conventions as `DeviceCapabilitiesManifest` specification extensions [[spec: device-capabilities.md, Specification Extensions][1]]. |

---

#### Metadata Attributes

The `metadata` object identifies the cluster endpoint. It replaces the device-centric identity fields (`vendor`, `modelNumber`, `serialNumber`) with cluster-appropriate identification.

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| id | string | Y | Unique identifier for this cluster endpoint. Assigned by the cluster owner or operator. MUST include only unreserved characters as specified in [RFC 3986](https://www.rfc-editor.org/rfc/rfc3986#section-2.3) plus the path separator (`/`), consistent with the `deviceId` format in the existing spec [[spec: device-capabilities.md, Properties Attributes][1]]. For a cluster behind a gateway, the id takes the hierarchical form `{gatewayId}/{clusterId}`. |
| name | string | N | Human-readable display name for the cluster (e.g., `"Plant A Edge Cluster 01"`). |
| kubernetesVersion | string | Y | The Kubernetes server version string, as reported by the API server's `/version` endpoint (e.g., `"1.31.2"`, `"1.30.4+k3s1"`). The agent SHOULD report the `gitVersion` field from the `/version` response. |
| platform | string | N | The Kubernetes distribution or platform running the cluster. Informational only — Margo MUST NOT change behavior based on this field. WFM implementations MUST NOT use this field for workload placement decisions; use `labels` or `apiSurface` instead. |

##### Platform Enumeration (Informational)

The following values are RECOMMENDED for commonly encountered distributions but this list is not exhaustive. Implementations MAY report any string value.

| Value | Description |
|-------|-------------|
| k3s | Rancher K3s lightweight Kubernetes |
| k0s | Mirantis k0s zero-friction Kubernetes |
| rke2 | Rancher RKE2 (RKE Government) |
| openshift | Red Hat OpenShift |
| microshift | Red Hat MicroShift |
| eks | Amazon Elastic Kubernetes Service |
| aks | Azure Kubernetes Service |
| gke | Google Kubernetes Engine |
| vanilla | Upstream Kubernetes (kubeadm or equivalent) |

> **Note:** The `DeviceCapabilitiesManifest` fields `vendor`, `modelNumber`, and `serialNumber` are intentionally omitted from `ClusterCapabilitiesManifest`. These fields describe physical hardware identity, which does not apply to a logical cluster endpoint that may span multiple vendors and hardware platforms.

---

#### API Surface Attributes

The `apiSurface` object reports the Kubernetes API groups and Custom Resource Definitions available on the cluster. The Margo agent obtains this information from the Kubernetes API discovery endpoints [[k8s: API Discovery][5]].

This section addresses a gap identified in the current spec's device requirements: the `.Capabilities.APIVersions.Has` Helm function requires the list of Kubernetes APIs to be provided out-of-band when not using `helm install` directly [[spec: device-requirements.md, lines 32-36][11]]. With `apiSurface`, the cluster agent reports this information natively as part of capabilities, eliminating the need for out-of-band API list provisioning.

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| apiGroups | []ApiGroup | Y | API groups available on the cluster. The agent MUST populate this from the Kubernetes API discovery endpoint (`/apis`). The core API group (`""`) SHOULD be included, obtained from `/api`. |
| customResourceDefinitions | []CustomResourceDefinition | N | CRDs installed on the cluster. The agent SHOULD populate this by listing `apiextensions.k8s.io/v1` CustomResourceDefinition resources. MAY be omitted if the agent lacks RBAC permission to list CRDs. |

##### ApiGroup Attributes

Each entry in the `apiGroups` array describes one Kubernetes API group.

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| name | string | Y | The API group name as returned by `/apis` (e.g., `"apps"`, `"networking.k8s.io"`, `"gateway.networking.k8s.io"`). For the core API group, this MUST be the empty string `""`. |
| versions | []string | Y | List of available versions within this API group (e.g., `["v1"]`, `["v1", "v1beta1"]`). The agent SHOULD list the preferred version first. MUST contain at least one entry. |

##### CustomResourceDefinition Attributes

Each entry in the `customResourceDefinitions` array describes one CRD installed on the cluster.

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| group | string | Y | The API group the CRD belongs to (e.g., `"gpu-operator.nvidia.com"`, `"longhorn.io"`). |
| kind | string | Y | The resource kind defined by the CRD (e.g., `"ClusterPolicy"`, `"Volume"`). |
| versions | []string | Y | Versions served by this CRD (e.g., `["v1"]`, `["v1beta2", "v1beta1"]`). The agent SHOULD list versions in the order returned by the CRD's `.spec.versions` (served versions only). MUST contain at least one entry. |

##### API Surface Example

```json
{
    "apiGroups": [
        {"name": "", "versions": ["v1"]},
        {"name": "apps", "versions": ["v1"]},
        {"name": "batch", "versions": ["v1"]},
        {"name": "networking.k8s.io", "versions": ["v1"]},
        {"name": "storage.k8s.io", "versions": ["v1"]},
        {"name": "node.k8s.io", "versions": ["v1"]},
        {"name": "gateway.networking.k8s.io", "versions": ["v1", "v1beta1"]}
    ],
    "customResourceDefinitions": [
        {"group": "gpu-operator.nvidia.com", "kind": "ClusterPolicy", "versions": ["v1"]},
        {"group": "nfd.kubernetes.io", "kind": "NodeFeatureDiscovery", "versions": ["v1alpha1"]},
        {"group": "longhorn.io", "kind": "Volume", "versions": ["v1beta2"]}
    ]
}
```

This enables the WFM to determine whether a cluster has the API resources required by a given workload — for example, whether the cluster supports Gateway API, has a GPU operator installed, or has a particular CRD that a Helm chart depends on. Combined with `deviceConstraints` eligibility rules [13], application suppliers can express requirements like "this chart requires the NVIDIA GPU Operator CRD" without Margo needing to standardize GPU operator support.

---

#### Resource Attributes

The `resources` object reports the aggregate allocatable resources across the cluster, as derived from the Kubernetes Node API [[k8s: Node `.status.allocatable`][4]]. The Margo agent computes aggregate values by summing the `.status.allocatable` fields from all nodes in `Ready` condition.

These values represent what the cluster makes available for scheduling workloads. They intentionally do not reflect raw hardware capacity — the Kubernetes `allocatable` quantity already accounts for resources reserved by the kubelet, OS, and system daemons.

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| allocatable | Allocatable | Y | Aggregate allocatable resources across all Ready nodes. See [Allocatable Attributes](#allocatable-attributes). |
| nodeCount | integer | Y | Number of nodes in `Ready` condition. The agent MUST count only nodes with `condition.type == "Ready"` and `condition.status == "True"`. |
| nodeSummary | []NodeSummary | N | Per-node resource breakdown. See [NodeSummary Attributes](#nodesummary-attributes). RECOMMENDED for heterogeneous clusters (mixed architectures, GPU nodes alongside CPU-only nodes). MAY be omitted for homogeneous single-node clusters. |

##### Allocatable Attributes

The `allocatable` object contains the summed allocatable resources from all `Ready` nodes.

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| cpu | string | Y | Total allocatable CPU across all Ready nodes. MUST use Kubernetes quantity format: either whole cores (`"48"`) or millicores (`"48000m"`). Computed by summing each node's `.status.allocatable.cpu`. |
| memory | string | Y | Total allocatable memory across all Ready nodes. MUST use Kubernetes binary quantity format with a binary unit suffix (`"192Gi"`, `"196608Mi"`). Computed by summing each node's `.status.allocatable.memory`. |
| ephemeralStorage | string | N | Total allocatable ephemeral storage across all Ready nodes. Uses the same binary quantity format as `memory`. Computed by summing each node's `.status.allocatable.ephemeral-storage`. MAY be omitted if ephemeral storage is not relevant or not reported by the cluster's nodes. |
| gpu | map[string]integer | N | Map of GPU extended resource names to their total allocatable count across all Ready nodes. Keys MUST use the Kubernetes extended resource naming convention (e.g., `"nvidia.com/gpu"`, `"amd.com/gpu"`, `"intel.com/gpu"`). Values are integer counts. The agent populates this by summing each node's `.status.allocatable` for resource names matching known GPU resource patterns. |
| extendedResources | map[string]integer | N | Map of any other Kubernetes extended resource names to their total allocatable count. This covers resources beyond GPUs that device plugins or node drivers expose (e.g., `"smarter-devices/ttyUSB0"`, `"example.com/fpga"`). Keys MUST follow the Kubernetes extended resource naming convention (`<domain>/<resource>`). |

> **Note on Kubernetes quantity format:** Kubernetes quantities can be expressed as plain integers (`"48"`), decimal values (`"0.5"`), milliunits with an `m` suffix (`"500m"`), or with binary suffixes (`Ki`, `Mi`, `Gi`, `Ti`, `Pi`, `Ei`). For CPU, `"1"` equals `"1000m"`. For memory and storage, binary suffixes are standard (e.g., `1Gi` = 1,073,741,824 bytes). The agent SHOULD report values in the same format as the Kubernetes API returns them.

##### NodeSummary Attributes

Each entry in the `nodeSummary` array provides a per-node resource breakdown. This enables the WFM to understand the resource distribution within a heterogeneous cluster without exposing node-internal details like hostnames, IP addresses, or kubelet configuration.

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| architecture | string | Y | CPU architecture of the node, matching the Kubernetes `kubernetes.io/arch` label value. Uses the same enumeration as the existing spec's `CpuArchitectureType` [[spec: device-capabilities.md, CpuArchitectureType][1]]: `"amd64"`, `"arm64"`, `"arm"`. |
| allocatableCpu | string | Y | Allocatable CPU on this node, in Kubernetes quantity format. |
| allocatableMemory | string | Y | Allocatable memory on this node, in Kubernetes binary quantity format. |
| allocatableGpu | map[string]integer | N | Map of GPU resource names to allocatable count on this specific node. Same format as the top-level `allocatable.gpu` field. |
| allocatableExtendedResources | map[string]integer | N | Map of other extended resource names to allocatable count on this specific node. |

> **Note:** `nodeSummary` intentionally does NOT include node names, IP addresses, taints, labels, or condition details. These are cluster-internal scheduling concerns that Margo does not need. The purpose of `nodeSummary` is to allow the WFM to assess whether a workload's resource requirements can be satisfied by a single node in the cluster — particularly important for workloads that cannot be spread across nodes (e.g., a GPU workload that needs 4 GPUs on a single node).

##### Resource Example

```json
{
    "allocatable": {
        "cpu": "48",
        "memory": "192Gi",
        "ephemeralStorage": "500Gi",
        "gpu": {
            "nvidia.com/gpu": 4
        }
    },
    "nodeCount": 3,
    "nodeSummary": [
        {
            "architecture": "amd64",
            "allocatableCpu": "16",
            "allocatableMemory": "64Gi"
        },
        {
            "architecture": "amd64",
            "allocatableCpu": "16",
            "allocatableMemory": "64Gi"
        },
        {
            "architecture": "amd64",
            "allocatableCpu": "16",
            "allocatableMemory": "64Gi",
            "allocatableGpu": {"nvidia.com/gpu": 4}
        }
    ]
}
```

In this example, the cluster has 48 total allocatable CPU cores across 3 nodes. Only one node has GPUs. A workload requiring 4 GPUs on a single node can be placed; a workload requiring 8 GPUs cannot, despite the cluster having "enough" aggregate resources. The `nodeSummary` makes this assessment possible without requiring the WFM to query the Kubernetes API directly.

---

#### Capabilities Attributes

The `capabilities` object reports workload-relevant cluster features that the Margo agent discovers from Kubernetes API objects. These are cluster-level features — not per-node — and represent what workloads can depend on when deployed to the cluster.

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| supportedDeploymentTypes | []string | Y | Deployment profile types the cluster can receive and process. For Kubernetes clusters this MUST include `"helm"`. Uses the same `SupportedDeploymentType` enumeration as the existing spec [[spec: device-capabilities.md, SupportedDeploymentType][1]]. MUST contain at least one entry. |
| storageClasses | []StorageClass | N | StorageClasses available on the cluster. See [StorageClass Attributes](#storageclass-attributes). The agent populates this by listing `storage.k8s.io/v1` StorageClass resources. |
| ingressClasses | []IngressClass | N | IngressClasses available on the cluster. See [IngressClass Attributes](#ingressclass-attributes). The agent populates this by listing `networking.k8s.io/v1` IngressClass resources. |
| runtimeClasses | []RuntimeClass | N | RuntimeClasses available on the cluster. See [RuntimeClass Attributes](#runtimeclass-attributes). The agent populates this by listing `node.k8s.io/v1` RuntimeClass resources. |
| otelCollector | boolean | Y | Whether an OpenTelemetry (OTEL) collector is deployed and operational on the cluster. Consistent with the existing spec requirement that workload-hosting devices report OTEL collector presence [[spec: device-capabilities.md, otelCollector field][1]]. |

##### StorageClass Attributes

Each entry in the `storageClasses` array describes one Kubernetes StorageClass [[k8s: StorageClass][7]].

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| name | string | Y | The StorageClass name (`.metadata.name`), e.g., `"local-path"`, `"longhorn"`, `"gp3"`. |
| provisioner | string | Y | The volume plugin or CSI driver that provisions volumes for this class (`.provisioner`), e.g., `"rancher.io/local-path"`, `"driver.longhorn.io"`, `"ebs.csi.aws.com"`. |
| isDefault | boolean | Y | Whether this StorageClass is the cluster default. `true` if the annotation `storageclass.kubernetes.io/is-default-class` is `"true"`. At most one StorageClass SHOULD be marked default. |

##### IngressClass Attributes

Each entry in the `ingressClasses` array describes one Kubernetes IngressClass [[k8s: IngressClass][8]].

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| name | string | Y | The IngressClass name (`.metadata.name`), e.g., `"nginx"`, `"traefik"`, `"haproxy"`. |
| controller | string | Y | The ingress controller implementation (`.spec.controller`), e.g., `"k8s.io/ingress-nginx"`, `"traefik.io/ingress-controller"`. |
| isDefault | boolean | N | Whether this IngressClass is the cluster default. `true` if the annotation `ingressclass.kubernetes.io/is-default-class` is `"true"`. |

##### RuntimeClass Attributes

Each entry in the `runtimeClasses` array describes one Kubernetes RuntimeClass [[k8s: RuntimeClass][9]].

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| name | string | Y | The RuntimeClass name (`.metadata.name`), e.g., `"crun"`, `"gvisor"`, `"kata"`. |
| handler | string | Y | The CRI handler that services this runtime (`.handler`), e.g., `"crun"`, `"runsc"`, `"kata-runtime"`. This corresponds to the handler name configured in the container runtime (containerd, CRI-O) on the cluster's nodes. |

##### Capabilities Example

```json
{
    "supportedDeploymentTypes": ["helm"],
    "storageClasses": [
        {"name": "local-path", "provisioner": "rancher.io/local-path", "isDefault": true},
        {"name": "longhorn", "provisioner": "driver.longhorn.io", "isDefault": false}
    ],
    "ingressClasses": [
        {"name": "nginx", "controller": "k8s.io/ingress-nginx", "isDefault": true}
    ],
    "runtimeClasses": [
        {"name": "crun", "handler": "crun"}
    ],
    "otelCollector": true
}
```

---

#### installContext and labels

The `installContext` and `labels` fields follow the same schema and semantics as defined in the existing Device specific parameter values SUP [[spec-enhancements: proposals/device-specific-parameter-values.md][12]] and Device-specific runtime matching SUP [[spec-enhancements: completed/sup_device_specific_runtime_affinity_matching.md][13]], respectively.

##### installContext

`installContext` is a map of key/value entries that supply parameter values to applications at install time. Each entry provides a value the Margo agent resolves from the cluster's own configuration, eliminating the need for manual user input.

`installContext` is particularly well-suited to Kubernetes clusters, where device-supplied values like `margo.cluster.hostname`, `margo.cluster.storageClass`, and `margo.cluster.ingressClass` [[spec-enhancements: proposals/device-specific-parameter-values.md, Margo-reserved Keys][12]] can be resolved directly from the cluster's own API objects. For example:
- `margo.cluster.storageClass` can be resolved by querying for the StorageClass with the `is-default-class` annotation
- `margo.cluster.ingressClass` can be resolved by querying for the default IngressClass
- `margo.cluster.hostname` can be resolved from cluster configuration or an Ingress resource's host field

Each `installContext` entry follows the schema defined in [12]:

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| type | string | Y | The data type of the value (e.g., `"string"`, `"number"`, `"boolean"`). |
| value | any | Y | The resolved value for this parameter. |
| description | string | N | Human-readable description of what this parameter represents. |
| mutable | boolean | N | Whether the value may change over time. If `true`, the agent MUST update the capabilities manifest when the value changes. |

##### labels

`labels` are supplier-defined key/value metadata for placement matching via `deviceConstraints` eligibility rules [[spec: device-capabilities.md, labels field][1]] [[spec-enhancements: completed/sup_device_specific_runtime_affinity_matching.md, §1][13]]. Label values MUST be a string, number, boolean, an array of strings, or an array of numbers. Label keys are case-sensitive. Implementations SHOULD use stable, collision-resistant label names; prefixing with an organization domain is RECOMMENDED (e.g., `example.com/environment`).

---

### 2. Margo Agent Behavior on Kubernetes Clusters

The Margo agent (WFM Client) running on a Kubernetes cluster MUST:

1. **Report cluster capabilities** by constructing and submitting a `ClusterCapabilitiesManifest` to the WFM via the capabilities API endpoint.
2. **Derive capabilities from the Kubernetes API**, not from hardcoded configuration. The agent queries:
   - `/version` for the Kubernetes version
   - `/apis` for available API groups
   - The CRD API (`apiextensions.k8s.io/v1`) for installed CRDs
   - The Node API for allocatable resources on Ready nodes
   - The StorageClass, IngressClass, and RuntimeClass APIs for available cluster features
3. **Periodically refresh** the capabilities manifest. If any reported capability changes (e.g., a node is added, a CRD is installed, a StorageClass is created), the agent MUST submit an updated manifest.
4. **Not report cluster-internal topology**. The agent MUST NOT report the number of control plane nodes, etcd topology, leader election state, or any other cluster-internal architectural detail. The `nodeCount` and `nodeSummary` fields are intentionally limited to workload-placement-relevant information (aggregate resources, architecture).

The Margo agent MUST NOT require cluster-admin privileges. It SHOULD operate with the minimum RBAC permissions necessary to:
- Read API discovery information (typically available to all authenticated users)
- List nodes and read node status
- List StorageClasses, IngressClasses, and RuntimeClasses
- List CRDs
- Manage workloads within its designated namespace(s)

---

### 3. API Endpoint Compatibility

The `ClusterCapabilitiesManifest` is submitted to the WFM using the same capabilities endpoint as `DeviceCapabilitiesManifest`:

```
PUT /api/v1/capabilities/{deviceId}
DELETE /api/v1/capabilities/{deviceId}
```

The `{deviceId}` parameter follows the same format and rules as defined in the current spec. The WFM distinguishes between device and cluster endpoints by inspecting the request body — the presence of `ClusterCapabilitiesManifest`-specific fields (`apiSurface`, `metadata.kubernetesVersion`) vs. `DeviceCapabilitiesManifest` fields (`properties.vendor`, `properties.modelNumber`).

The desired state retrieval (`GET /api/v1/devices/{deviceId}/deployments`) and deployment status APIs remain unchanged. A Kubernetes cluster endpoint receives `ApplicationDeployment` documents with `type: helm` deployment profiles, exactly as it does today.

---

### 4. Integration with deviceConstraints

The existing `deviceConstraints` model from the device-specific runtime matching SUP [[spec-enhancements: completed/sup_device_specific_runtime_affinity_matching.md, §2-3][13]] can be used to target Kubernetes cluster endpoints. The `propertySelector` can reference fields within the `ClusterCapabilitiesManifest` using JSON Pointer paths (per [RFC 6901](https://datatracker.ietf.org/doc/html/rfc6901)), and the `labelSelector` works against `labels` as before.

Examples of cluster-aware constraints:

```yaml
deviceConstraints:
  capacityRequirements:
    cpu:
      cores: 4
    memory: 8Gi
    storage: 50Gi
  eligibilityRules:
    - propertySelector:
        matchExpressions:
          - key: /capabilities/storageClasses
            operator: ContainsAll
            itemSelector:
              matchExpressions:
                - key: /provisioner
                  operator: In
                  values: ["driver.longhorn.io"]
          - key: /apiSurface/customResourceDefinitions
            operator: ContainsAll
            itemSelector:
              matchExpressions:
                - key: /group
                  operator: In
                  values: ["gpu-operator.nvidia.com"]
```

This constraint would match only cluster endpoints that have Longhorn storage and the NVIDIA GPU Operator CRD installed — without the WFM needing to know anything about the cluster's internal node topology.

---

### 5. Specification Text Changes

The following sections of the Margo specification require updates:

1. **`margo-devices/device-requirements.md`**: The current "Helm-enabled Device Requirements" subsection describes clusters as devices that happen to have Kubernetes. This section should be refactored to distinguish between:
   - **Helm-enabled devices**: physical devices running Kubernetes that report via `DeviceCapabilitiesManifest` (preserved for backwards compatibility)
   - **Kubernetes cluster endpoints**: clusters managed as logical endpoints that report via `ClusterCapabilitiesManifest` (new)
   
   The existing note about `.Capabilities.APIVersions.Has` and providing the Kubernetes API list out-of-band [[spec: device-requirements.md, lines 32-36][11]] is directly addressed by this SUP's `apiSurface` field — the cluster agent reports this natively.

2. **`margo-management-interface/device-capabilities.md`**: Document that the capabilities endpoint (`PUT /api/v1/capabilities/{deviceId}`) accepts both request body schemas. Add a section for `ClusterCapabilitiesManifest` alongside the existing device capabilities documentation. The gateway considerations (opaque and see-thru) apply equally to cluster endpoints behind gateways.

3. **`observability/collecting-workload-observability-data.md`**: This section already correctly refers to "Cluster" observability and the Kubernetes Cluster Receiver [[spec: observability/collecting-workload-observability-data.md, lines 35-41][14]]. No changes needed beyond ensuring the terminology is consistent with the new "cluster endpoint" concept.

4. **Terminology**: Introduce the following terms in the spec's glossary or index:
   - **Managed Endpoint**: A workload target managed by a WFM or DFM. May be either a device (reporting via `DeviceCapabilitiesManifest`) or a Kubernetes cluster (reporting via `ClusterCapabilitiesManifest`).
   - **Cluster Endpoint**: A Kubernetes cluster registered with a WFM or DFM, identified by its cluster ID and API surface rather than by hardware properties.

---

## Example ClusterCapabilitiesManifest Payload

```json
{
    "apiVersion": "cluster.margo.org/v1alpha1",
    "kind": "ClusterCapabilitiesManifest",
    "metadata": {
        "id": "plant-a-edge-cluster-01",
        "name": "Plant A Edge Cluster 01",
        "kubernetesVersion": "1.31.2",
        "platform": "k3s"
    },
    "apiSurface": {
        "apiGroups": [
            {"name": "", "versions": ["v1"]},
            {"name": "apps", "versions": ["v1"]},
            {"name": "batch", "versions": ["v1"]},
            {"name": "networking.k8s.io", "versions": ["v1"]},
            {"name": "storage.k8s.io", "versions": ["v1"]},
            {"name": "node.k8s.io", "versions": ["v1"]},
            {"name": "gateway.networking.k8s.io", "versions": ["v1", "v1beta1"]},
            {"name": "helm.cattle.io", "versions": ["v1"]}
        ],
        "customResourceDefinitions": [
            {"group": "gpu-operator.nvidia.com", "kind": "ClusterPolicy", "versions": ["v1"]},
            {"group": "nfd.k8s-sigs.io", "kind": "NodeFeatureDiscovery", "versions": ["v1alpha1"]},
            {"group": "longhorn.io", "kind": "Volume", "versions": ["v1beta2"]}
        ]
    },
    "resources": {
        "allocatable": {
            "cpu": "48",
            "memory": "192Gi",
            "ephemeralStorage": "1500Gi",
            "gpu": {
                "nvidia.com/gpu": 4
            }
        },
        "nodeCount": 3,
        "nodeSummary": [
            {
                "architecture": "amd64",
                "allocatableCpu": "16",
                "allocatableMemory": "64Gi"
            },
            {
                "architecture": "amd64",
                "allocatableCpu": "16",
                "allocatableMemory": "64Gi"
            },
            {
                "architecture": "amd64",
                "allocatableCpu": "16",
                "allocatableMemory": "64Gi",
                "allocatableGpu": {"nvidia.com/gpu": 4}
            }
        ]
    },
    "capabilities": {
        "supportedDeploymentTypes": ["helm"],
        "storageClasses": [
            {"name": "local-path", "provisioner": "rancher.io/local-path", "isDefault": true},
            {"name": "longhorn", "provisioner": "driver.longhorn.io", "isDefault": false}
        ],
        "ingressClasses": [
            {"name": "nginx", "controller": "k8s.io/ingress-nginx"}
        ],
        "runtimeClasses": [
            {"name": "crun", "handler": "crun"}
        ],
        "otelCollector": true
    },
    "installContext": {
        "margo.cluster.hostname": {
            "type": "string",
            "value": "edge01.plant-a.example.com",
            "description": "Cluster FQDN for ingress and certificate DNS names.",
            "mutable": true
        },
        "margo.cluster.storageClass": {
            "type": "string",
            "value": "longhorn",
            "description": "Preferred StorageClass for persistent workloads.",
            "mutable": true
        },
        "margo.cluster.ingressClass": {
            "type": "string",
            "value": "nginx",
            "description": "Default IngressClass for HTTP/HTTPS workloads.",
            "mutable": true
        }
    },
    "labels": {
        "example.com/environment": "production",
        "example.com/region": "us-east-plant-a"
    }
}
```

## Breaking Changes

1. **New manifest kind**: WFM implementations must be updated to accept and process `ClusterCapabilitiesManifest` in addition to `DeviceCapabilitiesManifest`. Existing device endpoints are not affected.

2. **Removal of cluster-as-device pattern**: WFM Client implementations that currently report Kubernetes clusters using `DeviceCapabilitiesManifest` with `supportedDeploymentTypes: ["helm"]` will need to migrate to `ClusterCapabilitiesManifest`. A transition period should be defined where WFMs accept both patterns.

3. **No changes to deployment artifacts**: `ApplicationDeployment` documents with `type: helm` deployment profiles are unchanged. The breaking change is limited to capabilities reporting.

## Alternatives considered

### Alternative 1: Extend DeviceCapabilitiesManifest with optional Kubernetes fields

Rather than introducing a new manifest kind, add optional Kubernetes-specific fields (apiSurface, storageClasses, etc.) to the existing `DeviceCapabilitiesManifest`.

**Rejected because:** This perpetuates the conceptual mismatch of calling a cluster a "device" and would require making most existing required fields (vendor, modelNumber, serialNumber) optional, undermining the schema's clarity. A separate kind cleanly separates the two endpoint types.

### Alternative 2: Use Kubernetes labels and annotations directly as the capabilities model

Rather than defining a Margo-specific capabilities manifest, have the agent forward raw Kubernetes node labels, annotations, and API discovery responses to the WFM.

**Rejected because:** This would leak Kubernetes-internal implementation details into the Margo API surface, creating tight coupling to Kubernetes API schemas. The Margo capabilities model should provide a stable, well-defined interface that the WFM can depend on, insulating it from Kubernetes API version churn.

### Alternative 3: Model each Kubernetes node as a separate Margo device

Report each node in the cluster as its own `DeviceCapabilitiesManifest`, similar to how a see-thru gateway reports sub-devices.

**Rejected because:** This fundamentally misunderstands how Kubernetes scheduling works. Workloads are submitted to the cluster's API server, not to individual nodes. The Kubernetes scheduler handles node placement. Margo should not duplicate or compete with the Kubernetes scheduler's responsibility.

## References

### Margo Specification (current)

| Ref | Document | Description |
|-----|----------|-------------|
| [1] | [`spec: margo-management-interface/device-capabilities.md`](https://github.com/margo/specification/blob/main/system-design/specification/margo-management-interface/device-capabilities.md) | Device Capabilities API — Properties Attributes table (vendor, modelNumber, serialNumber as required fields), CPU/memory/storage attributes, labels field, gateway considerations. Route: `PUT /api/v1/capabilities/{deviceId}`. |
| [11] | [`spec: margo-devices/device-requirements.md`](https://github.com/margo/specification/blob/main/system-design/specification/margo-devices/device-requirements.md) | Device Requirements — defines "Helm-enabled Device Requirements" (lines 19-37), including the probationary `.Capabilities.APIVersions.Has` note about providing the K8s API list out-of-band. |
| [14] | [`spec: observability/collecting-workload-observability-data.md`](https://github.com/margo/specification/blob/main/system-design/specification/observability/collecting-workload-observability-data.md) | Observability — already uses "Cluster" terminology correctly (lines 35-41), references Kubernetes Cluster Receiver. |

### Margo Specification Enhancements (completed & in-progress SUPs)

| Ref | Document | Description |
|-----|----------|-------------|
| [2] | [`spec-enhancements: completed/gateway.md`](https://github.com/margo/specification-enhancements/blob/main/completed/gateway.md) | Gateway SUP (approved) — defines opaque/see-thru gateway patterns, historical `roles` field with values including "Standalone Cluster", "Cluster Leader", "Gateway". |
| [3] | [`spec-enhancements: completed/miaf-pr3-inputs/miaf-multi-holder-identities-and-cluster-topology.md`](https://github.com/margo/specification-enhancements/blob/main/completed/miaf-pr3-inputs/miaf-multi-holder-identities-and-cluster-topology.md) | MIAF Multi-Holder Identities — discusses cluster topology, leader failover, and multi-node cluster identity challenges under the device-as-foundation model. |
| [12] | [`spec-enhancements: proposals/device-specific-parameter-values.md`](https://github.com/margo/specification-enhancements/blob/main/proposals/device-specific-parameter-values.md) | Device Specific Parameter Values SUP (P3) — defines `installContext` in DeviceCapabilitiesManifest and `valueFrom.device` in ApplicationDescription. Margo-reserved keys: `margo.cluster.hostname`, `margo.cluster.storageClass`, `margo.cluster.ingressClass`, `margo.device.dataPath`. |
| [13] | [`spec-enhancements: completed/sup_device_specific_runtime_affinity_matching.md`](https://github.com/margo/specification-enhancements/blob/main/completed/sup_device_specific_runtime_affinity_matching.md) | Device-Specific Runtime Matching SUP (approved) — defines `labels` on DeviceCapabilitiesManifest, `deviceConstraints` with `capacityRequirements` and `eligibilityRules` (propertySelector/labelSelector with matchExpressions), and ContainsAll/ContainsAny operators. |
| [15] | [`spec-enhancements: completed/sup_device_roles_to_capabilities.md`](https://github.com/margo/specification-enhancements/blob/main/completed/sup_device_roles_to_capabilities.md) | Roles to Capabilities SUP (approved) — removed `roles` field, promoted resources/capabilities to flat `properties` fields, introduced `supportedRuntimes` and `supportedDeploymentTypes`. This SUP extends that direction further for Kubernetes. |
| [16] | [`spec-enhancements: completed/sup_desired_state_via_manifest_api.md`](https://github.com/margo/specification-enhancements/blob/main/completed/sup_desired_state_via_manifest_api.md) | Desired State via Manifest API SUP (approved) — defines pull-based state distribution protocol using `GET /api/v1/devices/{deviceId}/deployments`. This SUP does not change these endpoints. |

### Kubernetes API References

| Ref | Resource | Description |
|-----|----------|-------------|
| [4] | [Node `.status.allocatable`](https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/node-v1/#NodeStatus) | Per-node allocatable resources (CPU, memory, ephemeral-storage, extended resources). The Margo agent sums these across Ready nodes. |
| [5] | [API Discovery](https://kubernetes.io/docs/reference/using-api/api-concepts/#api-discovery) | `/api` and `/apis` endpoints — enumerate available API groups and versions on the cluster. |
| [6] | [CustomResourceDefinition](https://kubernetes.io/docs/reference/kubernetes-api/extend-resources/custom-resource-definition-v1/) | `apiextensions.k8s.io/v1` — lists all CRDs installed on the cluster. |
| [7] | [StorageClass](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/storage-class-v1/) | `storage.k8s.io/v1` — available storage backends, provisioners, and default class annotation. |
| [8] | [IngressClass](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/ingress-class-v1/) | `networking.k8s.io/v1` — available ingress controllers. |
| [9] | [RuntimeClass](https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/runtime-class-v1/) | `node.k8s.io/v1` — available container runtime handlers (e.g., runc, crun, gVisor, Kata). |
| [10] | [ResourceQuota](https://kubernetes.io/docs/reference/kubernetes-api/policy-resources/resource-quota-v1/) | Namespace-scoped resource limits — relevant when the Margo agent operates within a namespace. |

### Note on `DeviceCapabilitiesManifest` terminology

The `kind: "DeviceCapabilitiesManifest"` and `apiVersion: "device.margo.org/v1alpha1"` fields referenced throughout this SUP were introduced by the Roles to Capabilities SUP [15], which is approved but not yet merged into the published specification schema. The published spec's device capabilities request body does not currently include `kind` or `apiVersion` as formal JSON fields — the term "DeviceCapabilitiesManifest" appears only in prose (e.g., the gateways section of device-capabilities.md [1]). Since this SUP explicitly depends on [15], the `ClusterCapabilitiesManifest` kind proposed here follows the same pattern and assumes [15] is integrated into the spec.

## Rejection reason
