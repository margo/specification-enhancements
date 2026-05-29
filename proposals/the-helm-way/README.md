# Margo WG -- Specification Enhancement Proposals

| Field | Value |
|-------|-------|
| **Package date** | 2026-05-07 |
| **Prepared by** | Andrii Melashchenko (Belden Inc.) |
| **Status** | Ready for public review |
| **Applies to** | Margo specification pre-draft |

---

## Documents

| File | Title | Type | Status |
|------|-------|------|--------|
| `WG-PROPOSAL-00_the-helm-way.md` | The Helm Way -- Canonical OCI Component Publishing Pattern | Informational | Rev 1 |
| `WG-PROPOSAL-01_compose-oci-and-structure.md` | Compose OCI Registry Publishing and Archive Structure | Cat 2 normative | Rev 1 |
| `WG-PROPOSAL-02_quadlet-component-type.md` | Quadlet Component Type | Cat 2 normative | Rev 1 |
| `WG-PROPOSAL-03_apiversion-evolution-strategy.md` | API Version Evolution Strategy | Cat 2 normative | Rev 2 |
| `PATCH_desired-state-quadlet.diff` | Desired-state schema patch for Quadlet | Companion artifact | Ready to apply |

---

## Summary of Changes

**P-00 -- The Helm Way** (informational, no spec diffs): Names the five-element OCI publishing pattern already implemented by Helm -- Storage, Reference, Media Types, Integrity, Parametrization. Serves as the reference all other proposals point to.

**P-01 -- Compose OCI** (normative): Mandates OCI registry storage for Compose Archives. Introduces `compose.v1` deployment profile type, two new media types, normative archive structure, and deprecates `packageLocation`.

**P-02 -- Quadlet** (normative): Introduces Quadlet as a third component type (`quadlet.v1`) for native systemd container management on OT edge devices. Defines schema classes, media types, and archive structure.

**P-03 -- API Version Evolution** (normative): Adds `pattern` constraints to `apiVersion` in both `ApplicationDescription` and `DesiredState` schemas. Introduces a normative version catalogue, lifecycle states (Alpha → Beta → Stable → Deprecated → Removed), WFM rejection/warning behaviour, and compatibility rules.

---

## Application Order

```
P-00 (informational)  -->  P-01 (Compose)  -->  P-02 (Quadlet)

P-03 (API Version) -- independent, may be applied at any point
```

P-01 must be applied before P-02. P-03 is independent of P-01 and P-02.

---

*Subject to the Open Web Foundation Contributor License Agreement governing the Margo specification.*
