# Specification Update Proposal

## Owner

Armand Craig
@ajcraig
acraig@project.margo.org

## Summary

This SUP's intent is to replace device `roles` with a more flexible `capabilities` framework to enable expanded use-cases and ensure we are not over prescribing "devices" within the specification. 

## Reason for proposal
The goal of this SUP is to improve the specification sections revolving around Margo devices. The original idea for devices was to assign a [`device role`](https://docs.margo.org/specification/margo-devices/device-requirements) to a particular device. This enabled us to quickly frame out the first two device roles, `Standalone cluster` and `Standalone device`. However, as the community expands use cases related to devices, the roles concept is limiting the flexibility of the architecture. 

Groups of functionality and/or configuration of the device will enable a unique capability that is reported to the WFM during device onboarding. This will enable the WFM to understand what types of manifests/workloads/configurations are compatible with the newly onboarded device. 

Relevant concepts that drove this SUP:
- Approval of the Gateway SUP
    - https://github.com/margo/specification-enhancements/pull/36
- Concept of multi functional device
    - i.e. device that supports one to many application manifests
- Enable constrained device types
    - WASM / leaf devices
- Enable freedom for device and platform vendors to create unique offerings in the field while not restricting management capabilities via Margo.

 including gateway functionality, multi functional devices, 

## Requirements alignment acknowledgement

This SUP aims to close out the following TWG Feature:
- [Replace device roles with capabilities within Margo](https://github.com/margo/specification/issues/136)

## Technical proposal
(coming soon)

## Alternatives considered (optional)

N/A

## Rejection reason