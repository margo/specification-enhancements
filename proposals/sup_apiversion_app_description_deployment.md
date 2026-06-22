# Specification Update Proposal

## Owner

@vireshnavalli

## Summary

Retain the `apiVersion` attribute in `ApplicationDescription` and `ApplicationDeployment` structures. Although API routes include the version within the URL path, keeping the apiVersion in these structures provides explicit version information and ensures consistency with semantic versioning requirements.

## Reason for proposal

As API routes include version within URL, there is debate about the necessity of explicitly having 'apiVersion' attribute for margo specified REST APIs. However, maintaining the apiVersion in ApplicationDescription and ApplicationDeployment provides:

1. Explicit version tracking at the application definition level
2. Clear semantic versioning compliance
3. Version information available without parsing URL paths
4. Better consistency across specification structures
5. Addresses specification issue #134

## Requirements alignment acknowledgement

This SUP aligns with the margo specification requirements for clear API versioning and semantic versioning practices. The apiVersion attribute ensures that application descriptions and deployments carry explicit version information, improving clarity and consistency across the specification.

**Related Feature(s):** [Issue #134 - margo/specification](https://github.com/margo/specification/issues/134)

## Technical proposal

### Changes to ApplicationDescription

Retain the `apiVersion` field in the `ApplicationDescription` schema:

```yaml
ApplicationDescription:
  type: object
  properties:
    apiVersion:
      type: string
      description: Semantic version of the application API specification
      example: "1.0.0"
    # ... other properties
```

### Changes to ApplicationDeployment

Retain the `apiVersion` field in the `ApplicationDeployment` schema:

```yaml
ApplicationDeployment:
  type: object
  properties:
    apiVersion:
      type: string
      description: Semantic version of the application API specification
      example: "1.0.0"
    # ... other properties
```

### Version Consistency

- The `apiVersion` value must match the OpenAPI specification version
- Must follow semantic versioning format (MAJOR.MINOR.PATCH)
- Should be maintained in sync with API route versioning

## Alternatives considered

1. **Remove apiVersion entirely** - Rejected because it removes explicit version information from application definitions (application description) and application deployment manifest, making it harder to track versioning at the application description/deployment manifests. Remove apiVersion from the body of the API requests as it is part of the API route, keep the apiVersion in ApplicationDescription (and ApplicationDeployment)


## Related PRs

- margo/specification PR #189: "chore: retains apiversion in app description and app deployments"
