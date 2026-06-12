  # Providers-Based Resource Resolution — Informative Use Cases

  > **Status:** Informative (Non-Normative)
  >
  > This document provides example scenarios illustrating how the Provider framework can be applied across different domains. The examples are intended to demonstrate the flexibility of the framework and are not normative. Individual providers define their own request, output, offer, and allocation schemas through separate provider-specific SUPs.

  ---

  # Introduction

  The Provider framework is designed to solve a broad class of deployment-time capability resolution problems.

  Some providers represent hardware resources such as GPUs, cameras, or fieldbus channels.

  Some providers represent software capabilities such as storage classes.

  Some providers represent platform services such as secret-vaults, ingress, certificates, identities, or DNS names.

  The examples in this document demonstrate how a common Provider lifecycle can be applied across diverse deployment environments:

  ```text
  Provider
      ↓

  ProviderState
      ↓

  ProviderBinding
      ↓

  ProviderResolution
  ```

  ---

  # UC1 — Network Port Allocation

  ## Problem

  Two applications are deployed to the same device.

  Both vendors ship their application with a default HTTP port of `8080`.

  The second application cannot start because the port is already in use.

  ## Without the Provider Framework

  The deployment succeeds.

  The container starts.

  The application fails at runtime with:

  ```text
  bind: address already in use
  ```

  The failure is detected only after deployment.

  ## With the Provider Framework

  The application requests a network port through a ProviderBinding.

  ```yaml
  providerBindings:
    - id: webPort
      provider: provider.margo.org/network/port
  ```

  The Device Agent detects that port `8080` is already allocated.

  Using the selected fallback strategy, it finds an alternative available port.

  ## Outcome

  The deployment succeeds without runtime conflict.

  The found port is reported through ProviderResolution and injected into the application configuration.

  ## Demonstrates

  * Resource allocation
  * Conflict detection
  * Fallback strategies
  * Configuration injection

  ---

  # UC2 — Ingress Hostname Reservation

  ## Problem

  Two applications attempt to use the same hostname.

  ```text
  helloworld.example.com
  ```

  Only one application can own the route.

  ## Without the Provider Framework

  The conflict may only become visible after deployment.

  One application may become unreachable.

  Operator visibility depends on platform-specific behavior.

  ## With the Provider Framework

  The application requests ownership of a hostname.

  ```yaml
  providerBindings:
    - id: ingressRoute
      provider: provider.margo.org/network/ingress
  ```

  The Device Agent validates the request against existing allocations.

  If the hostname is already allocated, resolution fails before deployment begins.

  ## Outcome

  The deployment is rejected before any components are installed.

  The operator retains full control over hostname selection.

  ## Demonstrates

  * Uniqueness constraints
  * Pre-deployment validation
  * Platform service resolution

  ---

  # UC3 — GPU Allocation

  ## Problem

  An AI inference workload requires an NVIDIA GPU with sufficient memory.

  The application vendor cannot know which GPUs are available on a target device.

  ## Without the Provider Framework

  The application hardcodes assumptions about available hardware.

  Deployment succeeds.

  The workload fails when the expected device does not exist.

  ## With the Provider Framework

  The deployment requests a GPU matching specific requirements.

  ```yaml
  providerBindings:
    - id: inferenceGPU
      provider: provider.margo.org/hardware/gpu
  ```

  The Device Agent evaluates the request against the device's advertised ProviderState.

  A suitable GPU is found.

  ## Outcome

  The application receives a valid device-specific gpu details.

  Deployment only proceeds when suitable hardware is available.

  ## Demonstrates

  * Hardware capability matching
  * Exclusive resource allocation
  * Hardware abstraction
  * Device-specific value injection

  ---

  # UC4 — Camera Reservation

  ## Problem

  A machine vision application requires exclusive access to a high-resolution camera.

  Another application is already using the only suitable camera.

  ## Without the Provider Framework

  The application deploys successfully.

  Camera initialization fails at runtime because the device is already in use.

  ## With the Provider Framework

  The deployment requests a camera meeting specific requirements.

  ```yaml
  providerBindings:
    - id: inspectionCamera
      provider: provider.margo.org/hardware/camera
  ```

  The Device Agent evaluates available and allocated cameras.

  No suitable unallocated camera exists.

  ## Outcome

  The deployment is rejected before application startup.

  The operator receives a clear explanation of the conflict.

  ## Demonstrates

  * Exclusive hardware ownership
  * Capability-based selection
  * Runtime failure prevention

  ---

  # UC5 — CAN Bus Channel Allocation

  ## Problem

  Industrial automation applications communicate with field devices through CAN Bus interfaces.

  Multiple applications must not accidentally claim the same channel configuration.

  ## Without the Provider Framework

  Channel allocation becomes implementation-specific.

  Conflict detection varies across vendors.

  ## With the Provider Framework

  A custom CAN Bus provider advertises available channels and supported baud rates.

  Applications request channels through ProviderBindings.

  ```yaml
  providerBindings:
    - id: motorControlBus
      provider: provider.example.com/canbus/channel
  ```

  The Device Agent finds a compatible channel.

  ## Outcome

  Channel becomes discoverable.

  ## Demonstrates

  * Custom providers
  * Vendor extensibility
  * Industrial communication resources

  ---

  # UC6 — Storage Class Selection

  ## Problem

  An application requires persistent storage.

  The application vendor cannot know which storage classes exist on the target platform.

  ## Without the Provider Framework

  Operators manually provide platform-specific values.

  Deployments become less portable.

  ## With the Provider Framework

  The application requests a storage capability.

  ```yaml
  providerBindings:
    - id: appStorage
      provider: provider.margo.org/storage/volume
  ```

  The Device Agent resolves the request using the platform's available storage offerings.

  ## Outcome

  The deployment receives a valid storage configuration without requiring platform-specific knowledge.

  ## Demonstrates

  * Platform capability discovery
  * Deployment portability
  * Configuration abstraction

  ---

  # UC7 — Service Identity Assignment

  ## Problem

  Applications require platform-managed identities to access external systems securely.

  Identity implementations differ between environments.

  ## Without the Provider Framework

  Applications must embed platform-specific identity assumptions.

  Portability suffers.

  ## With the Provider Framework

  The deployment requests an identity.

  ```yaml
  providerBindings:
    - id: telemetryIdentity
      provider: provider.margo.org/security/identity
  ```

  The platform resolves the identity and provides the required credentials or references.

  ## Outcome

  Applications receive environment-specific identities through a consistent framework.

  ## Demonstrates

  * Identity management
  * Security capabilities
  * Platform services

