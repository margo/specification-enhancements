# Compose Deployment Profile as a Single Compose Project

## Owner

[@MayTakeUpTo8Hours](https://github.com/MayTakeUpTo8Hours) - Moritz Berner, COPA-DATA

## Summary

A `compose` deployment profile can list more than one component, but the specification doesn't say how a device should deploy them together. This SUP proposes that all components of a `compose` deployment profile are merged into one Compose project, the same way `docker compose -f a.yaml -f b.yaml` merges multiple files.

With one project, services from different components share the project name, the default network and named volumes, and they can use `depends_on` on each other. Start order and readiness are then handled by Compose itself and Margo doesn't need its own sequencing for Compose.

It builds on [SUP-01 Compose OCI Registry Publishing and Archive Structure](../../completed/compose-oci/sup-01-compose-oci.md), which defines what a single Compose Archive looks like.

## Reason for proposal

The specification currently only says that `compose` components "will be provided to the device in the same order they are listed" and that "the device will install the components in the same order". What "install" means for several Compose files is left open.

One device may run every component as its own Compose project, another may combine them. The containers, networks and volumes one ends up with are different, so the same application behaves differently on two devices that are both compliant. It is exactly undefined behaviour like this that makes or breaks Margo, as an application description should work the same on any compliant device regardless of the implementation's interpretation.

If every component is its own project, each one gets its own default network and its own project-scoped volumes. Services in different components can't reach each other by service name, share a named volume or use `depends_on` with start and readiness conditions unless someone creates external networks or volumes outside of Margo, even though they are part of one "Application" in Margo's sense. The per-component `wait` attribute tries to fill the dependency gap, but it only works per component and does a fraction of what `depends_on` already does per service.

Merging the components into one project gives one defined result, and it's the same result that is known and established by Docker Compose as in using multiple `-f` arguments: `docker compose -f ... -f ... up`.

## Requirements alignment acknowledgement

This SUP addresses TWG feature [margo/specification #168](https://github.com/margo/specification/issues/168), "Define improvements to compose manifest workloads when targeting typical targets (Docker/Podman)". It further improves the guidance for application developers and device manufacturers on how a compose is applied. Compose features like explicit networks, volumes and `depends_on` with `condition: service_healthy` assume that the services can see each other, which is only true across components if they end up in the same Compose project. Although there is currently no Podman Compose support, it would come up there as well.

Out of scope:

- Sharing networks, volumes or `depends_on` between different deployments. Each deployment stays its own Compose project as they are supposed to be independent of each other.

## Technical proposal

### Affected files

| File | Change |
|---|---|
| `system-design/specification/applications/application-description.md` | `type` description for `compose`, `wait` and `timeout` descriptions |
| `src/specification/applications/application-description.linkml.yaml` | `wait` and `timeout` descriptions |

If [margo/specification #222](https://github.com/margo/specification/issues/222) is completed first, these changes apply to the standalone Compose deployment specification instead of `application-description.md`.

### Change 1: Merge components into one Compose project

Replace the `compose` sentences in the `type` description of the DeploymentProfile attributes with:

```markdown
When installing the application on a device supporting Compose, all `compose` components, and only `compose` components, will be provided to the device in the same order they are listed in the application description file.

The device MUST combine all components of a `compose` deployment profile into a single Compose project. The Compose files of the components MUST be merged in the order the components are listed, following the [merge rules of the Compose Specification](https://github.com/compose-spec/compose-spec/blob/main/13-merge.md), so that a later component's values take precedence over an earlier component's values.

If the merged result is not a valid Compose project (for example, a `depends_on` entry references a service that is not defined in any component), the device MUST NOT start any service of the deployment and MUST report the deployment as failed.
```

One thing the Compose merge rules don't cover is relative paths. Per SUP-01 every component is its own Compose Archive with its own top-level directory, so after extraction the device has for example `database/compose.yaml` and `app/compose.yaml` + `app/app.env`. `docker compose -f database/compose.yaml -f app/compose.yaml` resolves every relative path against the directory of the first file, so `env_file: ./app.env` from the app component would point to `database/app.env`. How to handle this is still open, see [Open questions](#open-questions).

### Change 2: Project name

```markdown
The device MUST assign the Compose project name. The project name MUST be unique among the deployments on the device and MUST remain the same across updates of the same deployment. Margo RECOMMENDS deriving the project name from the deployment's identifier. A top-level `name` element in a component's `compose.yaml` MUST be ignored.
```

The specification doesn't define the Compose project name today, so every device picks one its own way (directory name, the `name` in `compose.yaml`, or something else). Two deployments of the same application can even end up with the same project name if both use the `name` from the `compose.yaml`. The same gap exists for Helm release names ([margo/specification #183](https://github.com/margo/specification/issues/183), closed as a duplicate of [#142](https://github.com/margo/specification/issues/142)).

The name has to stay the same across updates so Compose recognises the existing containers and only recreates the services that changed.

### Change 3: Lifecycle

```markdown
Start order and readiness between services, including services from different components, are determined by `depends_on` in the merged Compose project. The order of the components in the application description only determines merge precedence; it does not define a start order.

On an update of the deployment, the device MUST re-create the merged Compose project from the new set of components and apply it to the existing project. Services that are no longer defined in the merged project MUST be removed.
```

### Change 4: `wait` and `timeout` don't apply to `compose` components

`wait` and `timeout` are there to install one component after the other. With a single Compose project there is only one install step, and ordering between services is done with `depends_on`, so both attributes lose their meaning for Compose.

New descriptions in ComponentProperties:

| Attribute | Description (Compose part) |
|---|---|
| `wait` | Not applicable to `compose` components. The device MUST ignore this value for `compose` components. |
| `timeout` | Not applicable to `compose` components. The device MUST ignore this value for `compose` components. |

Nothing changes for `helm` components.

The success criterion from SUP-01 now applies to the merged project as a whole. The deployment is successful once all containers of the project are running (and healthy, where health checks are defined, which implementations SHOULD wait for). If a container exits with a non-zero exit code during startup, the deployment MUST be reported as failed.

### Example (informative)

Two components in one `compose` deployment profile:

```yaml
# component "database", database/compose.yaml
services:
  db:
    image: postgres:16
    volumes:
      - db-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD", "pg_isready"]
volumes:
  db-data:
```

```yaml
# component "app", app/compose.yaml
services:
  app:
    image: registry.example.com/org/app:1.0.0
    env_file: ./app.env          # shipped in the app archive
    depends_on:
      db:
        condition: service_healthy
```

The device runs both as one project. `app` can reach `db` by its service name on the shared default network and only starts once `db` is healthy.

### Open questions

#### Relative paths across components

Docker Compose resolves all relative paths against the first component's directory, but each Margo component ships its referenced files in its own archive directory, so relative paths in every component except the first point to files that don't exist.

- **Option A: Normalise paths per component before merging.** Resolve each component's relative paths against its own directory (e.g. with `docker compose config` per component), then merge. Components stay self-contained, but devices need this extra step.

- **Option B: Extract all components into one shared directory.** Docker Compose then runs unchanged, but the `compose.yaml` files need unique names per component and a rule is needed for files that exist in more than one archive.

### Conformance impact

| RFC 2119 Keyword | Statement |
|---|---|
| MUST | All components of a `compose` deployment profile MUST be combined into a single Compose project. |
| MUST | Component Compose files MUST be merged in list order according to the Compose Specification merge rules. |
| MUST | If the merged project is invalid, the device MUST NOT start any service and MUST report failure. |
| MUST | The device MUST assign a project name that is unique on the device and stable across updates; `name` in component files MUST be ignored. |
| RECOMMENDED | The project name is RECOMMENDED to be derived from the deployment identifier. |
| MUST | On update, services no longer in the merged project MUST be removed. |
| MUST | `wait` and `timeout` MUST be ignored for `compose` components. |

## Breaking Changes

- `wait` and `timeout` have no effect on `compose` components anymore.
- The order of components in a `compose` deployment profile defines merge precedence, not start order.
- Devices that currently run each component as its own Compose project have to switch to a single project.

## Alternatives considered

### One Compose project per component

**Rejected**. Components couldn't share networks or volumes and couldn't use `depends_on` on each other. For applications whose components need to talk to each other and work together, this gets very hard to manage, because every connection between components needs external networks or volumes that live outside the Compose lifecycle.

Margo would also have to keep its own sequencing (`wait`, `timeout`) instead of using what Compose has. To make that anywhere near as usable as `depends_on` (dependencies per service, health and completion conditions, restarts) we would be opening up a whole new rabbit hole in the specification, for problems Compose has already solved.

### Only one component per `compose` deployment profile

**Rejected**. Applications could no longer be split into separately versioned modular components.
