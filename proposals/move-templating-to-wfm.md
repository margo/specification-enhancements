# Specification Update Proposal

## Owner

| Name | Email | Organization |
| --- | --- | --- |
| Josh Swanson | jswanson@redhat.com | Red Hat |
| Tim Mirth | tmirth@redhat.com | Red Hat |
| Luca Ferrari | ferrari@redhat.com | Red Hat |

## Summary

This SUP is to adjust the features and functionality of the agent deployed to devices under the management of a fleet/workload manager - moving more of the “heavy lifting” up to the manager level, while keeping the agent feature set more streamlined.

Specifically, this SUP moves templating of the application deployment (and the corresponding tooling) to the WFM level, instead of expecting the agent/device to handle it.

## Reason for proposal

Under the current specification, the agent is responsible for most actions related to the deployment of an application to a device, and requires a relatively large amount of functionality to be handled by the agent (and thus, the device).

These actions include:
- Pulling of container images from a registry
- Unpacking of those container images
- Pulling application deployment specifications
- Pulling env vars/variables related to deployment
- Rendering templated deployment from deployment specification and variables
- Applying the deployment
- Reporting deployment information back to the corresponding manager
- Reporting device information to the corresponding manager
- Reporting deployment information to the corresponding manager

This approach runs counter to generally accepted best practices for large-scale device management, specifically:
1. Devices should only communicate with management tools instead of content sources, code repositories, etc.
    * This greatly simplifies communication patterns, making network security more  easily configured and managed
    * Many end users simply do not allow communication from devices, specifically ones in L2 and below, to communicate with any systems not at the site
    * Firewalls between levels can be more easily managed and configured for the onboarding of new devices, as traffic patterns are firmly established and aligned to specific resources instead of a large number of possible endpoints
2. The resources of a device (cpu, memory, network bandwidth, etc) should be primarily focused on production workloads
    * Established workloads take priority over any management functionality
    * All management activity should be as limited as possible, to keep device resources free for production workloads
    * Required for embedded/low power devices, where compute availability is very limited

In an ideal state, the agent would only be responsible for:
- Gathering the desired rendered state from the manager
- Gathering required assets (container images, etc) from the manager
- Apply the desired state to the device
- Report back deployment information
- Report application information to the manager

All other actions should be taken at the manager layer, such as:
- Gathering application deployment specification
- Storing application deployment information
- Gathering deployment variables
- Rendering the application deployment relative to the managed device
- Host the required deployment information for the device agent to retrieve
- Accept the incoming information from the agent

This SUP is focused on moving the templating functionality to the WFM, and having the devices simply pull the rendered deployment definition and apply it.

## Requirements alignment acknowledgement

This SUP addresses functionality and behavior in-scope for Margo, specifically: the behaviors of managers and agents, thus, is aligned to Margo's overall scope and vision.

## Technical proposal

![WFM vs. Device Responsibilities](./res/move-templating-to-wfm/wfm-vs-device-responsibilites.png)

From a technical perspective, most/all of the existing functionality is kept, however where the functionality is executed and by what layer of the architecture is adjusted.

The intention is to keep the requirements of the agent and the underlying device as lightweight and straightforward as possible, requiring the least amount of dependencies and tools be shipped with the agent. While general purpose devices are fairly straightforward to ship tools and technologies too, embedded devices carry unique challenges.

## Alternatives considered (optional)

Many tools already prevalent in the application deployment and device management space leverage this behavior pattern.

ArgoCD, Kustomize, and Helm all render the deployment information at the time of deployment, and then apply the rendered manifests to the deployment target.

Various edge management applications also behave this way (flightctl, for example) - with the intention of keeping the devices as focused on real workloads instead of management work.

## Rejection reason

> If a SUP is rejected, indicate the reason why it was rejected.
> 
> Complete if SUP is rejected at Phase 2: Proposal Creation or Phase 4: Final Decision 
