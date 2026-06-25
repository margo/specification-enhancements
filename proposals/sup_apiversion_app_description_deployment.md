# Specification Update Proposal

## Owner

@vireshnavalli

## Summary

Retain the `apiVersion` attribute in `ApplicationDescription` and `ApplicationDeployment`
structures. The `apiVersion` in these structures is independent of the Margo's OpenAPI specification
version and API route versioning — it identifies the schema version of the document itself,
following a Kubernetes-style API group versioning convention.

## Reason for proposal

As API routes include version within the URL path, there is debate about the necessity of
explicitly having an `apiVersion` attribute. However, the `apiVersion` in `ApplicationDescription`
and `ApplicationDeployment` serves a distinct purpose — it identifies the **schema version of the
document**, not the API route version. Maintaining it provides:

1. Explicit schema version tracking at the document level — independent of API route versioning
2. Clear distinction between API route version (`/api/v1/`) and document schema version
3. Schema version available without parsing URL paths or OpenAPI spec metadata
4. Alignment with Kubernetes-style API group versioning conventions
5. Enables schema evolution independently of API route changes
6. Addresses specification [Issue #134 - margo/specification](https://github.com/margo/specification/issues/134)

## Requirements alignment acknowledgement

This SUP aligns with the margo specification requirements for clear document schema versioning.
The `apiVersion` attribute in `ApplicationDescription` and `ApplicationDeployment` is explicitly
**not** tied to:
- The OpenAPI specification version (e.g. `1.0.0`)
- The API route version (e.g. `/api/v1/`)

It is an independent document schema version identifier.

**Related Feature(s):** [Issue #134 - margo/specification](https://github.com/margo/specification/issues/134)

## Technical proposal

### apiVersion Values

| Document | apiVersion value |
|---|---|
| `ApplicationDescription` | `margo.org/v1-alpha1` |
| `ApplicationDeployment` | `application.margo.org/v1alpha1` |

These values are **independent** of:
- OpenAPI spec version (`1.0.0`)
- API route version (`/api/v1/`)

---

### Changes to ApplicationDescription

Retain the `apiVersion` field in the `ApplicationDescription` schema with the correct value:

```yaml
ApplicationDescription:
  type: object
  required: [apiVersion, kind]
  properties:
    apiVersion:
      type: string
      description: >
        Schema version of the ApplicationDescription document.
        This is independent of the OpenAPI specification version and API route version.
        MUST be one of the Margo defined stable value.
      enum:
        - margo.org/v1-alpha1
      example: margo.org/v1-alpha1
    kind:
      type: string
      enum: [ApplicationDescription]
    # ... other properties
```

---

### Changes to ApplicationDeployment

Retain the `apiVersion` field in the `ApplicationDeployment` schema with the correct value:

```yaml
ApplicationDeployment:
  type: object
  required: [apiVersion, kind]
  properties:
    apiVersion:
      type: string
      description: >
        Schema version of the ApplicationDeployment document.
        This is independent of the OpenAPI specification version and API route version.
        MUST be one of the Margo defined stable value.
      enum:
        - application.margo.org/v1alpha1
      example: application.margo.org/v1alpha1
    kind:
      type: string
      enum: [ApplicationDeployment]
    # ... other properties
```

---

### Version Progression

The `apiVersion` follows a Kubernetes-style API group versioning convention.
The progression path for breaking changes is:

| Stage | ApplicationDescription | ApplicationDeployment |
|---|---|---|
| Current | `margo.org/v1-alpha1` | `application.margo.org/v1alpha1` |
| Next breaking change | `margo.org/v1-alpha2` | `application.margo.org/v1alpha2` |
| Stable release | `margo.org/v1` | `application.margo.org/v1` |

**Rules:**

- A new `apiVersion` value MUST be introduced for any **significantly breaking** schema change
- Non-breaking additive changes (new optional fields) MAY be made within the same `apiVersion`
- Clients MUST reject documents with an unrecognised `apiVersion`
- Servers MUST NOT serve documents with a deprecated `apiVersion` after its removal date
- Both old and new `apiVersion` values SHOULD be supported simultaneously during a
  transition period of at least one major specification release

> **Note:** The versioning path (e.g. `v1-alpha1` → `v1-alpha2`) is not yet fully defined.
> The exact criteria for what constitutes a "significantly breaking change" requiring a new
> `apiVersion` is an open question to be resolved during SUP review.

---

### Version Independence Clarification

```
OpenAPI spec version:   1.0.0                          ← spec metadata, semver
API route version:      /api/v1/                       ← URL path segment
ApplicationDescription: margo.org/v1-alpha1            ← document schema version
ApplicationDeployment:  application.margo.org/v1alpha1 ← document schema version

These are three independent versioning axes.
A change to the API route version does NOT require a change to apiVersion.
A change to apiVersion does NOT require a change to the API route version.
```

---

## Alternatives considered

1. **Remove apiVersion entirely** — Rejected because it removes explicit schema version
   information from document definitions, making it impossible for consumers to determine
   which schema version a document conforms to without out-of-band information.

2. **Tie apiVersion to OpenAPI spec version (1.0.0)** — Rejected because the document schema
   version and the API specification version are independent concerns. Coupling them would
   force unnecessary document version bumps on unrelated API changes.

3. **Tie apiVersion to API route version (/api/v1/)** — Rejected for the same reason — document
   schema evolution and API route evolution are independent.

4. **Use semver (1.0.0) for apiVersion** — Rejected in favour of Kubernetes-style API group
   versioning (`margo.org/v1-alpha1`) which is more expressive about stability stage
   (alpha/beta/stable) and is familiar to edge/cloud-native implementors.

## Related PRs

- margo/specification PR #189: "chore: retains apiversion in app description and app deployments"