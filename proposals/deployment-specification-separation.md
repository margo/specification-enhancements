# Specification Update Proposal: Separate Margo Core from deployment specifications

## Owner

@phil-abb

## Summary

This SUP reorganizes Margo into a technology-agnostic Margo Core specification and
independently versioned deployment specifications. It moves Helm (Kubernetes) and
Compose (Docker/Podman) deployment support rules out of the current specification, defines the
common rules that every deployment specification must follow, establishes how
the Helm and Compose specifications are maintained and referenced, and defines how new
deployment specification are handled.

Existing Helm and Compose application formats remain supported. The change is a
separation of responsibility and governance, not a replacement of either
deployment technology.

## Reason for proposal

The current specification combines two concerns with different owners and release
cycles:

1. Fleet-management interoperability: device identity and relationships,
   capability/profile reporting, matching, desired state, status, observability,
   and the management APIs.
2. Deployment technology: Helm charts, Kubernetes behavior, Compose archives,
   container-engine behavior, component packaging, and deployment-provider
   details.

The current documentation makes Helm and Compose privileged concepts in otherwise
technology-neutral pages. For example, the Application Description defines Helm
exceptions, the Application Registry defines the Compose OCI artifact, Device
Requirements defines Helm and Compose device requirements, and observability
requirements distinguish Kubernetes from Compose platforms. This couples
changes to those technologies to changes in the Margo core specification and makes
it difficult to add another deployment type without editing core contracts.

