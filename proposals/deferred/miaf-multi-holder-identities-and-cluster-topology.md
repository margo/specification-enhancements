# Specification Update Proposal

## Owner

[@matlec](https://github.com/matlec) (currently deferred — owner to be confirmed at promotion)

## Summary

Defines the framework-level Multi-Holder Logical Identities primitive under MIAF, plus the cluster topology profile for the WFM Client Identity Profile that applies that primitive. Originally drafted as part of the active MIAF SUP and the WFM Client Identity Profile; deferred for PR 2 since baseline interoperability covers the standalone (single-holder) topology only — gateway deployments also fit the single-holder primitive defined in the WFM Client Identity Profile and are addressed there via a binding-subject choice. The primitive is wire-compatible with single-holder identities (a multi-holder SVID is the same X.509 cert), so adding multi-holder support later does not require breaking changes to verifiers.

## Reason for proposal

Multi-node Kubernetes clusters need WFM Client identities that are shared across cooperating runtime entities. The primitive defines what "multi-holder" means at the framework level (so future profiles for gateway HA, telemetry agents, DFM clients, or workloads can reuse it without redefinition) and applies it to the cluster topology in the WFM Client Identity Profile. Gateway deployments are out of scope here — the baseline gateway WFM Client identity is single-holder and is covered by the WFM Client Identity Profile via a gateway-installation binding-subject choice; a future Gateway HA SUP could re-enter multi-holder territory if cooperating gateway controllers become a concern.

## Requirements alignment acknowledgement

This SUP extends the active [MIAF SUP](../margo-identity-and-authorization-framework.md) and the [WFM Client Identity Profile](../wfm-client-identity-profile.md). Detailed feature linkage and Owner are TBD at promotion.

## Technical proposal

### 1. Framework: Multi-Holder Logical Identities

MIAF supports two patterns for the relationship between an SVID and the runtime entities that present it:

- **Single-holder identities:** the SVID is held and presented by exactly one entity, and the SVID effectively proves both the logical identity and the current holder. The Logical Device Identity defined by the Edge Compute Device Identity Profile is single-holder: each device holds and presents its own LDI's SVID.
- **Multi-holder logical identities:** the SVID names a logical identity that **MAY** be held and presented by any of several cooperating holders under deployment-specific credential-storage rules. A presented SVID proves the **logical identity**, not which holder is currently presenting it.

The multi-holder pattern is needed wherever multiple cooperating runtime entities act under one logical identity - for example, the nodes of a Kubernetes cluster acting under one cluster-scoped client identity, or future workload identities shared across replicas.

For multi-holder logical identities, the following framework-level rules apply:

- The bootstrap-time binding to a specific original holder (for example, a device SVID used to enroll a cluster-scoped identity) is **bootstrap-time authorization**, not continuous proof of the current holder.
- Failover or rotation among holders **MUST NOT** require contacting MIS while the SVID remains valid.
- Verifiers **MUST** treat the presented SVID as proof of the logical identity only; any per-holder binding (for example, "which node currently holds the credential") is a deployment concern, not part of MIAF identity verification.

A profile that applies the multi-holder pattern **MUST** specify:

- the **binding-subject kind** (for example, cluster), naming the stable referent that the bootstrap-time authorization is bound to;
- credential-storage discipline appropriate to the holder set (for example, cluster-scoped secret storage with access restricted to the active holder);
- any holder-rotation, leader-failover, or holder-revocation rules that apply within the credential's validity window; and
- the additional WFM Client Binding Assertion claims required to convey topology context to MIS. Profiles building on the [WFM Client Identity Profile](../wfm-client-identity-profile.md) **MUST** add the following claims to the v0 claim set defined in [WFM Client Identity Profile §4.5](../wfm-client-identity-profile.md#45-wfm-client-binding-assertion):
  - `topology` — the topology mode this profile applies (for example, `cluster`); **MUST** be unique among multi-holder profiles in this Trust Domain;
  - `bindingSubjectKind` — the kind of stable binding subject the profile uses; and
  - `bindingSubject` — the binding subject value selected by the WFM at issuance time.

Profiles applying this pattern **SHOULD** recommend SVID validity periods that cover expected disconnected-failover windows.

The active MIAF SUP does not itself apply the multi-holder pattern - the Edge Compute Device Identity Profile is single-holder. The pattern is defined here so that downstream profiles (such as the WFM Client Identity Profile cluster topology below) can apply it without re-introducing framework-level concepts.

### 2. WFM Client Cluster Topology

The cluster WFM Client identity is a **multi-holder logical identity** per [Multi-Holder Logical Identities](#1-framework-multi-holder-logical-identities). The framework-level rules defined there apply here without restatement; this section adds only the cluster-specific specializations.

For a multi-node Kubernetes cluster:

- there is exactly one logical WFM Client identity for the cluster's WFM Client relationship;
- initial issuance **MUST** be requested by a current cluster node holding a valid device SVID;
- the **binding-subject kind** is `cluster`, and the binding subject **MUST** be cluster-scoped and stable across leader failover;
- the resulting WFM Client identity is **cluster-scoped**, not node-scoped; and
- the holder set comprises the cluster nodes that may legitimately present the credential under the cluster's local credential-delivery rules.

In the baseline cluster model defined by this SUP:

- the cluster holds one issued WFM Client credential as cluster-scoped secret material (storage discipline detailed in [Cluster credential storage](#cluster-credential-storage));
- failover between nodes does **not** require contacting MIS, provided the issued credential is still valid; and
- the identity remains tied to a device identity only at **initial issuance time**, not as continuous per-request proof of the current holder node.

Deployments that require per-leader holder binding may layer stricter local credential-delivery or delegated-issuer mechanisms on top, but those are outside the scope of this profile.

#### Cluster binding-assertion claims

Cluster topology binding assertions **MUST** set:

- `topology` = `cluster`
- `bindingSubjectKind` = `cluster`
- `bindingSubject` = a cluster-scoped stable identifier (typically a cluster UUID or operator-assigned cluster name) that remains stable across leader failover and node turnover.

#### Cluster credential storage

Per [Multi-Holder Logical Identities](#1-framework-multi-holder-logical-identities), profiles that apply the multi-holder pattern **MUST** specify credential-storage discipline appropriate to the holder set. For clusters under this profile, the cluster-scoped WFM Client credential is security-critical material and the following requirements apply.

Deployments **MUST** store it in cluster-scoped secret storage with access restricted to the active WFM Client instance or leader-designated holder.

Deployments **SHOULD**:

- minimize the lifetime of cluster-scoped WFM Client credentials consistent with disconnected-operation requirements;
- protect access to cluster secret storage with the strongest available platform controls; and
- rotate credentials after suspected compromise or control-plane recovery.

#### Informative workflow

Cluster bootstrap and failover:

1. Cluster nodes enroll individually and obtain device identities.
2. A current leader or designated bootstrap node obtains a WFM Client Binding Assertion for the cluster-scoped binding subject.
3. That node enrolls the cluster-scoped WFM Client identity with MIS.
4. The resulting credential is stored as cluster-scoped secret material.
5. On leader failover, the new leader reuses the same still-valid cluster credential without contacting MIS.

## Alternatives considered (optional)

TBD

## Rejection reason

Not applicable.
