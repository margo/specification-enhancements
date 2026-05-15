# Specification Update Proposal

## Owner

[@matlec](https://github.com/matlec) (currently deferred — owner to be confirmed at promotion)

## Summary

Defines the Device Replacement / rebinding model under MIAF: how a Logical Device Identity (LDI) is rebound to a new Enrollment Subject Identifier (ESI) when device hardware is replaced, factory-reset, or re-credentialed, while preserving the LDI's authorization scope and audit history. Specifies the binding rules, the on-the-wire `replacementAuthorization` enrollment-request field, and one normative replacement-authorization method (`urn:margo:replacement-auth:operator-ticket:v1`). Originally drafted as part of the active [MIAF SUP](../margo-identity-and-authorization-framework.md); deferred for PR 2 since v0 deployments can handle hardware replacement by treating the replaced device as a fresh enrollment with a new LDI.

## Reason for proposal

Hardware replacement and credential reissuance are operationally important across long device lifetimes, but standardizing the rebinding semantics is bound up with wider Device Fleet Management work that is out of scope for PR 2. Deferring the formal model lets v0 ship without it.

## Requirements alignment acknowledgement

This SUP extends the active [MIAF SUP](../margo-identity-and-authorization-framework.md) and aligns with Margo's flexibility, scalability, and security goals. Detailed feature linkage and Owner are TBD at promotion.

## Technical proposal

### 1. Logical Device Identity Replacement: binding rules

Device replacement has two parts: what the MIS is allowed to do (this section) and how that action is authorized (next section).

For devices, the MIS **MUST** enforce the following binding model between Enrollment Subject Identifiers (ESIs) and Logical Device Identities (LDIs):

1. **ESI immutability:** Once an ESI is bound to an LDI, the MIS **MUST NOT** bind that ESI to a different LDI.
2. **LDI replacement support:** An LDI **MAY** have multiple ESIs associated with it over time, but at most one may be **active** at any point.
3. **Single-active bootstrap subject:** An LDI **MUST NOT** have more than one **active** ESI at a time.
4. **Replacement semantics:** Replacement binds a **new** ESI to an **existing** LDI and retires the previously active ESI for that LDI.
5. **Retired ESI handling:** A retired ESI **MUST NOT** be sufficient to obtain new SVIDs for the LDI unless explicitly re-authorized by Trust Domain policy.

An ESI is **active** for an LDI if the MIS will accept enrollment requests based on that bootstrap subject for the purpose of issuing SVIDs for the LDI. An ESI is **retired** if it is no longer accepted for enrollment for that LDI.

If an ESI is not currently bound to an LDI, the MIS **MUST NOT** bind it to an existing LDI unless explicitly authorized by Trust Domain policy.

Replacement authorization is **policy-controlled** and **SHOULD** be auditable.

### 2. Replacement authorization conveyance

A request that attempts replacement / rebinding to an existing identity **MUST** convey the replacement authorization to the MIS in the enrollment request body using a `replacementAuthorization` object.

For baseline interoperability, this SUP defines one replacement-authorization method:

**Replacement Authorization Method Identifier (URN):**
`urn:margo:replacement-auth:operator-ticket:v1`

**`replacementAuthorization` object schema (`application/json`):**

| Field | Type | Required | Description |
| :---- | :--- | :------- | :---------- |
| `method` | string | Y | **MUST** be `urn:margo:replacement-auth:operator-ticket:v1`. |
| `proof` | object | Y | **MUST** contain `ticket`. |
| `proof.ticket` | string | Y | Opaque, single-use, time-bounded replacement ticket issued by operator tooling or another Trust Domain authority authorized by policy. The replacement device **MUST** present this value unchanged. |

**Validation requirements:**

- The MIS **MUST** validate the ticket before performing replacement / rebinding to an existing identity.
- A valid ticket **MUST** authorize binding the enrollment request's derived ESI to a specific existing LDI.
- The ticket **MUST** be single-use and time-bounded; the MIS **MUST** reject expired or replayed tickets.
- The ticket **MUST** be auditable and **MAY** additionally bind the expected ESI or other non-secret enrollment metadata extracted from the request.
- The exact encoding of the ticket is deployment-specific and opaque to the device; interoperability is achieved through the standard `replacementAuthorization` field, method identifier, and validation semantics defined here.
- If a replacement / rebinding request omits `replacementAuthorization`, uses an unsupported replacement authorization method, or presents a ticket that does not authorize the requested rebinding, the MIS **MUST** reject the request.

> **Informative:**
> The workflow by which an operator or an existing device obtains a valid replacement ticket remains deployment-specific. Deployments may realize this through planned handover, fleet tooling, or human-in-the-loop approval, but the on-the-wire conveyance to the MIS is standardized by this section.

### 3. Lifecycle integration

The active [MIAF SUP](../margo-identity-and-authorization-framework.md) defines the Logical Device Identity lifecycle in v0 with phases **enrollment**, **active**, **renewal**, and **revocation / termination**. This SUP introduces a **Replacement** phase between **active** and **revocation / termination**. The lifecycle table in the active MIAF SUP would gain a row for Replacement that describes binding a new method-derived ESI to the existing LDI and retiring the previously active ESI. The lifecycle Mermaid diagram would gain a `REP` node with an edge `ACT -->|Authorized rebinding (hardware replacement, credential refresh, operator-initiated)| REP` and a return edge `REP -->|Rebinding complete| ACT`. The remainder of the lifecycle (enrollment → active, renewal loop, revocation terminal path) is unchanged.

## Alternatives considered

TBD

## Rejection reason

Not applicable.
