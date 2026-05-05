# Margo Identity and Authorization Framework (MIAF) PoC

> **Disclaimer:** This is a proof-of-concept. It is not intended for production use. Cryptographic material is stored on disk without hardware protection, the CA seeds are checked-in test fixtures, and the admin API is unauthenticated.

Proof-of-concept implementation of the [Margo Identity and Authorization Framework (MIAF)](../margo-identity-and-authorization-framework.md) SUP. Two goals drive it:

1. **Validate implementability.** Confirm that every normative requirement in the SUP can be implemented end-to-end with real cryptographic primitives, a real CA, and a real device lifecycle.
2. **Surface spec gaps.** Identify ambiguities, contradictions, and implementation constraints that are only visible during implementation.

## Getting started

**Prerequisites:** Docker, Go 1.25+, and `make`.

Start the stack and build the binaries:

```bash
make up       # start Step CA + MIS in Docker; initializes the PKI on first run
make build    # compile mis and device-agent to bin/
```

The commands below use `--data-dir var/dev/device-agent` to store SVID files in the local tree and `--mis-trust-anchor` to pin the self-signed TLS certificate that `make up` generates under `var/dev/mis-etc/`.

**Enroll** - authenticate via the pre-provisioned factory certificate and receive an X.509 SVID:

```bash
./bin/device-agent \
  --data-dir var/dev/device-agent \
  --mis-trust-anchor var/dev/mis-etc/tls/server.crt \
  enroll factory-cert-mtls \
  --factory-cert var/dev/mis-etc/factory/device-01.crt \
  --factory-key  var/dev/mis-etc/factory/device-01.key
```

On success you'll see:

```
Enrolled: spiffe://margo.example.com/device/<ldi> (status 201, expires ...)
Chain written to var/dev/device-agent/svid.crt
Key written to var/dev/device-agent/svid.key
State written to var/dev/device-agent/state.json
```

**Renew** - exchange the current SVID for a fresh one:

```bash
./bin/device-agent \
  --data-dir var/dev/device-agent \
  --mis-trust-anchor var/dev/mis-etc/tls/server.crt \
  renew
```

**Exchange for a JWT SVID** - obtain a short-lived JWT SVID for workload-to-workload authentication:

```bash
./bin/device-agent \
  --data-dir var/dev/device-agent \
  --mis-trust-anchor var/dev/mis-etc/tls/server.crt \
  jwt-exchange --audience https://example.com/workload
```

**Tear down:**

```bash
make down
```

> **First run note.** `make up` persists Step CA data in a Docker volume and reuses it on subsequent runs. To reset the PKI: `make down && docker volume prune` (confirm you are only pruning volumes belonging to this project).

### Run as daemon

The daemon runs SVID renewal, Bundle Map refresh, and revocation checks automatically on configurable schedules:

```bash
./bin/device-agent \
  --data-dir var/dev/device-agent \
  --mis-trust-anchor var/dev/mis-etc/tls/server.crt \
  daemon --socket var/dev/device-agent/daemon.sock &

./bin/device-agent --data-dir var/dev/device-agent ctl --socket var/dev/device-agent/daemon.sock status
```

`ctl` also exposes `force-renew`, `force-bundle-refresh`, `force-revocations-check`, and `exchange-jwt` for on-demand operations against the running daemon.

### Other enrollment methods

- **`enroll factory-cert-jwt`** - device self-issues a JWT assertion signed by the factory cert private key; use the same `--factory-cert` / `--factory-key` flags as above.
- **`enroll enrollment-token --token-file <path>`** - zero-touch onboarding without factory PKI; generate a token first with `./bin/mis admin issue-token`.

### SPIFFE interop demo

```bash
make up-interop   # start MIS (RA mode) + SPIRE stack together
make e2e          # runs the e2e test suite (incl. the STPIFFE interop test)
```

This demonstrates that MIAF-issued credentials are standard SPIFFE credentials usable with any SPIFFE-compatible tooling. See [spiffe_interop_test.go](./test/e2e/spiffe_interop_test.go) for further details.

## Architecture

The PoC builds two components defined by the SUP:

- **MIS (Margo Identity Service)** - HTTPS identity service that handles device enrollment, SVID issuance, renewal, revocation, and JWT SVID exchange.
- **Device agent** - CLI + long-running daemon representing an Edge Compute Device working through its full identity lifecycle.