This proposal follows [product-management issue #97](https://github.com/margo/product_management/issues/97),
which calls for a technology-agnostic core and independent deployment
specifications.

This separation improves the specification by:

- allowing Helm, Compose, and future deployment specifications to evolve on their
  own schedules;
- allowing each deployment specification to have focused maintainers,
  documentation, and conformance tests;
- keeping workload fleet manager implementations focused on discovery, matching,
  orchestration, and lifecycle management rather than embedding deployment-provider logic; and
- preserving a clear interoperability contract for applications, devices, and
  workload fleet managers that support the same deployment specification.

[Issue #97](https://github.com/margo/product_management/issues/97) describes expectations for a schema-agnostic approach for matching applications to devices. This SUP has a dependency on an approved schema-agnostic, passthrough-by-default, approach. The [schema-agnostic characteristic matching SUP](https://github.com/margo/specification-enhancements/blob/pdp/schema-agnostic-matching/proposals/schema-agnostic-matching.md) proposes one way of handling this.

## Requirements alignment acknowledgement

This SUP addresses [product-management issue #97](https://github.com/margo/product_management/issues/97) for separating core and deployment specifications.

This SUP does not introduce any changes to the specification content
apart form reorganizing the existing content into three separate specifications:

- The core specification
- A Helm deployment specification
- A Compose deployment specification

The following are out of scope:

- defining any new contents for the Helm or Compose deployment specifications;
- defining a central deployment specification registry;
- defining deployment specification governance rules (these will be defined by PM group).

## Technical proposal

### 1. Specification parts

Margo shall publish the following independently versioned parts:

1. **Margo Core Specification.** This contains the common management and
   interoperability contract. The core specification will continue using current [specification repository](https://github.com/margo/specification)
2. **Common Deployment Specification Guidelines.** This contains rules and
   recommendations shared by all deployment specifications. It is not itself a
   deployment type and does not define a deployable artifact. A new `deployment-spec-common` repository
   will be created for this content.
3. **Helm Deployment Specification.** This defines the Helm (Kubernetes) deployment
   contract. A new `deployment-spec-helm` repository will be created for this content.
4. **Compose Deployment Specification.** This defines the Compose (Docker/Podman) and OCI
   container-engine deployment contract. A new `deployment-spec-compose` repository will be created for this
   content.

Margo Core MUST contain only the technology-neutral portions of the following model:

- device identity, hierarchy, and gateway relationships;
- the profile and mapping endpoints and their transport/security requirements;
- the schema-agnostic, passthrough-by-default, matching approach;
- schema-agnostic matching grammar and evaluation semantics;
- the Application Description and ApplicationDeployment fields required to carry
  an opaque deployment profile and common parameters;
- desired-state synchronization, deployment status, and lifecycle semantics;
- common application/package identity and registry references; and

### 2. Common Deployment Specification Guidelines

The common guidelines MUST define:

- the minimum metadata every deployment specification publishes, including a
  stable identifier, specification version, lifecycle state, owners, source
  repository, normative document, and conformance-test location;
- how a deployment type is identified and matched. Identifiers are opaque to
  Margo Core;
- requirements for a deployment specification to define its schema-agnostic
  capabilities, compatibility rules, and conformance tests;
- guidelines for how deployments specifications should evolution;
- governance, publication, versioning, lifecycle, deprecation, and retirement
  expectations for deployment specifications;
- the conformance test expectations for deployment specifications; and
- deployment specification repository naming and ownership conventions.

### 3. Helm and Compose deployment specifications

The following table is describes the initial repository and documentation
split. A topic listed under a deployment specification is removed from the
core specification.

| Content | Core Specification | Common guidelines | Helm specification | Compose specification |
| --- | --- | --- | --- | --- |
| Deployment identifier and lifecycle | N/A | Identifier format, registry metadata, lifecycle and governance | Helm deployment identifier and Helm deployment specification version | Compose deployment identifier and Compose deployment specification version |
| Component contract | Required common metadata, OCI reference conventions, revision and integrity expectations | Guidelines for defining packaging of deployable components | Helm chart artifact, Chart API v2, chart repository/OCI reference, chart version rules, Helm exception rules | Compose artifact type, single-layer OCI archive, archive media type, annotations and revision rules |
| Schema-agnostic device matching | Definition for schema-agnostic device descriptions and matching language | Guidelines for naming, schema publication, evolution | Helm-specific schema-agnostic definitions, such as supported Chart API v2, declared API set, operators | Compose-specific schema-agnostic definitions, such as engine and supported Compose behavior |
| Parameters | Device specific parameter implementation and non-deployment specific device specific parameters definition | Guidelines for handling device specific parameter that are deployment specific | Mapping a parameter to Helm values and any Helm-specific value rules | Mapping a parameter to Compose environment variables and Compose-specific substitution rules |
| Device requirements | Requirements for workload fleet manager clients, gateway, and observability | Requirement to advertise a schema-agnostic deployment definition | Kubernetes orchestration platform and Helm deployment-provider schema-agnostic definition | OCI container runtime and Compose deployment-provider schema-agnostic definition |
| Deployment behavior | Common components definition, ordering, wait/timeout field semantics | Guidelines for defining deployment specific requirements | Rendering/install/apply behavior, Kubernetes interaction, releases, and Helm restrictions | Compose project extraction, startup/reconciliation, runtime paths, secrets and archive safety |
| Observability | Common OTLP, source identity, export, and lifecycle requirements | Guidelines for defining deployment specific telemetry requirements | Kubernetes cluster, node, pod, container, and event telemetry | Docker/Podman container telemetry and Compose service identity |
| Conformance | Core specification conformance testing and reporting requirements | Guidelines for conformance testing requirements deployment specifications must provide | Helm chart, provider, Helm (Kubernetes) related conformance tests | Archive, OCI, Compose (Docker/Podman) related conformance tests |

#### Helm deployment specification content

The Helm deployment specification MUST contain the current Helm-specific material, including:

- support for Helm version 3 and version 4 using Chart API version 2;
- the Helm component artifact and its OCI/reference requirements;
- the permitted deployment approaches, including rendering and applying
  manifests without requiring a particular Helm CLI workflow;
- restrictions on lookup, hooks, CRD management, and other functions that
  require provider-specific Kubernetes API behavior;
- Kubernetes orchestration and Helm deployment-provider requirements;
- Helm value parameter target syntax and resolution behavior;
- Schema-agnostic Helm-specific device definition based on approved approach; and
- Kubernetes and Helm related conformance tests; and
- Kubernetes specific observability reporting.

#### Compose deployment specification content

The Compose deployment specification MUST contain the current Compose-specific material,
including:

- the Compose Specification version/compatibility statement;
- the Compose Archive layout, required compose.yaml, referenced-file rules,
  path traversal and link safety rules, and extraction behavior;
- OCI artifact type, layer media type, annotations, revision rules, and digest
  verification for Compose components;
- Docker and Podman runtime requirements and Compose project startup,
  reconciliation, wait, timeout, and health semantics;
- Compose environment-variable parameter target syntax and substitution rules;
- handling of secrets, bind mounts, named volumes, and device-defined runtime
  paths;
- Schema-agnostic Compose-specific device definition based on approved approach;
- Compose, Docker, Podman, archive, and container conformance tests; and
- Docker/Podman specific observability reporting.

### 4. Explicit documentation relocation matrix

The following pages need to be reviewed to move or rewritten content as follows:

| Current page | Required disposition |
| --- | --- |
| [Technical Lexicon](https://docs.margo.org/personas-and-definitions/technical-lexicon) | Keep common terms such as application, workload, device, WFM, and registry in Core. Make language deployment technology neutral. Move the Helm Chart, Compose Archive, provider, and deployment-type-specific definitions to the applicable deployment specification. |
| [Software Composition](https://docs.margo.org/personas-and-definitions/software-composition) | Keep the packaging-versus-deployment stages and generic application/component relationships in Core. Move the Helm and Compose component/workload examples and provider behavior to the applicable specifications. |
| [Envisioned System Design](https://docs.margo.org/overview/envisioned-system-design) | Keep the technology-neutral architecture, WFM, device, workload, and observability relationships. Replace Helm/Compose assumptions with deployment-agnostic references. |
| [Applications](https://docs.margo.org/overview/applications) | Keep Application Description goals, profile selection, and compatibility concepts. Move Helm/Compose support claims and technology-specific content to the deployment specifications. |
| [Application Package](https://docs.margo.org/concepts/applications/application-package) | Keep the application description, resources, and generic component relationships in Core. Move Helm Chart and Compose Archive composition and artifact details to the relevant specifications. |
| [Conceptual Application Registry](https://docs.margo.org/concepts/applications/application-registry) | Keep the three-registry relationship and the application-registry role in Core. Remove references to Helm and Compose. |
| [Local Registries](https://docs.margo.org/concepts/applications/local-registries) | Keep only registry locality and connectivity guidance in common documentation. Move Kubernetes registry mirrors and Helm caches to Helm; move Docker registry mirrors/caches and Compose artifact guidance to Compose. |
| [Workload Manager Device Capabilities](https://docs.margo.org/concepts/workload-fleet-managers/device-capabilities) | Keep generic profile reporting and matching. Move examples of Helm and Compose supported deployment types to their specifications. |
| [Workload Deployment](https://docs.margo.org/concepts/workload-fleet-managers/workload-deployment) | Keep Desired State retrieval, reconciliation, and status workflow in Core. Move provider execution examples to Helm and Compose. |
| [Edge Compute Devices](https://docs.margo.org/concepts/edge-compute-devices/devices) | Keep identity, relationships, generic profiles, resources, and gateway concepts in Core. Move the meanings of Helm and Compose deployment capabilities to the applicable specifications. |
| [Device Capabilities API](https://docs.margo.org/specification/margo-management-interface/device-capabilities) | Replace fixed deployment-specific capability functionality with the approved schema-agnostic approach. Define Helm and Compose specific device definitions in their deployment specification based on the approved approach. |
| [Desired State](https://docs.margo.org/specification/margo-management-interface/desired-state) | Keep transport, digest, caching, deployment lifecycle, and opaque profile propagation in Core. Move Helm and Compose specific content and examples to their deployment specification. |
| [Application Description](https://docs.margo.org/specification/applications/application-description) | Keep application metadata, generic profiles, components, parameters, and matching references in Core. Move Helm exceptions, Compose properties, and technology-specific parameter targets to the deployment specifications. |
| [Application Registry API](https://docs.margo.org/specification/applications/application-registry) | Keep OCI application-package retrieval, manifest, and common application-resource rules in Core. Move Compose artifact types, archive structure, annotations, and reconciliation rules to Compose; move any Helm component artifact rules to Helm. |
| [Device Requirements](https://docs.margo.org/specification/margo-devices/device-requirements) | Keep baseline WFM-client, hosting, and common profile requirements in Core. Move the Helm-enabled and Compose-enabled subsections to their specifications. |
| [Collecting Workload Observability Data](https://docs.margo.org/specification/observability/collecting-workload-observability-data) | Keep OTLP, export, source identity, and common WFM-client requirements in Core. Move Kubernetes receiver requirements to Helm and Docker/Podman receiver requirements to Compose. |

Pages retained in Core must start use deployment-neutral language such as “deployment
specification”, “deployment identifier”, “component artifact”, and “compatible
device”. The core specification must not make any specific deployment technology a normative dependency
of the core contract.

### 5. Repository and release model

The Margo project SHOULD maintain separate repositories for:

- Margo Core specification;
- the Common Deployment Specification Guidelines;
- the Helm Deployment Specification;
- the Compose Deployment Specification; and
- each new Deployment Specification;

Separating each deployment specification into its own git repository has the following advantages:

- It give better control over the content because deployment specification repositories can have their own set of owners that control the permissions for the repository so the Margo leadership doesn't have to be responsible for each
- Each deployment specification can move at its own pace and be versioned independently of anything else
- We can define a structure and expected metadata files so a deployment specification registry can be created more dynamically based on a known structure
- We can control when a new repository is created and how the initial owners are established.

### 6. Migration and compatibility

Migration should occur in these steps:

1. Publish the Core and Common Guidelines documents and cross-reference the
   existing pages.
2. Publish Helm and Compose specifications containing the current normative
   deployment rules and conformance tests.
3. Update application-package and registry documentation so existing artifacts
   resolve to the same deployment specification identifiers and artifact rules.
4. Update examples and schemas to use the approved schema-agnostic approach.
5. Remove duplicated technology-specific normative text from Core after the
   independent specifications are available.

Existing Application Description and ApplicationDeployment documents should remain
processable during the transition. The existing desired-state HTTP routes,
content-addressed artifacts, deployment-status reporting, mTLS requirements,
OCI application registry interface, and Helm/Compose artifact formats MUST NOT
change solely because of this reorganization.

### 7. Conformance model

Conformance claims MUST identify the specification parts supported. Examples are:

| Claim | Meaning |
| --- | --- |
| Margo Core | Supports the common APIs, profile exchange, matching, desired state, status, and lifecycle contracts; no deployment type is implied. |
| Margo Core + Helm | Also supports the Helm Deployment Specification and its conformance tests. |
| Margo Core + Compose | Also supports the Compose Deployment Specification and its conformance tests. |
| Margo Core + Helm + Compose | Supports both deployment specifications and may select between compatible profiles. |

A device or workload fleet manager MUST NOT claim Helm or Compose conformance
merely because it supports OCI containers. Deployment-specification conformance
requires the corresponding artifact, schema-agnostic definition, deployment behavior, and test
requirements.

## Breaking changes

This SUP is intended to be functionally non-breaking for existing Helm and Compose
deployments. It does introduce these specification-organization changes:

- normative content moves to new documents and repositories, so links and
  document ownership change;
- deployment technologies become independently versioned specifications rather
  than privileged core concepts;
- future deployment types are represented by schema-agnostic identifiers and their own
  specifications rather than additions to the core specification; and
- conformance claims become additive across Core and deployment specifications.

## Alternatives considered (optional)

### Keep all deployment content in Margo Core

Rejected because every Helm, Compose, or future-runtime change would continue to
couple core releases, governance, and conformance work to one deployment
technology.

### Add more deployment types as core features

Rejected because it preserves a privileged list and requires core changes for
every new technology. It also makes vendor and community deployment formats
compete for core-specification ownership.

### Keep one repository with separate directories

Considered as a transition arrangement, but not sufficient as the final model.
Separate repositories provide clearer ownership, permissions, release cadence,
issue tracking, and conformance test ownership. A single repository MAY be used
temporarily while the independent repositories are established.

### Put deployment rules in specification extensions

Rejected because extensions may be ignored and are not a suitable contract for
portable application placement or deployment conformance. Extensions remain
appropriate for supplier-specific information that is not part of a deployment
specification.

## Rejection reason

N/A. This is a draft proposal for working-group review.
