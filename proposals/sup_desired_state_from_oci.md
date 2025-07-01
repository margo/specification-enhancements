# Specification Update Proposal

## Owner

@Silvanoc

## Summary

Instead of using Git to transport the desired state to the device, a REST API is used - namely, the established REST API of the [OCI Distribution v1.1 specification](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md). This results in the benefit that available OCI-registries and client libraries can be used.


## Reason for proposal

This SUP relates to this [issue](https://github.com/orgs/margo/projects/22/views/2?pane=issue&itemId=111409860&issue=margo%7Cspecification%7C100), which addresses the need to define the desired state artifact(s) that are produced via the WFM, and retrieved via the edge client, to enable state seeking management of workloads at the edge.







## Requirements alignment acknowledgement

> An acknowledgement that the SUP meets minimum requirements and doesn't introduce any requirements that are out of Margo's scope or vision. This section must have link(s) to applicable features and a statement about any requirements that were agreed to be out of scope for the SUP.

## Technical proposal

> The SUPs' technical details. There must be enough technical details that someone can take the information in this section and implement it on their own.

## Alternatives considered (optional)

> List any alternative solutions considered while working on the SUP and the reason for not choosing them. If the SUP owner knows that there is a risk of a competing SUP, this section can be used to make their case ahead of any potential votes on why their solution is better.

## Rejection reason

> If a SUP is rejected, indicate the reason why it was rejected.
