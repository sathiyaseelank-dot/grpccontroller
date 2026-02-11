# Copilot Instructions for grpccontroller

## Architecture Overview

This is a **zero-trust gRPC connector system** with three main components:

- **Controller** (`backend/controller/`): Internal CA + enrollment/control-plane gRPC server
- **Connector** (`backend/connector/`): Long-running service connecting outbound to controller, accepting inbound tunneler connections
- **Tunneler** (`backend/tunneler/`): Client connecting to a connector with mTLS

**Trust Domain**: `spiffe://mycorp.internal`

- Connector SPIFFE ID: `spiffe://mycorp.internal/connector/<id>`
- Tunneler SPIFFE ID: `spiffe://mycorp.internal/tunneler/<id>`

## Build, Test, and Run Commands

### Build Each Component

Each component is a separate Go module (Go 1.24.13):

```bash
cd backend/controller && go build ./...
cd ../connector && go build ./...
cd ../tunneler && go build ./...
```

The repo root does not contain Go packages—`go build ./...` there will report no matches.

### Frontend (Next.js)

```bash
cd frontend/next.js-app
npm run dev      # Development server
npm run build    # Production build
npm run start    # Start production server
npm run lint     # ESLint
```

No tests are currently defined in the backend or frontend.

### Setup Script

`scripts/setup.sh` is a one-time installer for the connector (root-only, non-interactive):

```bash
sudo CONTROLLER_ADDR=host:port CONNECTOR_ID=id ENROLLMENT_TOKEN=token CONTROLLER_CA_PATH=/path/to/ca.crt ./scripts/setup.sh
```

## Key Conventions

### Proto Generation

The single `.proto` file is at `backend/proto/controller.proto`. It defines:

- `EnrollmentService.EnrollConnector()` and `EnrollTunneler()`
- `ControlPlane.Connect()` (bidirectional stream)

Generated code lives in `backend/controller/gen/controllerpb/`. **Do not edit generated files manually.**

### Configuration via Environment Variables

- **Controller**: Reads from env or local files (`ca/ca.crt`, `ca/ca.key`). Key vars: `INTERNAL_CA_CERT`, `INTERNAL_CA_KEY`, `ADMIN_AUTH_TOKEN`, `INTERNAL_API_TOKEN`, `TRUST_DOMAIN`, `ADMIN_HTTP_ADDR`, `TOKEN_STORE_PATH`
- **Connector/Tunneler**: Both use subcommands (`enroll` and `run`). Both read from env (systemd supplies via `EnvironmentFile=/etc/grpcconnector/connector.conf`). Key vars: `CONTROLLER_ADDR`, `CONNECTOR_ID` (or `TUNNELER_ID`), `ENROLLMENT_TOKEN`, `CONTROLLER_CA_PATH`, `TRUST_DOMAIN`

See `docs/controller.md` and `docs/connector.md` for detailed configuration and runtime flows.

### TLS/mTLS/SPIFFE

- gRPC uses mTLS with certificate validation (no `InsecureSkipVerify`)
- SPIFFE URI SANs are required and validated by interceptors
- Trust domain matching is enforced
- Connector and tunneler auto-renew short-lived certs in background renewal loops

### Package Structure

- **controller**: `admin/` (REST API), `api/` (gRPC services), `ca/` (cert generation), `state/` (registry/token store), `tls/` (TLS helpers)
- **connector**: `enroll/` (enrollment logic), `run/` (long-running loop), `internal/` (shared helpers)
- **tunneler**: Similar structure to connector

### Deployment

systemd units are in `systemd/` with hardened security options. Controller listens on `:8443` (gRPC) and `:8080` (admin HTTP). Connector listens on `:9443` (tunneler connections). Token store persists to `/var/lib/grpccontroller/tokens.json` (controller only).

## Documentation

- `docs/controller.md`: Controller configuration, runtime flow, and primary functions
- `docs/connector.md`: Connector configuration, runtime flow, and primary functions
- `backend/README.md`: High-level overview of components, env vars, and systemd units
- `backend/controller/RUN.md`: Quick-start guide for running the controller locally
