# Specification Enhancements
***
## Margo Members

Follow the [specification update proposal process](https://github.com/margo/Margo-collaboration/blob/main/specification_update_processes/process-overview.md) to propose updates to the specification
***
## Individual Contributor (Non-member)

If you, as an individual non-Margo member, own the contribution, you **MUST SIGN** the Individual [Contributor License Agreement](https://github.com/margo/EasyCLA-public-feedback) (CLA) at the point of submitting a PR. This signed CLA will be required before your contribution can be merged into the project repository. As an individual contributor, you are acting on your own behalf and not on your employer.
***
## Submission Steps

**Step 1:** The Contributor selects either Category 1 or Category 2 based on the type of submission proposal.

**Category 1 (Cat 1):** [Minor Bug Report Issue](https://github.com/margo/specification/issues) or [pull request submission](https://github.com/margo/specification/pulls) to suggest a simple editorial change

**Category 2 (Cat 2):** [Specification Enhancement Request](https://github.com/margo/specification-enhancements/blob/main/sup-template.md) detailing a proposed function.

<img width="2606" height="880" alt="image" src="https://github.com/user-attachments/assets/880ae53b-298c-42c6-a816-8c2ba166066f" />

**Cat 1 - Step 2:** [Submit PR](https://github.com/margo/specification/pulls)

**Cat 2 - Step 2:** [Submit Enhancement Request](https://github.com/margo/specification-enhancements)

**ALL** Non-member PR submissions must sign the [Contributor License Agreement](https://github.com/margo/EasyCLA-public-feedback).  

### Process Outcomes

#### **Success Paths**
- **Cat 1**:
  - PR approved by TWG members and merged into the appropriate release branch
- **Cat 2**:
  - Enhancement requests accepted and added to the roadmap 
  - (Optional) Specification update proposal (SUP) document submitted and approved by the TWG technical leads

***

## SUP Status

### Stages

[Phase 3: SUP Technical Design (P1)](https://github.com/margo/Margo-collaboration/blob/main/specification_update_processes/process-overview.md#phase-3-sup-technical-design)

A `draft` PR was created in the `/proposals` folder indicating the SUP has been started and the SUP group is working on the technical design and supporting code. During this phase community members can comment on the daft PR if they wish even though the SUP details are still being worked on.

[Phase 3: Final Review (P2)](https://github.com/margo/Margo-collaboration/blob/main/specification_update_processes/process-overview.md#phase-3-sup-technical-design)

The PR has been moved out of the `draft` state indicating the technical details and example code are ready for a final review before voting. Margo community members have two weeks to start providing feedback on the PR, or prepare alternative proposals. Once all comments are addressed, the SUP will be voted on by the TWG leads.

[Phase 4: SUP Approved (P3)](https://github.com/margo/Margo-collaboration/blob/main/specification_update_processes/process-overview.md#decision-gate-2)

The SUP has been approved and the SUP group is working on updating the specification and code first sandbox repositories.

### Status

|Owner|Description|Stage|LINK|
|-----|-----------|-----|----|
|@silvanoc | Data model single source of truth with LinkML | P3 | [SUP](https://github.com/margo/specification-enhancements/blob/main/proposals/data-model_linkml.md)
|@ajcraig | Moving away from Device Roles to capabilities | P3 | [SUP](https://github.com/margo/specification-enhancements/blob/main/proposals/sup_device_roles_to_capabilities.md) |
|@phil-abb | Supporting specification extensions | P3 | [SUP](https://github.com/margo/specification-enhancements/blob/main/proposals/sup_enable_vendor_extensions.md) |
|@matlec | Margo Identity and Authorization Framework | P3 | [SUP](https://github.com/margo/specification-enhancements/blob/main/proposals/margo-identity-and-authorization-framework.md) |
|@matlec | WFM Identity Profile | P3 | [SUP](https://github.com/margo/specification-enhancements/blob/main/proposals/wfm-identity-profile.md) |
|@javatask | Canonical OCI component publishing pattern | P2 | [PR](https://github.com/margo/specification-enhancements/pull/67) |
|@javatask | Compose OCI registry publishing and archive structure | P2 | [PR](https://github.com/margo/specification-enhancements/pull/68) |
|@javatask | Quadlet deployment type | P2 | [PR](https://github.com/margo/specification-enhancements/pull/69) |
|@arne-broering | Supporting device specific runtimes | P2 | [PR](https://github.com/margo/specification-enhancements/pull/55) |
|@phil-abb | Alternative proposal for supporting device specific runtimes | P2 | [PR](https://github.com/margo/specification-enhancements/pull/59) |
|@jjaswanson4 | Move Application Deployment Templating to the WFM | P1 | [Draft PR](https://github.com/margo/specification-enhancements/pull/40) |
|@arne-broering | Device specific parameter values | P1 | [Draft PR](https://github.com/margo/specification-enhancements/pull/54) |
|@singhmj-1 | Resource allocation and conflict resolution proposal for app workloads | P1 | [Draft PR](https://github.com/margo/specification-enhancements/pull/66) |
|@vireshnavalli | RFC 9457 for common error responses | P1 | [Draft PR](https://github.com/margo/specification-enhancements/pull/73) |
|@vireshnavalli | Updates to apiVersion usage | P1 | [Draft PR](https://github.com/margo/specification-enhancements/pull/74) |
|@pperycz | Support for real-time workloads | P1 | [Draft PR](https://github.com/margo/specification-enhancements/pull/75) |

## SUP Owner SUP Checklist

This is a checklist for the [Specification Update Proposal processes](https://github.com/margo/Margo-collaboration/blob/main/specification_update_processes/process-overview.md) for SUP owners for follow. Please read the full processes to make sure you understand it before using this checklist

### Phase 1- Research & Roadmap Alignment

- [ ] Review [In Progress](https://github.com/margo/specification-enhancements#status) and [Rejected](https://github.com/margo/specification-enhancements/tree/main/rejected) SUPs to see if any information applies to your SUP
- [ ] Review [TWG Feature Backlog](https://github.com/orgs/margo/projects/22/views/3) to see if there are any features your SUP addresses
- [ ] Optional: Discuss planned SUP with TWG Char/Co-Chair, with PM group during a weekly PM meeting, or with the TWG during a bi-weekly technical sync call

### Phase 2 - Proposal Creation

- [ ] Complete the [SUP template's](https://github.com/margo/specification-enhancements/blob/main/sup-template.md) "Owner", "Summary" and "Reason for Proposal" sections
- [ ] Create a [`Draft` PR](https://github.blog/news-insights/product-news/introducing-draft-pull-requests/) to add your SUP to the `proposals` folder in the [specification-enhancements](https://github.com/margo/specification-enhancements) repository
- [ ] Notify the TWG Chair/Co-Chair and Proposal Committee, via tagging in a PR comment, you are starting a SUP

### Phase 3 - SUP Technical Design

- [ ] Optional: Invited community members to join your SUP group to help work on the SUP
- [ ] Complete the `Technical Proposal` and `Alternatives considered (optional)` template sections
- [ ] Implement your proposal on a branch of the [Sandbox repository](https://github.com/margo/sandbox) or as a PoC in a different location
- [ ] Ensure a feature exists in the backlog and is documented in the `Requirements Alignment Acknowledgement` template section. Work with the TWG Chair/Co-Chair or PM Group to create a feature if one does not exist.
- [ ] Move your PR out of `Draft` state
- [ ] Notify the TWG Chair/Co-Chair and Proposal Committee, via tagging in a PR comment, you are ready to submit your SUP

### Phase 4 - Final Decision: Approved

- [ ] Update the specification with the details for the SUP and create a PR
- [ ] Work with the dev team to implement the changes to the Sandbox and create a PR
- [ ] Once PRs are completed notify the TWG Chair/Co-Chair and Proposal Committee that the SUP has been completed.

> **Note:**
>
> - if SUP is rejected, the SUP owner does not need to do anything further.
> - If SUP is undecided, the SUP owner does not need to do anything further until the external community has evaluated the options, provided feedback, and a decision has been made.