Both run against a **[Step CA](https://smallstep.com/docs/step-ca/)** instance acting as the operator's PKI root. The stack runs in one of two deployment modes, selected by the `MIS_ISSUER_BACKEND` environment variable:

```text
┌──────────────────────── Step CA (root) ─────────────────────────┐
│                    spiffe://margo.example.com                     │
└──────────────────┬──────────────────────────┬────────────────────┘
                   │                          │
      Intermediate CA mode               RA mode
      make up                            make up-ra
                   │                          │
           MIS holds                  MIS holds no signing key
           intermediate CA cert
           + private key
                   │                          │
           MIS signs SVIDs            MIS mints one-time token;
           directly via crypto/x509   Step CA signs the SVID
                   │                          │
           device leaf                device leaf
           → MIS intermediate         → Step CA intermediate
           → Step CA root             → Step CA root
```

Both modes expose an **identical device-facing API**; the internal signing and revocation paths differ.

## Scope

### Implemented

| Feature | SUP reference |
| --- | --- |
| Factory-cert mTLS bootstrap | §5.1 |
| Factory-cert JWT bootstrap | §5.2 |
| Enrollment-token bootstrap | §5.3 |
| SVID renewal with key-rotation policy | §6 |
| JWT SVID profile + exchange endpoint | §8 |
| Revocation list (MIS-authoritative) | §9 |
| SPIFFE Bundle Map | §4 |
| Discovery document | §3 |
| Error handling per SUP Appendix B | Appendix B |
| SPIFFE interop (SPIRE + MIS under shared root) | non-normative |
| Intermediate CA and RA deployment modes | §3 |

### Not implemented

- **FDO bootstrap** - architecture is in place; implementation plan not yet written
- **Root CA mode** - SUP §3 mentions three MIS modes; only Intermediate CA and RA are implemented
- **TPM/TEE device key protection** - PoC uses software-only keys (PoC limitation, not a spec gap)
- **Production HA/scaling** - single-node SQLite

## Local dev

```bash
make up           # start stack in Intermediate CA mode
make up-ra        # start stack in RA mode (MIS delegates signing to Step CA)
make up-interop   # start MIS (RA mode) + SPIRE stack together (full interop demo)
make up-spire     # start SPIRE stack only (pair with local MIS for interop dev)
make build        # compile mis and device-agent binaries to bin/
make test         # unit + conformance tests (~30 s, no external deps)
make integration  # integration tests (requires make up or make up-ra first)
make e2e          # end-to-end tests (requires make up or make up-ra first)
make e2e-daemon   # fake-clock daemon lifecycle tests (no external deps)
make down         # tear down the started stack (all profiles)
make clean        # remove bin/ and var/dev/ (requires make dev-up to re-populate)
```

**Local MIS development.** `make dev-up` starts Step CA in Docker and populates `var/dev/mis-etc/` with the TLS certificate, CA chain, JWT signing key, and Step CA credentials MIS needs. `make dev-env` prints the shell exports (`MIS_ETC_DIR`, `MIS_DATA_DIR`, and connection settings) so you can run `./bin/mis` locally against the containerized CA - useful for rapid iteration without rebuilding the full image.

## Testing

Tests are layered to minimize external dependencies:

| Layer | Command | External deps |
| --- | --- | --- |
| Unit | `make unit` | None |
| Conformance (golden-file) | `make conformance` | None |
| Unit + Conformance | `make test` | None |
| Integration | `make integration` | Running stack |
| End-to-end | `make e2e` | Running stack |
| Daemon lifecycle | `make e2e-daemon` | Docker (daemon-test profile) |

**Conformance tests** (`test/conformance/`) assert MIS request/response shapes match the SUP's normative examples, normalizing non-deterministic fields (timestamps, UUIDs, serials).

**Error matrix** - E2E tests cover every row of SUP Appendix B. Each `(endpoint, condition)` pair is asserted against the expected HTTP status code, `type` URI, `title`, and (where required) `Retry-After` header.

## Layout

```text
cmd/mis/                    MIS HTTPS server + admin client CLI
cmd/device-agent/           Device agent CLI (enroll, renew, daemon, ...)
pkg/mis/
  bootstrap/                Per-method enrollment validators (one sub-package per method)
  transport/http/           HTTPS handler per endpoint (enrollment, renewal, bundle-map, ...)
  transport/admin/          Loopback admin API (enrollment-token generation, replacement tickets)
  identity/                 Core enrollment pipeline: ESI derivation, LDI minting, policy checks
  ca/                       Intermediate CA issuer (Intermediate mode)
  ra/stepca/                Step CA RA client (RA mode); same port.Issuer interface as ca/
  bundlemap/                SPIFFE Bundle Map formation + sequence versioning
  errors/                   RFC 9457 Problem Details registry
pkg/device-agent/
  identitymanager/          Daemon: scheduler + 4 background jobs (fake-clock testable)
  enroll/                   Bootstrap credential → enrollment request (per method)
  renew/                    SVID renewal client
  keygen/                   Key-pair generation + persistence (PEM, 0600)
  agentstate/               Local identity state (SPIFFE ID, serial, expiry, refresh times)
pkg/common/                 Types + constants shared between MIS and device-agent
test/integration/           MIS integration tests (no external processes)
test/e2e/                   Full-stack end-to-end tests
test/conformance/           Golden-file conformance tests against SUP examples
config/                     Step CA, MIS, and SPIRE configuration files
scripts/                    PKI bootstrap and dev-setup helpers
```
