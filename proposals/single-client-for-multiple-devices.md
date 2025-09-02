# Specification Update Proposal

## Owner

Merrill Harriman, Julien Duquesnay

## Summary

We propose to enhance the Workload Fleet Manager to device interface to allow a single client - a gateway service for example - to provide device capabilities, retrieve desired states, etc. on behalf of one or multiple target devices. 

The idea is to augment the Workload Fleet Manager interface to enable the connection of this "single client", while the communication between the "single client" and the target devices stays out of the scope of Margo (it could be a proprietary communication).

Additional services required by Margo to be provided by the device, e.g. OTEL collector, are not in the scope of this SUP.

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

## Alternatives considered (optional)

> List any alternative solutions considered while working on the SUP and the reason for not choosing them. If the SUP owner knows that there is a risk of a competing SUP, this section can be used to make their case ahead of any potential votes on why their solution is better.
> 
> Complete as part of Phase 3: SUP Technical Development

## Rejection reason

> If a SUP is rejected, indicate the reason why it was rejected.
> 
> Complete if SUP is rejected at Phase 2: Proposal Creation or Phase 4: Final Decision 
