# Zero-Trust Connector/Tunneler (Twingate-Like) Features

This document summarizes the current features and behavior of the controller, connector, and tunneler, modeled after Twingate’s setup + bootstrap flow.

## 1. High-Level Components

- **Controller**
  - gRPC control plane (mTLS + SPIFFE)
  - Admin REST API for dashboard
  - Token service (one-time enrollment tokens)
  - In-memory registry for connector/tunneler runtime status

- **Connector**
  - Privileged workload
  - Enrolls once with controller
  - Maintains persistent gRPC control-plane connection
  - Accepts mTLS connections from authorized tunnelers

- **Tunneler (Agent)**
  - Enrolls once with controller
  - Establishes mTLS connection to connector
  - Sends heartbeat through connector → controller

## 2. Enrollment Flow (One-Time)

### Connector

- `grpcconnector enroll`
- Requires:
  - `ENROLLMENT_TOKEN`
  - `CONTROLLER_ADDR`
  - `CONNECTOR_ID`
  - `CONTROLLER_CA_PATH`
- Enrolls via gRPC using bootstrap trust
- Receives:
  - Short-lived workload certificate
  - Internal CA certificate

### Tunneler

- `grpctunneler enroll`
- Requires:
  - `ENROLLMENT_TOKEN`
  - `CONTROLLER_ADDR`
  - `TUNNELER_ID`
  - `CONTROLLER_CA_PATH`
- Enrolls via gRPC using bootstrap trust
- Receives:
  - Short-lived workload certificate
  - Internal CA certificate

## 3. Runtime Behavior

### Connector (`grpcconnector run`)

- Persistent gRPC control-plane connection to controller
- Sends heartbeat every ~10s
- Exposes mTLS server for tunnelers
- Only accepts tunnelers approved by controller

### Tunneler (`grpctunneler run`)

- Connects via mTLS to connector
- Sends `tunneler_heartbeat` every ~10s
- Heartbeat is forwarded by connector → controller

## 4. Trust & Security

- Strict mTLS for all control-plane connections
- SPIFFE identity enforced in URI SAN only
- Controller signs workload certificates (short-lived)
- No secrets written to disk by connector/tunneler
- Enrollment tokens are one-time use
- No UI access for connector/tunneler

## 5. Certificates & Rotation

- Connector certs: 5 minutes (testing)
- Tunneler certs: 30 minutes (default)
- Early renewal at 30% remaining lifetime
- Renewal updates in-memory cert store without tearing down streams

## 6. Controller Registry

- Tracks connectors:
  - `id`
  - `private_ip`
  - `last_seen`
  - `version`
- Tracks tunnelers:
  - `id`
  - `connector_id`
  - `last_seen`

## 7. Admin API Endpoints

- `POST /api/admin/tokens`
  - Create one-time enrollment token
- `GET /api/admin/connectors`
  - List connectors with ONLINE/OFFLINE status
- `GET /api/admin/tunnelers`
  - List tunnelers with ONLINE/OFFLINE status

## 8. UI Features

- Dashboard
  - Total/online/offline connectors
  - Total/online/offline tunnelers
  - Latest activity
- Create Connector Token page
- Create Tunneler Token page
- Connectors table
- Tunnelers table

## 9. Installation Scripts

### Connector

```
curl -fsSL https://raw.githubusercontent.com/sathiyaseelank-dot/grpccontroller/main/scripts/setup.sh | sudo \
  CONTROLLER_ADDR="127.0.0.1:8443" \
  CONNECTOR_ID="connector-local-01" \
  ENROLLMENT_TOKEN="<token>" \
  CONTROLLER_CA_PATH="/path/to/ca.crt" \
  bash
```

### Tunneler

```
curl -fsSL https://raw.githubusercontent.com/sathiyaseelank-dot/grpccontroller/main/scripts/tunneler-setup.sh | sudo \
  CONTROLLER_ADDR="127.0.0.1:8443" \
  CONNECTOR_ADDR="<connector-private-ip>:9443" \
  TUNNELER_ID="tunneler-local-01" \
  ENROLLMENT_TOKEN="<token>" \
  CONTROLLER_CA_PATH="/path/to/ca.crt" \
  bash
```

## 10. Systemd Services

- `/etc/systemd/system/grpcconnector.service`
- `/etc/systemd/system/grpctunneler.service`

Both services:
- Use `EnvironmentFile=/etc/.../*.conf`
- Run as `DynamicUser=yes`
- No runtime secrets in systemd
- Hardened filesystem settings

---

## 11. Features Still Needed for Full Twingate Parity

These are missing or incomplete compared to a production Twingate‑style connector.

1. **Persistent identity storage**
   - Connector and tunneler currently keep certs/keys only in memory.
   - Twingate persists identity so restarts do not require re‑enrollment.

2. **Bootstrap vs runtime CA separation**
   - Embedded/bootstrap CA vs controller‑supplied internal CA rotation is not fully modeled.

3. **Token lifecycle policy**
   - Current token store can be long‑lived and reusable (testing).
   - Twingate enforces one‑time use, short TTL, non‑replayable tokens.

4. **CA rotation handling**
   - Runtime CA rotation is not fully supported (clients reject new CA).

5. **Graceful reconnect logic**
   - Control‑plane stream reconnects on errors, but no jitter/backoff tuning.

6. **Connector health monitoring**
   - No explicit health or readiness endpoints for monitoring/diagnostics.

7. **Audit logging**
   - Minimal logging today; no structured audit trail of enrollments/renewals.

8. **Admin auth hardening**
   - Admin API uses static token only (no sessions or JWT rotation).

9. **Multi‑network or multi‑controller support**
   - Single controller address only, no HA or failover.

10. **Secure token delivery**
    - Tokens are displayed in UI and copy/paste only; no secure delivery channel.

11. **Connector/Tunneler upgrade workflow**
    - No staged upgrades or version pinning for agents.

12. **Metrics/observability**
    - No Prometheus or structured metrics for heartbeat and cert rotation.

If you want any of these implemented, tell me the priority and scope.