---

  # UC8 — Secret Resolution from AWS Secrets Manager

  ## Problem

  An application requires access to a database password stored in a centralized secret management system.

  The application vendor does not know the secret value when the application is authored, and the secret must not be embedded in the deployment manifest.

  ## Without the Provider Framework

  Operators manually inject secrets into deployment configurations.

  Different environments often use different secret management solutions and integration mechanisms.

  Applications become tied to platform-specific secret handling approaches.

  ## With the Provider Framework

  A fleet-scoped provider is configured to integrate with AWS Secrets Manager.

  The application requests a secret through a ProviderBinding.

  ```yaml
  providerBindings:
    - id: databasePassword

      provider: provider.margo.org/security/secret

      request:
        secretName: production/database/password

      inject:
        - parameter: DB_PASSWORD
          from: result.value
  ```

  The WFM resolves the binding using the configured provider implementation.

  The provider retrieves the secret from AWS Secrets Manager and returns the resolved value.

  ```yaml
  providerResolutions:
    - bindingId: databasePassword

      status: resolved

      result:
        value: "<redacted>"
  ```

  The resolved value is injected into the deployment without requiring the application to understand how or where the secret is stored.

  ## Outcome

  Applications can consume centrally managed secrets through a common mechanism while remaining independent of the underlying secret management system.

  The same application can be deployed in different environments without modification, provided a compatible secret provider is available.

  ## Demonstrates

  * Fleet-scoped providers
  * Resolution performed by the WFM
  * Integration with external systems
  * Non-resource provider functionality
  * Sensitive value injection
  * Environment portability

  ---

  # Key Observation

  Although these examples span multiple industries and deployment models, they all follow the same lifecycle:

  ```text
  Provider
      ↓ defines capability contract

  ProviderState
      ↓ advertises available capabilities

  ProviderBinding
      ↓ requests a capability

  ProviderResolution
      ↓ reports the outcome
  ```

  This common lifecycle allows the Provider framework to support hardware resources, software capabilities, and platform services through a single extensible mechanism.

  Note: If you are looking for allocations via Providers framework, then it's not covered.