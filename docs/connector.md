# Connector Configuration and Runtime Behavior

This document describes how the connector is configured, how it starts, and the primary functions involved.

## Configuration Sources

The connector reads configuration from **environment variables** only. systemd is expected to supply these via:

`EnvironmentFile=/etc/grpcconnector/connector.conf`

### Required Environment Variables
- `CONTROLLER_ADDR`  
  Controller gRPC address in `host:port` form.
- `CONNECTOR_ID`  
  Stable connector identifier.
- `ENROLLMENT_TOKEN`  
  Required for enrollment (in-memory mode).
- `CONTROLLER_CA_PATH`  
  Filesystem path to the controller CA PEM file.

### Optional Environment Variables
- `CONNECTOR_PRIVATE_IP`  
  Overrides auto-detected private IP.
- `CONNECTOR_VERSION`  
  Overrides build version.
- `TRUST_DOMAIN`  
  SPIFFE trust domain; defaults to `mycorp.internal` and is normalized (trailing dot removed).

## Runtime Flow

1. Read env variables (systemd supplies them).
2. Enroll using `ENROLLMENT_TOKEN` and controller CA from `CONTROLLER_CA_PATH`.
3. Establish control-plane gRPC connection with mTLS.
4. Send heartbeat every ~10 seconds.
5. Auto-reconnect on failure.

## Primary Functions

### Entry
- `main.go`
  - Dispatches subcommands: `enroll` and `run`.

### Enrollment
- `enroll.ConfigFromEnvEnroll()`  
  Reads enrollment configuration from environment.
- `enroll.Enroll()`  
  Performs enrollment RPC, validates returned CA and cert, returns workload cert and CA.
- `loadExplicitCA()`  
  Reads CA PEM from `CONTROLLER_CA_PATH`.

### Run
- `run.Run()`  
  Main long-running loop: enrolls, builds cert store, connects to control-plane, sends heartbeats.
- `controlPlaneLoop()` / `connectControlPlane()`  
  Maintains persistent gRPC stream and heartbeats.
- `renewalLoop()` / `renewOnce()`  
  Renews short-lived certificates using the controller.

## TLS / SPIFFE Verification

- The controller certificate is verified against the CA at `CONTROLLER_CA_PATH`.
- SPIFFE URI SAN is required and validated for the controller role.
- TLS chain validation uses `RootCAs` and verified chains; no `InsecureSkipVerify`.

