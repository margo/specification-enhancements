# Specification Update Proposal

## Owner

[@phil-abb](https://github.com/phil-abb)

## Summary

This SUP proposes updating the Margo specification to add explicit support for Helm version 4 for application deployments, in addition to the already-supported Helm version 3. Helm chart compatibility is maintained across these versions and that breaking changes between v3 and v4 are limited to lower-level implementation details, not the chart format or rendering. Both Helm v3 and and v4 support the [Chart API Version 2](https://helm.sh/docs/v3/topics/charts) specification so this support will be further clarified by indicating Helm v3 and Helm v4 are supported through the Chart v2 specification.

## Reason for proposal

The current Margo specification indicates support for Helm v3 but does not address Helm v4, which has now been released. To ensure forward compatibility and provide clear guidance to application developers and device integrators, the specification should explicitly support both Helm v3 and v4. This proposal addresses [feature #148](https://github.com/margo/specification/issues/148), which calls for a defined set of Helm features and explicit version support, and references [issue #103](https://github.com/margo/specification/issues/103) regarding deployment compatibility.

## Requirements alignment acknowledgement

This proposal aligns with the Margo vision of open, interoperable orchestration at the edge by ensuring that application packaging and deployment workflows remain standards-based and forward-compatible. It addresses [feature #148](https://github.com/margo/specification/issues/148) and does not introduce requirements outside the current scope.

## Technical proposal

Update the Margo specification to state that Helm v4 is supported for application deployment, in addition to Helm v3. Since Helm chart format compatibility is maintained between v3 and v4; breaking changes between these versions are limited to the Helm CLI and library APIs, not the chart structure or rendering. This chart compatibility is maintained because both Helm v3 and Helm v4 are based on the [Chart v2 API specification](https://helm.sh/docs/v3/topics/charts). There is ongoing work to define a [Chart V3 API](https://helm.sh/community/hips/hip-0020/), which will eventually also be supported with Helm v4, but this is not released yet.

### Specification Updates

- The `DeploymentProfile` type in the Margo Application Description document (see [specification reference](https://docs.margo.org/specification/applications/application-description#deploymentprofile-attributes)) MUST be updated to use only `helm` as the value, instead of `helm.v3` or `helm.v4`. This reflects that both Helm v3 and v4 are supported and that the chart format is compatible across these versions.
- The `DeploymentProfile` type in the Margo Desired State document (see [specification reference](https://docs.margo.org/specification/margo-management-interface/desired-state#deploymentprofile-attributes)) MUST be updated to use only `helm` as the value, instead of `helm.v3` or `helm.v4`. This reflects that both Helm v3 and v4 are supported and that the chart format is compatible across these versions.
- All references in the Margo specification that indicate only Helm v3 is supported MUST be updated to state that both Helm v3 and Helm v4 are supported for application deployment.
- The documentation will be updated to clarify Helm v3 and Helm v4 are supported because the are based on the use of the Chart v2 API specification.

### Compatibility Details Between Helm v3 and v4

- **Chart Format:** The Helm chart format remains compatible between v3 and v4. Charts created for v3 do not require modification to be used with v4, and vice versa. There are no breaking changes to the `Chart.yaml`, templates, or values file structure.
- **CLI and SDK Changes:** Most breaking changes in Helm v4 are at the CLI and Go SDK/library level. These include refactoring of internal APIs, changes to plugin interfaces, and updates to command-line flags and behaviors. These do not impact the ability to render or install charts created for v3.
- **Deprecations and Removals:** Some deprecated flags and commands from v3 have been removed in v4. For example, certain plugin hooks and legacy flags may no longer be available. This affects tool and automation authors, not chart authors.
- **OCI and Registry Handling:** Improvements and bug fixes in OCI registry support have been made in v4, but these are backward compatible for charts packaged and distributed using OCI.
- **Behavioral Fixes:** v4 includes bug fixes and behavioral changes (e.g., handling of nil values, CRD management, and wait strategies) that may affect edge cases in chart installation, but do not break chart compatibility.

In summary, Helm v4 is a drop-in replacement for v3 from a chart author and user perspective. The main compatibility considerations are for developers of Helm plugins, automation, or integrations using the Go SDK, not for the charts themselves.

#### References

- [Helm v4.0.0 Release Notes](https://github.com/helm/helm/releases/tag/v4.0.0)

## Alternatives considered (optional)

Supporting only Helm v3 was considered but rejected to ensure forward compatibility and leverage improvements in Helm v4.

## Rejection reason

N/A
