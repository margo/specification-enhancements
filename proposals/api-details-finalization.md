# Specification Update Proposal

## Owner

Armand Craig
Nilanjan Samajder

## Summary

This SUP is focused on finalizing the technical details pertaining to our previously approved REST API, for usage between the Device and the Workload Fleet Manager. 
    - see the following [Link](https://github.com/margo/specification/issues/21) for a history on when the REST API was approved for the following functionalities
        - Onboarding of the API Interface 
        - Providing WOS with Device Capabilities information (content of message in another Decision tracker issue)
        - Providing WOS with workload deployment status updates

Technical Details the SUP aims to finalize in the specification:

- Secure certificates utilized / format
- API Authentication Mechanism
- API Security / Encryption strategy
- How unique identifiers are produced and who is responsible for providing them/maintaining them. 
- Required Ports to enable the edge to cloud communication
- TLS Protocol version
- API definition / documentation strategy

What this SUP does not cover:
- How the API is onboarded in an automated fashion
- How the desired state is retrieved via the edge device
- Multi-vendor trusted CA strategy for Margo

## Reason for proposal

The main reason for this proposal is to complete and gain community consensus on the technical details pertaining to the decided API mechanism for Devices to communicate with the WFM. 

## Requirements alignment acknowledgement

This SUP is aligned with the following Technical Feature. 
- https://github.com/margo/specification/issues/101


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
