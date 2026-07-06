# Use Case — Secret Resolution from a Vault

> **Status:** Informative (Non-Normative)

## Problem

An application requires a database password to connect to a production database.

The password is stored in a centralized secret management system — HashiCorp
Vault, AWS Secrets Manager, Azure Key Vault, or any equivalent platform service.

The application vendor does not know the secret value when authoring the
application. The secret must not be embedded in the deployment manifest. The
secret management system in use differs between environments.

---

## Without the Capability Framework

The operator manually retrieves the secret from the vault and injects it into
the deployment manifest as a plaintext parameter value.

This means:

- The secret travels through the deployment pipeline in plaintext
- The operator must have direct access to the vault
- The deployment manifest becomes environment-specific — it cannot be reused
  across staging and production without manual modification
- There is no audit trail connecting the secret consumption to the deployment
  that consumed it
- Rotation requires the operator to manually update the deployment

---

## With the Capability Framework

A fleet-scoped `CapabilityDefinition` is authored for vault secret resolution.
The WFM is configured with a vault integration — the specific vault backend is
an implementation detail of the WFM vendor, not the specification.

The application operator declares a `CapabilityDiscoveryRequest` in the
`ApplicationDeployment`. The WFM resolves it at deployment trigger time and
injects the resolved value into the deployment parameters before the manifest
reaches the device.

The device never sees the secret reference — it receives only the resolved value.

---

## Step 1 — TWG Capability Author publishes the `CapabilityDefinition`

```yaml
apiVersion: margo.org/v1alpha1
kind: CapabilityDefinition

metadata:
  id: capability.margo.org/security/secret

spec:
  scope: fleet                    # resolved by WFM — device is not involved

  description: |
    Resolves a named secret from a platform-managed secret store.
    The specific vault backend is implementation-defined by the WFM vendor.
    Supports any secret addressable by a name or path within the configured store.

  sourceState:
    schema:
      type: object
      properties:
        available:
          type: boolean
          description: Whether the vault backend is reachable and operational
        backend:
          type: string
          description: >
            The type of vault backend configured on this WFM instance.
            Informative only — e.g. hashicorp-vault, aws-secrets-manager,
            azure-key-vault.

  discovery:
    requestSchema:
      type: object
      required: [secretName]
      properties:
        secretName:
          type: string
          description: >
            The name or path of the secret in the vault.
            Format is backend-specific — e.g. production/database/password
            for AWS Secrets Manager, or secret/data/db/password for
            HashiCorp Vault KV v2.
        version:
          type: string
          description: >
            Optional. The version of the secret to retrieve.
            If omitted, the latest version is returned.
        field:
          type: string
          description: >
            Optional. If the secret is a structured object (e.g. JSON),
            the specific field to extract. If omitted, the entire secret
            value is returned.

    outputSchema:
      type: object
      properties:
        value:
          type: string
          description: >
            The resolved secret value. Treated as sensitive — WFM
            implementations SHOULD ensure this value is not logged
            or persisted in plaintext.
        resolvedVersion:
          type: string
          description: The version of the secret that was resolved.

    failureCodes:
      - SecretNotFound
      - VaultUnavailable
      - AccessDenied
      - InvalidSecretPath
      - VersionNotFound
```

---

## Step 2 — WFM publishes `CapabilityState`

The WFM publishes the state of its vault integration. This is fleet-scoped —
the device does not publish this state.

```json
{
  "apiVersion": "margo.org/v1alpha1",
  "kind": "CapabilityState",
  "metadata": {
    "capability": "capability.margo.org/security/secret"
  },
  "spec": {
    "available": true,
    "backend": "hashicorp-vault"
  }
}
```

---

## Step 3 — Application Vendor authors `ApplicationDescription`

The application vendor declares the parameters the application needs. They
have no knowledge of the vault, the secret path, or the environment. They
only declare that a database password is required.

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDescription
id: com.example.payments-service
metadata:
  name: Payments Service
  version: 3.2.0

parameters:
  dbPassword:
    value: ""                     # no meaningful default — must be supplied
    targets:
      - pointer: env.DB_PASSWORD
        components: ["payments-api"]
  dbHost:
    value: "localhost"
    targets:
      - pointer: env.DB_HOST
        components: ["payments-api"]

deploymentProfiles:
  - type: helm.v3
    components:
      - name: payments-api
        properties:
          repository: oci://registry.example.com/charts/payments-service
          revision: 3.2.0
