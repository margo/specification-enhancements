# Specification Update Proposal: Read receipts for desired state

- [Specification Update Proposal: Read receipts for desired state](#specification-update-proposal-read-receipts-for-desired-state)
  - [Owner](#owner)
  - [Summary](#summary)
  - [Reason for proposal](#reason-for-proposal)
  - [Requirements alignment acknowledgement](#requirements-alignment-acknowledgement)
  - [Technical proposal](#technical-proposal)
    - [New field on `DeploymentStatusManifest`](#new-field-on-deploymentstatusmanifest)
    - [Client requirement](#client-requirement)
    - [WFM interpretation (informative)](#wfm-interpretation-informative)
    - [Specification changes](#specification-changes)
    - [Example](#example)
  - [Alternatives considered](#alternatives-considered)
  - [Rejection reason](#rejection-reason)

## Owner

[@matlec](https://github.com/matlec)

## Summary

Today the WFM publishes a desired state but never hears
back which revision a client actually took up. This SUP adds one field to the
`DeploymentStatusManifest` that the WFM client sends the Workload Fleet Manager (WFM), so that
each status report says which desired state it is reporting on:

- `observedManifestVersion`: the `manifestVersion` of the most recent state manifest the WFM
  client has reconciled this deployment against.

It lets the WFM tell, for each deployment, whether the WFM client has picked up the current
desired state. Manifest versions increase monotonically and never repeat, so the WFM can place
any report on its own publication timeline, even when a deployment's content reverts to an
earlier revision.

Today the status manifest reports lifecycle state (`installing`, `installed`, `failed`, ...)
but never says which revision of the desired state that state refers to, so the WFM can only
guess whether a client has picked up a change.

## Reason for proposal

The Deployment Status API is the WFM client's only channel for reporting deployment progress,
and it is missing the information needed to close the loop on a change.

The `DeploymentStatusManifest` carries `deploymentId`, an overall `status.state`, and a `state`
for each component, but nothing that ties a reported state to a specific revision of the
`ApplicationDeployment`. So a stale `installed` looks the same as a converged one. When the WFM
changes a deployment (for example a new component `revision`, or a parameter change), the
`ApplicationDeployment` changes and the WFM publishes a new state manifest. The WFM client's
last status report still says `installed`, for the previous content. The report carries no
version, so the WFM cannot bind it to a revision. Waiting for the next report does not help:
there is no defined reporting cadence and no contract to report on change, so a fresh
`installed` with no version is as ambiguous as the last one.

A partial signal exists today, but it is coarse. The state manifest envelope carries a
`manifestVersion`, a counter the WFM increases with each manifest it publishes to a client, for
rollback protection. A WFM can infer that the client has fetched a manifest at or above version
N, but the client never reports this back, and it says nothing about which revision the client
adopted for a given deployment. This SUP turns that guess into a value the client reports for
each deployment.

The value is already available to the WFM client: it holds the `manifestVersion` from the
manifest it fetched. This SUP asks the client to record, next to each deployment it reconciles,
the version of the manifest it reconciled against, and to report it. No new computation and no
extra fetch.

## Requirements alignment acknowledgement

This SUP contributes to the TWG Feature [#156](https://github.com/margo/specification/issues/156).

It addresses one part of #156: letting a WFM observe which revision of the desired state a
client is working from, so the WFM can tell whether a client has picked up a change. It also
makes normative the identity rule the field depends on (see the Technical proposal): a content
change keeps the same `deploymentId` and yields a new digest.

What this SUP changes in the specification:

- `DeploymentStatusManifest` gains one field, `observedManifestVersion`.
- It reuses a value already defined: the `manifestVersion` on the State Manifest.

Out of scope, including the parts of #156 left to other SUP(s):

- When a client reports (on a state change, on a heartbeat, or both).
- Retry, backoff, maximum report age, and how a WFM deduplicates repeated reports.
- Behavior during long disconnects.
- What a client does on failure (stop and report, or roll back, and whether it must retry a
  component apply). This SUP reports the version the client adopted and the outcome; it does not
  mandate the reaction.
- What a client reports for a deployment it never obtained from the desired state, for example
  a fetch that never succeeded. This SUP covers deployments the client has taken up.

## Technical proposal

This SUP makes one identity rule normative that the specification currently only implies: when
the WFM changes the content of an existing deployment (its parameters or a component
`revision`), the `deploymentId` stays the same and the digest changes; a different
`deploymentId` means a different deployment. The `{deploymentId}/{digest}` fetch endpoint
already assumes this. The WFM relies on the digest changing to know a deployment's target
changed, and the WFM client relies on it to know it must fetch a new revision. The field this
SUP adds is scoped to a single `deploymentId` and does not depend on when a WFM chooses to mint
a new `deploymentId` instead of updating in place; that policy stays with #156 and needs a
separate SUP.

### New field on `DeploymentStatusManifest`

Add one required field, `observedManifestVersion`. It is required, so this is a breaking change
to the status report.

| Field | Type | Required? | Description |
|-------|------|-----------|-------------|
| observedManifestVersion | number | Y | The `manifestVersion` (see [State Manifest](https://docs.margo.org/specification/margo-management-interface/desired-state#endpoints---state-manifest)) of the most recent state manifest the WFM client has reconciled this deployment against: the latest manifest whose revision of the deployment the client holds and is applying or running. Manifest versions are monotonic and never repeat across a client's manifest history, so the WFM can order this report against the versions it has published. |

Semantics:

- `observedManifestVersion = N` means the client is running the deployment as manifest N defined
  it, so the version it reports MUST match the revision it actually holds. A client still on an
  older revision reports that older version, and the WFM sees it has not caught up.
- Fetching a newer manifest does not by itself advance `observedManifestVersion` for a
  deployment. Until the client begins applying that deployment's newer revision, it keeps
  reporting the version of the revision it still holds and is applying or running.
- The reported version need not be the one at which the deployment last changed. A client that
  has lost its local state and resynchronized reports the version it resynchronized against,
  which can be later. The WFM treats any version at or above its own target for the deployment
  as current (see WFM interpretation).
- The field says which desired state the report is about; it does not claim the deployment
  applied successfully. Whether application succeeded is carried by `status.state`, as it is
  today. A `failed` status with an `observedManifestVersion` at or above the WFM's target means
  the client took up the current desired state and failed applying it.
- The field is present in every status report. A client learns of a deployment from a versioned
  state manifest, so it always has a version to report for a deployment it has taken up.

### Client requirement

A conformant client MUST populate `observedManifestVersion` with the version of the most recent
state manifest it has reconciled the reported deployment against. The reported version MUST
match the revision of the deployment the client holds.

### WFM interpretation (informative)

A `manifestVersion` counts publications for the whole manifest, so a new version does not
necessarily change a given deployment's target. For each deployment, the WFM records the version
at which it last changed that deployment's target (call it `targetVersion`). It decides whether
the WFM client is running its current intent by comparing the reported `observedManifestVersion`
to `targetVersion`, together with `status.state`:

| `observedManifestVersion` vs. `targetVersion` | `status.state` | Interpretation |
|---|---|---|
| at or above | `installed` | WFM client is running the current desired state |
| at or above | `installing` / `pending` | WFM client has taken up the current desired state and is applying it |
| at or above | `failed` | WFM client took up the current desired state and failed applying it (the `state` on each component localizes it) |
| below | `installed` | WFM client is still running a previous desired state; it has not yet picked up the change |
| below | `installing` / `pending` | WFM client is still applying a previous revision and has not yet started on the current one |
| below | `failed` | WFM client failed on a previous desired state while a newer one is published |

Because manifest versions never repeat, this comparison stays correct across reverts. After
`D1 -> D2 -> D1` the current target's version is the third publication. A client still running
`D2` cannot report the third version, because that version's revision (`D1`) is not what it
holds; it reports `D2`'s version, which is below `targetVersion`, so the WFM reads it as stale
rather than converged, even though the current content matches `D1` again. Once the client takes
up the re-published `D1`, it reports the third version and the WFM reads it as current.

The same rule handles newer content that arrives while an earlier apply is still running.
Suppose manifest `N` sets a deployment's revision, the client starts applying it, and manifest
`N+1` changes that deployment again before the first apply finishes. Having fetched `N+1` does
not advance the reported version: the client still holds and is applying `N`'s revision, so it
reports `N` with `installing` until it actually begins applying `N+1`'s revision, at which point
it reports `N+1`. The WFM therefore always knows which revision an `installing` refers to.

The WFM client is required only to report the version it took up and the outcome. Deriving these
states is the WFM's responsibility.

### Specification changes

- `workload-management-api-1.0.0.yaml`: add `observedManifestVersion` (a reference to the
  `ManifestVersion` schema) to the `DeploymentStatusManifest` schema and to its `required` list.
- `deployment-status.md`: add `observedManifestVersion` to the Request Body Attributes table
  with the semantics above, and update the example manifest.
- `desired-state.linkml.yaml`: state on the `id` field that a content change keeps the same
  `deploymentId` and yields a new digest, and that a different `deploymentId` is a different
  deployment.

### Example

```json
{
    "apiVersion": "deployment.margo.org/v1alpha1",
    "kind": "DeploymentStatusManifest",
    "deploymentId": "a3e2f5dc-912e-494f-8395-52cf3769bc06",
    "deviceId": "plant-omega-zone1-edge01",
    "observedManifestVersion": 7,
    "status": {
        "state": "installed"
    },
    "components": [
        { "name": "digitron-orchestrator", "state": "installed" },
        { "name": "database-services", "state": "installed" }
    ]
}
```

## Alternatives considered

- **Carry a new monotonic generation number in the desired state document.** A monotonic
  publication counter already exists as `manifestVersion`, which the WFM client already holds
  from the manifest fetch. Adding a separate generation number to the `ApplicationDeployment`
  would be a change on both the desired state and status sides to convey information that is
  already on the wire. That adds surface area for no gain over the existing version.

## Rejection reason

> If a SUP is rejected, indicate the reason why it was rejected.
> 
> Complete if SUP is rejected at Phase 2: Proposal Creation or Phase 4: Final Decision 
