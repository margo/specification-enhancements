## Desired State Manifest Changes

The Desired State Manifest (`UnsignedAppStateManifest`) itself requires no
structural changes. The `ApplicationDeployment` documents it references now
carry the `discoverCapabilities` block for device-scoped capabilities.

The Device Agent's processing order when it pulls a new Desired State Manifest:

```text
1. Pull Desired State Manifest
   GET /api/v1/clients/{clientId}/deployments

2. For each ApplicationDeployment in the manifest:

   a. No discoverCapabilities present:
      → Apply directly

   b. Fleet-scoped capabilities only:
      → All valueFrom references already resolved to value by WFM
      → Apply directly

   c. Device-scoped capabilities present:
      → For each entry in discoverCapabilities:
          Evaluate request against local CapabilityState
          On success: write resolved value into parameters
          On failure: abort — do NOT apply this deployment
                      Report CapabilityResolution with failureCode via
                      POST /api/v1/clients/{clientId}/deployments/{deploymentId}/status
      → If all resolved: apply deployment

3. After applying:
   → Resend affected CapabilityState documents to WFM to reflect
     any state changes caused by resolution
     POST /api/v1/clients/{clientId}/devices/{deviceId}/capabilities

4. Report deployment status as usual
```

**Rules:**
- The Device Agent MUST evaluate all `discoverCapabilities` entries before
  applying any deployment that contains device-scoped capabilities
- The Device Agent MUST report a `CapabilityResolution` for every entry in
  `discoverCapabilities` it processes — both success and failure
- The Device Agent MUST resend updated `CapabilityState` documents after
  resolution so the WFM's view of device state stays current
- The Device Agent MUST NOT partially apply a deployment — if any capability
  resolution fails, the entire deployment is rejected