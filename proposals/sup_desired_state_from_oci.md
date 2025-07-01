# Specification Update Proposal

## Owner

@Silvanoc

## Summary

Instead of using Git to transport the desired state to the device, a REST API is used - namely, the established REST API of the [OCI Distribution v1.1 specification](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md). This results in the benefit that available OCI-registries and client libraries can be used.

## Reason for proposal

This SUP relates to this [feature](https://github.com/orgs/margo/projects/22/views/2?pane=issue&itemId=111409860&issue=margo%7Cspecification%7C100), which addresses the need to define the desired state artifact(s) that are produced via the WFM, and retrieved via the edge client, to enable state seeking management of workloads at the edge.

The [specification currently indicates Git](https://specification.margo.org/fleet-management/workload/workload-deployment/) is used for this process but the Margo TWG members voted against using Git because of the following:

1. Git is an industry standard but not a specification so there is no way to implement compliance testing based on Git. It also means the Git implementation could change at any point. This proposal addresses this concern because there is an OCI specification we can reference.
2. Since Git was not based on a specification the members felt we were dictating an implementation. This proposal addresses this concern because OCI is based on a specification. Vendors have the freedom to choose whichever implementation approach they wish to comply with the Margo specification.
3. There were concerns over the amount of additional storage space a git repository requires on the device. While this can be alleviated with shallow checkouts, using an OCI artifact for the desired state doesn't include the complete history like a Git repository does.
4. There were concerns over the overhead of requiring a git client on the device. With this proposal, requests are all HTTP(s) requests and can be made using standard HTTP(s) clients/libraries.
5. There were concerns over the chattiness between the Git client and the server. By minimizing the number of layers in the OCI artifacts we have some control over how chatty the API calls will be. We will need to keep this in mind as we work on the technical details for this proposal.

## Requirements alignment acknowledgement

This proposal address part of this [feature](https://github.com/orgs/margo/projects/22/views/2?pane=issue&itemId=111409860&issue=margo%7Cspecification%7C100).

The following is in scope for this proposal:

1. Packaging, and distributing, the desired state documents as an OCI artifact
1. Using the APIs defined in the [OCI Distribution v1.1 specification](https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md) for pulling the OCI artifact to the device.

The following is not in scope for this proposal:

1. Defining how the device receives the location and authentication information for where the OCI artifacts are available
2. Defining how a device is notified of a updated desired state OCI artifacts beyond using the "latest" tag.

## Technical proposal

> The SUPs' technical details. There must be enough technical details that someone can take the information in this section and implement it on their own.

## Alternatives considered (optional)

> List any alternative solutions considered while working on the SUP and the reason for not choosing them. If the SUP owner knows that there is a risk of a competing SUP, this section can be used to make their case ahead of any potential votes on why their solution is better.

## Rejection reason

> If a SUP is rejected, indicate the reason why it was rejected.