```

The application vendor does not reference the vault. They do not know which
vault backend the operator uses. They only declare that `dbPassword` is a
parameter the application needs.

---

## Step 4 — Operator authors `ApplicationDeployment`

The operator knows the target environment and the vault path for the secret.
They wire the vault capability output into the `dbPassword` parameter using
`valueFrom`.

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDeployment
id: deployment-payments-service-prod-01
metadata:
  deviceId: device-edge-prod-001

spec:
  applicationId: com.example.payments-service
  discoverCapabilities:
    databaseSecret:                                     # unique key within this deployment
      id: capability.margo.org/security/secret          # references CapabilityDefinition URI
      request:                                          # conforms to discovery.requestSchema
        secretName: production/payments/db-password
        field: password

  parameters:
    dbPassword:
      valueFrom: discoverCapabilities.databaseSecret.output.value
      targets:
        - pointer: env.DB_PASSWORD
          components: ["payments-api"]
    dbHost:
      value: "db.prod.internal"
      targets:
        - pointer: env.DB_HOST
          components: ["payments-api"]

  deploymentProfile:
    type: helm.v3
    components:
      - name: payments-api
        properties:
          repository: oci://registry.example.com/charts/payments-service
          revision: 3.2.0
```

---

## Step 5 — WFM resolves the capability

When the deployment is triggered, the WFM:

1. Looks up `capability.margo.org/security/secret` in the Capability Registry
2. Confirms scope is `fleet` — WFM resolves this directly, device is not involved
3. Validates the `request` against `discovery.requestSchema`
4. Confirms `valueFrom` references valid fields in `discovery.outputSchema`
5. Connects to the configured vault backend (HashiCorp Vault in this environment)
6. Retrieves `production/payments/db-password`, extracts field `password`

**Success:**
```yaml
capabilityResolution:
  deploymentId: deployment-payments-service-prod-01
  bindingName: databaseSecret
  status: Success
  output:
    value: "s3cr3t-db-p@ssw0rd"       # resolved from vault
    resolvedVersion: "v7"
```

**Failure — secret not found:**
```yaml
capabilityResolution:
  deploymentId: deployment-payments-service-prod-01
  bindingName: databaseSecret
  status: Failure
  failureCode: SecretNotFound
  message: >
    No secret found at path 'production/payments/db-password'
    in the configured HashiCorp Vault backend.
    Verify the secret path and that the WFM service account has read access.
```

If resolution fails, the WFM blocks the deployment entirely and surfaces the
`failureCode` to the operator. The device never receives the deployment.

---

## Step 6 — WFM injects resolved value and dispatches deployment

The WFM writes the resolved secret value into `parameters.dbPassword.value`,
replacing the `valueFrom` reference. The `discoverCapabilities` block is
stripped — the device does not need it since this was fleet-scoped.

The deployment the device receives:

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDeployment
id: deployment-payments-service-prod-01
metadata:
  deviceId: device-edge-prod-001
  applicationId: com.example.payments-service

spec:
  parameters:
    dbPassword:
      value: "s3cr3t-db-p@ssw0rd"     # injected by WFM — resolved from vault
      targets:
        - pointer: env.DB_PASSWORD
          components: ["payments-api"]
    dbHost:
      value: "db.prod.internal"
      targets:
        - pointer: env.DB_HOST
          components: ["payments-api"]

  deploymentProfile:
    type: helm.v3
    components:
      - name: payments-api
        properties:
          repository: oci://registry.example.com/charts/payments-service
          revision: 3.2.0
```

The existing `targets` mechanism carries `dbPassword.value` into
`env.DB_PASSWORD` on the `payments-api` component. No changes to the
component layer are required.

---

## What This Demonstrates

| Concern | How it is addressed |
|---|---|
| Secret never in manifest at authoring time | `valueFrom` is a reference, not a value |
| Operator does not manually handle secrets | WFM resolves from vault directly |
| Application is environment-agnostic | Vault path is in `ApplicationDeployment`, not `ApplicationDescription` |
| Device never sees the secret reference | Fleet-scoped — WFM resolves before dispatch |
| Failure is caught before deployment | WFM blocks on `SecretNotFound` or `VaultUnavailable` |
| Different vault backends per environment | WFM implementation detail — spec does not mandate a backend |
| Secret rotation | Re-trigger deployment — WFM fetches latest version |

---

## Key Architectural Point — Fleet-Scoped Resolution

This use case demonstrates that the Capability Framework is not only for
hardware resources. The `scope: fleet` designation means the WFM is the
resolving actor — the device is never involved in the resolution.

This makes fleet-scoped capabilities the natural home for:

- Secret and credential resolution
- DNS name assignment
- Certificate issuance
- Identity provisioning
- Any platform service whose state is managed by the WFM, not the device

The device receives a fully resolved deployment. It has no knowledge that
a vault was consulted, no dependency on the vault backend, and no
credentials stored in its local state.