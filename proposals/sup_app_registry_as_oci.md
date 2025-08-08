# Specification Update Proposal

## Owner

[@arne-broering](https://github.com/arne-broering)

## Summary

Instead of using Git to provide an application to WFM, a REST API is used - namely, a well-established, well-defined, and widely-used API for digital artifact distribution and management.
This API is particularly easy to implement thanks to its simplicity, making it accessible for devices with varying capabilities.

This approach offers the significant benefit of leveraging available registries and client libraries.
There are multiple open-source software implementations of both registries and client libraries that are battle-tested and widely used across the industry, eliminating the need to develop and maintain custom solutions.

## Reason for proposal

This SUP relates to  [feature 89](https://github.com/margo/specification/issues/89), which addresses the need for Margo to prescribe a way for App developers to host and securely provide access to application packages and application description files.

The [specification currently indicates Git](https://specification.margo.org/app-interoperability/workload-orch-to-app-reg-interaction/) is used for this process, but there are several reasons against this:

1. Git is an industry standard but not a specification so there is no way to implement compliance testing based on Git. It also means the Git implementation could change at any point. This proposal addresses this concern by using a well-defined specification with clear compliance requirements.
1. Since Git is not based on a specification this proposal references an established specification. Vendors have the freedom to choose whichever implementation approach they wish to comply with the Margo specification.
1. Alignment with the currently ongoing definition of a REST API for the desired state using the same technology as proposed here.

## Requirements alignment acknowledgement

This proposal addresses parts of [feature 89](https://github.com/margo/specification/issues/89).

The following is in scope for this proposal:

1. Define standard interface
1. Define security aspects of interface and responsibility
1. Define how the WFM "onboards" to the description storage location
1. Reference implementation utilizing open-source components and example security components.

## Technical proposal

> The SUP's technical details. There must be enough technical details that someone can take the information in this section and implement it on their own.
> 
> Complete as part of Phase 3: SUP Technical Development

## Alternatives considered (optional)

> List any alternative solutions considered while working on the SUP and the reason for not choosing them. If the SUP owner knows that there is a risk of a competing SUP, this section can be used to make their case ahead of any potential votes on why their solution is better.
> 
> Complete as part of Phase 3: SUP Technical Development

## Rejection reason

> If a SUP is rejected, indicate the reason why it was rejected.
> 
> Complete if SUP is rejected at Phase 2: Proposal Creation or Phase 4: Final Decision 
