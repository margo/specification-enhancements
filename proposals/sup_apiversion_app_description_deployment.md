# Specification Update Proposal

## Owner

@vireshnavalli

## Summary

Retain the `apiVersion` attribute in `ApplicationDescription` and `ApplicationDeployment`
structures. The `apiVersion` in these structures is independent of the Margo's OpenAPI specification — it identifies the version of the contract the document satisfies.

## Reason for proposal

As API routes include structure contract versions within the URL path, there is debate about the necessity of
explicitly having an `apiVersion` attribute. Since there are no URL endpoints for the `ApplicationDescription` and `ApplicationDeployment` structures, the `apiVersion` serves to identify the ** structure contract version of the
document satisfies**, not the OpenAPI specification version. Maintaining it provides:

1. Explicit version tracking at the document level — since there is no URL endpoint to indicate the structure contract version.
2. Clear distinction between specification version (1.0.0) and structure contract version (v1, v2, v2alpha1)
3. The structure's contract version can be determined directly from the document since no URL is available
4. Alignment with Kubernetes-style API group versioning conventions
5. Enables document evolution independently of API and other specification changes
6. Addresses specification [Issue #134 - margo/specification](https://github.com/margo/specification/issues/134)

## Requirements alignment acknowledgement

This SUP aligns with the Margo specification requirements for clear contract versioning.
The `apiVersion` attribute in `ApplicationDescription` and `ApplicationDeployment` is explicitly
**not** tied to:
- The OpenAPI specification version (e.g. `1.0.0`)
- The API route version (e.g. `/api/v1/`)

It is an independent document  version identifier.

**Related Feature(s):** [Issue #134 - margo/specification](https://github.com/margo/specification/issues/134)

## Technical proposal

### apiVersion Values

| Document | apiVersion value |
|---|---|
| `ApplicationDescription` | `v1alpha1` |
| `ApplicationDeployment` | `v1alpha1` |

These values are **independent** of:
- OpenAPI spec version (`1.0.0`)
- API route version (`/api/v1/`)

---

### Changes to ApplicationDescription

Retain the `apiVersion` field in the `ApplicationDescription`  with the correct value:

```yaml
ApplicationDescription:
  type: object
  required: [apiVersion, kind]
  properties:
    apiVersion:
      type: string
      description: >
         version of the ApplicationDescription document.
        This is independent of the OpenAPI specification version and API route version.
        MUST be one of the Margo defined stable value.
      enum:
        - v1alpha1
      example: v1alpha1
    kind:
      type: string
      enum: [ApplicationDescription]
    # ... other properties
```

---

### Changes to ApplicationDeployment

Retain the `apiVersion` field in the `ApplicationDeployment`  with the correct value:

```yaml
ApplicationDeployment:
  type: object
  required: [apiVersion, kind]
  properties:
    apiVersion:
      type: string
      description: >
         version of the ApplicationDeployment document.
        This is independent of the OpenAPI specification version and API route version.
        MUST be one of the Margo defined stable value.
      enum:
        - v1alpha1
      example: v1alpha1
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
| Current | `v1alpha1` | `v1alpha1` |
| Next breaking change | `v1alpha2` | `v1alpha2` |
| Stable release | `v1` | `v1` |

**Rules:**

- A new `apiVersion` value MUST be introduced for any **significantly breaking**  change
- Non-breaking additive changes (new optional fields) MAY be made within the same `apiVersion`
- Clients MUST reject documents with an unrecognised `apiVersion`
- Servers MUST NOT serve documents with a deprecated `apiVersion` after its removal date
- Both old and new `apiVersion` values SHOULD be supported simultaneously during a
  transition period of at least one major specification release

> **Note:** The versioning path (e.g. `v1alpha1` → `v1alpha2`) is not yet fully defined.
> The exact criteria for what constitutes a "significantly breaking change" requiring a new
> `apiVersion` is an open question to be resolved during SUP review.

---

### Version Independence Clarification

```
OpenAPI spec version:   1.0.0    ← spec metadata, semver
API route version:      /api/v1/ ← structure contract version via URL path segment
ApplicationDescription: v1alpha1 ← structure contract version via document
ApplicationDeployment:  v1alpha1 ← structure contract version via document

These are three independent versioning axes.
A change to the API route version does NOT require a change to apiVersion.
A change to apiVersion does NOT require a change to the API route version.
```

---

## Alternatives considered

1. **Remove apiVersion entirely** — Rejected because it removes explicit  version
   information from document definitions, making it impossible for consumers to determine
   which  version a document conforms to without out-of-band information.

2. **Tie apiVersion to OpenAPI spec version (1.0.0)** — Rejected because the document 
   version and the API specification version are independent concerns. Coupling them would
   force unnecessary document version bumps on unrelated API changes.

3. **Tie apiVersion to API route version (/api/v1/)** — Rejected for the same reason — document
    evolution and API route evolution are independent.

4. **Use semver (1.0.0) for apiVersion** — Rejected in favour of Kubernetes-style API group
   versioning (`v1alpha1`) which is more expressive about stability stage
   (alpha/beta/stable) and is familiar to edge/cloud-native implementors.

## Related PRs

- margo/specification PR #189: "chore: retains apiversion in app description and app deployments"