# Controller Configuration and Runtime Behavior

This document describes how the controller is configured, how it starts, and the primary functions involved.

## Configuration Sources

The controller reads configuration from **environment variables** and optional local CA files.

### Required Environment Variables
- `INTERNAL_CA_CERT` or `ca/ca.crt`  
  CA certificate (PEM).
- `INTERNAL_CA_KEY` or `ca/ca.key`  
  CA private key (PEM, PKCS#8).
- `ADMIN_AUTH_TOKEN`  
  Auth token for admin REST API.
- `INTERNAL_API_TOKEN`  
  Auth token for internal REST API.

### Optional Environment Variables
- `TRUST_DOMAIN`  
  SPIFFE trust domain; defaults to `mycorp.internal` and is normalized (trailing dot removed).
- `ADMIN_HTTP_ADDR`  
  Admin REST bind address; default `:8080`.
- `TOKEN_STORE_PATH`  
  Persistent token store path; default `/var/lib/grpccontroller/tokens.json`.

## Runtime Flow

1. Load CA cert/key (env or `ca/ca.crt` + `ca/ca.key`).
2. Issue or load controller server cert.
3. Start gRPC server on `:8443` with mTLS and SPIFFE interception.
4. Start admin HTTP server concurrently.
5. Maintain in-memory registry of connector heartbeats.

## Primary Functions

### Entry
- `main.go`
  - Loads configuration and initializes CA, registry, token store.
  - Starts gRPC and admin HTTP servers.

### CA and Certificates
- `ca.LoadCA()`  
  Loads CA cert/key.
- `ca.IssueWorkloadCert()`  
  Issues workload certs with SPIFFE URI SAN.
- `loadOrIssueControllerCert()`  
  Creates controller TLS cert signed by the internal CA.

### Enrollment / Auth
- `api.EnrollmentServer.EnrollConnector()`  
  Validates token, issues connector cert, returns CA.
- `api.EnrollmentServer.Renew()`  
  Renews connector certs.
- `state.TokenStore`  
  Creates/consumes tokens and persists hashes (if configured).

### Control Plane
- `api.ControlPlaneServer.Connect()`  
  Accepts connector streams and records heartbeats.
- `state.Registry`  
  Tracks connectors, last seen timestamps, and private IP.

## TLS / SPIFFE Verification

- gRPC server uses mTLS with `ClientCAs` built from internal CA.
- SPIFFE identity is enforced by interceptors on all RPCs except `EnrollConnector`.
- SPIFFE URI SAN is required, trust domain must match, role must be valid.

