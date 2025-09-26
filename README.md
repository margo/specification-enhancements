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

[Phase 2 Decision Gate 3 (P2)](https://github.com/margo/Margo-collaboration/blob/main/specification_update_processes/process-overview.md#decision-gate-3)

Decision gate 3 is a PR to merge the initial proposal into the `/proposals` folder. The focus is on making sure what the SUP owner plans to propose is clear and aligns with the roadmap. Technical details are not expected at this point.

[Phase 3: Technical Design (P3)](https://github.com/margo/Margo-collaboration/blob/main/specification_update_processes/process-overview.md#phase-3-sup-technical-design)

Phase 3 is when SUP groups work on the technical design. The SUP owner may create a draft PR to merge the SUP into the `/submitted` folder to help get feedback from the community.

[Phase 4: Final (P4)](https://github.com/margo/Margo-collaboration/blob/main/specification_update_processes/process-overview.md#phase-4-final-decision)

Phase 4 is a PR to merge the full proposal to the `/submitted` folder. During this phase, community members can post comments on the PR. Once comments have been addressed, the SUP will be voted on by the TWG Leads.

### Status

|Owner|Description|Stage|LINK|
|-----|-----------|-----|----|
|@silvanoc | Desired state as OCI | P3 | [SUP Markdown](https://github.com/margo/specification-enhancements/blob/desired_state_from_oci/submitted/sup_desired_state_from_oci.md) |
|@arne-broering | Application package as OCI | P3 |  |
|@julienduquesnay-se | Device gateway | P2 | [PR](https://github.com/margo/specification-enhancements/pull/7) |
|@ajcraig | API Onboarding | P2 | [PR](https://github.com/margo/specification-enhancements/pull/8) |
|@matlec | Desired state as Manifest API | P2 | [PR](https://github.com/margo/specification-enhancements/pull/9) |  
