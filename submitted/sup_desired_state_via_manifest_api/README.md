# WFM API Proof of Concept

This repository contains a proof-of-concept implementation of a Workload Fleet Management (WFM) server and client, demonstrating the desired state delivery mechanism described in the associated SUP.

## Prerequisites

- Go 1.18+

## Setup

Build the server and client:

```bash
go build -o wfm ./cmd/wfm
go build -o wfm-client ./cmd/wfm-client
```

## Running the PoC

1. **Start the server:**

Open a terminal and run the server. It will listen on `localhost:8080` by default.

```bash
./wfm
```

2. **Run the client:**

In a separate terminal, run the client. It will start polling the server for a deployment manifest.

```bash
./wfm-client
```

You can customize the client's behavior with flags:
- `--wfm-base-url`: Server base URL (default: `http://localhost:8080`)
- `--device-id`: Device identifier to use (default: `c92cb339-c99c-4eca-9dd4-f8484dd16cfb`)
- `--poll-interval`: How often to poll for manifests (default: `30s`)
- `--verbose`: Enable detailed client-side logging.

You should see the client start, poll the server, and reconcile its state based on the manifest it receives.

> Note: The PoC DB migration seeds exactly one device (`c92cb339-c99c-4eca-9dd4-f8484dd16cfb`). Adding additional devices currently requires inserting rows into the `devices` table manually or extending the migration logic.

The server also exposes interactive documentation:

- Human-friendly docs UI at: `http://localhost:8080/docs`
- Raw OpenAPI/Swagger spec at: `http://localhost:8080/swagger`

## Exploring the API

Use the Postman collection in `docs/postman.json` to create, update, and delete deployments and observe the running client's reactions. The PoC mutation endpoints (POST/PUT/DELETE) are explicitly for demonstration and are not part of the proposed stable contract.

Import that file, set environment variables:

- wfmUrl = `http://localhost:8080`

Then run Create → Manifest → Update/Delete sequences and observe the running client logs.
