# Specification Update Proposal

## Owner

[@phil-abb](https://github.com/phil-abb)

## Summary

This SUP proposes that Margo-compliant applications MUST NOT use Helm v3 or v4 features that require interaction with the Kubernetes API during template rendering or deployment. Features such as Helm Lookup, Helm hooks, server-side apply (Helm 4), CRD management, "--wait" kstatus, and Helm tests are explicitly disallowed.

## Reason for proposal

The current Margo specification does not clearly define which Helm features are supported for application deployment. Allowing features that require Kubernetes API interaction (e.g., Lookup, hooks) creates ambiguity and implementation challenges for device vendors, as some approaches do not support server-side template rendering. This can lead to application failures that are difficult to diagnose. Disallowing these features ensures consistent, predictable behavior and avoids security concerns associated with uncontrolled API interactions.

Additionally, by not requiring the use of the Helm tool itself, this SUP allows vendors to use alternative approaches for rendering and applying manifests from Helm charts. For example:

- Rendering Helm templates using a library (e.g., [Helm Go SDK](https://pkg.go.dev/helm.sh/helm/v3/pkg)) and applying the resulting manifests with `kubectl` or a Kubernetes client library.
- Converting Helm charts to plain Kubernetes manifests ahead of time and distributing those manifests directly.
- Using [Kustomize](https://kustomize.io/) to render and apply overlays and transformations to Kubernetes manifests, then applying them with `kubectl` or a Kubernetes client library.
- Using custom tools or CI/CD pipelines to render and apply manifests, as long as they do not require Kubernetes API interaction during rendering.

This flexibility enables vendors to choose the most appropriate tooling for their environment, without being forced to use the Helm CLI or server-side rendering features.

Allowing Helm hooks also introduces additional security risks, as hooks can execute arbitrary operations during chart installation, upgrade, or deletion, potentially escalating privileges or interfering with the cluster in unintended ways. Furthermore, hooks can make it difficult for device vendors to reliably apply required updates to manifests, such as injecting security patches or [observability environment variables](https://docs.margo.org/specification/observability/collecting-workload-observability-data#connecting-to-the-opentelemetry-collector), because the hook logic may override or bypass these modifications. By disallowing hooks, this SUP ensures that device vendors retain full control over the manifest lifecycle and can enforce security and observability requirements consistently across all deployments.

## Requirements alignment acknowledgement

This SUP addresses [feature 148](https://github.com/margo/specification/issues/148) and is related to [issue 103](https://github.com/margo/specification/issues/103). It aligns with Margo's vision of providing clear, interoperable rules for application deployment and management workflows. No new requirements outside the Margo scope are introduced.

## Technical proposal

Margo-compliant applications MUST NOT depend on the use of the following Helm features, each of which requires communication with the Kubernetes API:

- [Lookup](https://helm.sh/docs/chart_template_guide/functions_and_pipelines#using-the-lookup-function): The `lookup` template function queries live resources in the target Kubernetes cluster at render time, for example to fetch ConfigMaps, Secrets, or other objects. This requires direct API access and cluster credentials during rendering.
- [Hooks](https://helm.sh/docs/topics/charts_hooks/): Hooks (pre/post install, upgrade, delete, rollback, test, etc.) execute additional Kubernetes resources or jobs at specific points in the release lifecycle. Helm waits for these resources to reach a ready state by monitoring them via the Kubernetes API, and the hooks themselves may perform privileged or arbitrary operations in the cluster.
- [CRD management via helm](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/): Installing or managing Custom Resource Definitions (CRDs) with Helm requires checking for the presence of CRDs and registering new APIs with the cluster, which involves API calls and may require elevated permissions.
- ["--wait" and kstatus](https://helm.sh/community/hips/hip-0022/): The `--wait` flag and kstatus integration cause Helm to poll or watch the Kubernetes API to determine when resources (including custom resources) are fully reconciled and ready. This requires ongoing API access and additional RBAC permissions.
- [Helm tests](https://helm.sh/docs/topics/chart_tests/): Helm tests are implemented as hooks that create and monitor test jobs in the cluster, requiring API access to create, observe, and clean up test resources.
- Server-side apply (Helm 4): Server-side apply uses the Kubernetes API's server-side apply feature to manage resource changes, which requires direct API calls and may result in different behavior than client-side apply.

These features all require live communication with the Kubernetes API during rendering or deployment, which is incompatible with the requirements and security model of Margo-compliant applications.

The [application description](https://docs.margo.org/specification/applications/application-description) page will be updated to indicate application vendors should not depend on these features in their helm charts.

All references to using a Helm deployment will be updated with a footnote pointing to the information indicating the unsupported features.

All Helm chart rendering MUST be able to be performed client-side, without requiring communication with the Kubernetes API. Application developers MUST ensure their charts are compatible with this restriction. Device vendors MAY implement Helm chart installation using any approach they choose, including but not limited to the examples listed above.

The [device requirements](https://docs.margo.org/specification/margo-devices/device-requirements) page will be updated to indicate device vendors have the freedom to choose how helm manifests are rendered and applied and do not need to support these features.

The Margo compliance test suite for conformant applications will be updated to validate that disallowed Helm features indicated above are not present in submitted charts. Applications using these features will be flagged as using unsupported Helm features. This doesn't automatically indicate the application is not compliant because it is possible to have some of these items in the helm chart, but not rely on them to be used to install that application (e.g., you can have a lookup command only used when a specific parameter in the values.yaml is not specified).

## Alternatives considered (optional)

Supporting all Helm v3/v4 features was considered, but would prevent device vendors from using approaches that do not support server-side template rendering, and would complicate WFM-driven deployments. Security concerns also influenced the decision.

## Rejection reason

N/A
