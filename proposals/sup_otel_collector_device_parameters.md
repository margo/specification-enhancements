# Move OTEL Configuration to Device-Specific Parameters

## Owner

@phil-abb

## Summary

This proposal changes how an application receives the OpenTelemetry Collector configuration associated with its device. Instead of devices injecting the collector configuration into each workload using environment variables, as is currently defined, an application supplier uses device-specific parameters in their Application Description to request the desired configuration information.

## Reason for proposal

The [current Margo observability guidance](https://docs.margo.org/specification/observability/collecting-workload-observability-data#connecting-to-the-opentelemetry-collector) describes connecting workloads to an OpenTelemetry Collector through environment-variable configuration. This device-wide injection approach makes the setting available to all workloads, even if they do not use it. Also, this requires device suppliers to modify workload files or deployment artifacts to inject these environment variables.

Using the device-specific parameter mechanism from the [approved device-specific parameter values proposal](https://github.com/margo/specification-enhancements/blob/main/proposals/device-specific-parameter-values.md) provides a consistent way to make device-known values available to applications:

* The application supplier declares the parameter and identifies the component and target that need it. Workloads that do not request the parameter do not receive it through this mechanism.
* Device-specific values use the same parameter-resolution model as other device-provided configuration, rather than a separate, observability-only injection mechanism.
* The device supplier reports the value as device install context. The device does not need to rewrite an application's files or deployment artifact to add the setting.

The value remains application configuration: the proposal does not change how an OpenTelemetry SDK reads its configuration inside a workload. It only changes how Margo resolves and delivers the device-specific collector configuration to the workloads.

## Requirements alignment acknowledgement

This proposal applies the existing device-specific parameter mechanism to workload observability configuration. It is aligned with the approved [device-specific parameter values proposal](https://github.com/margo/specification-enhancements/blob/main/proposals/device-specific-parameter-values.md) and the existing [Margo guidance for connecting workloads to the OpenTelemetry Collector](https://docs.margo.org/specification/observability/collecting-workload-observability-data#connecting-to-the-opentelemetry-collector).

Since the Observability information has been in the specification from the very beginning there was never child epics created around the observability functionality. This proposal is not introducing any new functionality, only changing the existing implementation details.

The scope of this proposal is limited to supplying the device-specific collector configuration information to application components that request it. It does not make any other specification changes for Observability.

## Technical proposal

### Application Description

An application that needs the device's OpenTelemetry Collector configuration declares a parameter whose `valueFrom.device.key` identifies the device-supplied value. The parameter should target only the component(s) that need the configuration. The target uses the existing parameter-targeting rules for the selected deployment profile

### Resolution and delivery

Before invoking the deployment provider, the Workload Fleet Management Client MUST resolve each declared device-sourced OTEL configuration parameter according to the device-specific parameter resolution rules. It MUST deliver the resolved value only to the parameter target(s) named by the application. If a required value cannot be resolved and no applicable fallback is defined by the device-specific parameter rules, installation or update MUST fail with a clear resolution error.

### Parameters

The following parameters would be used to replace the environment variables:

| Old Environment Variable | New Parameters | Required in App Description | Required by Device | Description |
|----|----|----|----|----|
| HTTP_OTEL_EXPORTER_OTLP_ENDPOINT | margo.otel.endpoint.http | Y | Y | The URL for the workload to use to connect to the OpenTelemetry collector HTTP + protobuf. Application must use this parameter to get the endpoint and use it as a fallback endpoint if they wish to request the gRPC endpoint. |
| GRPC_OTEL_EXPORTER_OTLP_ENDPOINT | margo.otel.endpoint.grpc | N | N | The URL for the workload to use to connect to the OpenTelemetry collector using gRPC. Should be empty if the device does not support gRPC. |
| OTEL_EXPORTER_OTLP_CERTIFICATE | margo.otel.certificate | Y | N | The client certificate string, in PEM format, to use for secure connections to the OpenTelemetry Collector. The workload must connect using the certificate if it is provided.|
| OTEL_EXPORTER_OTLP_PROTOCOL | margo.otel.protocol | Y | Y | "grpc" if the preferred protocol is gRPC, "http/protobuf" if the preferred protocol is HTTP + protobuf. The default is "http/protobuf" if nothing is provided for this environment variable. If the preferred protocol is "grpc" but no gRPC endpoint is provided, or if the workload client cannot connect via gRPC, the workload client connects using "http/protobuf". |


## Breaking Changes

* Any device implementation currently injecting the environment variables would be affected by this changes since it needs to change how the OTEL configuration is provided.
* Any workload assuming these environment variables would be provided needs to have its Application Description updated to request the parameters directly instead.

## Alternatives considered (optional)

1. **Continue device-wide environment-variable injection.** This preserves the current integration model, but continues to expose the setting to workloads that do not request it, retains a separate mechanism from providing OTEL configuration information to applications, and requires manipulating workload manifests directly.

## Rejection reason

Not applicable to this draft proposal.
