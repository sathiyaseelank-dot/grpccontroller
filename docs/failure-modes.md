# Failure Modes (Controller / Connector / Tunneler)

This document lists known failure modes, how they appear, and how they are handled.

## 1. Enrollment Failures

### 1.1 Missing Enrollment Token
- **Symptom**: `ENROLLMENT_TOKEN is required` (connector/tunneler)
- **Cause**: Token not provided in environment or systemd EnvironmentFile.
- **Effect**: Enrollment fails; process exits with non‑zero.

### 1.2 Invalid or Reused Token
- **Symptom**: `invalid enrollment token` or `permission denied` from controller.
- **Cause**: Token already consumed or expired.
- **Effect**: Enrollment fails; process exits with non‑zero.

### 1.3 Controller CA Missing or Invalid
- **Symptom**: `failed to read controller CA` or `invalid internal CA`.
- **Cause**: Missing CA file, incorrect path, or invalid PEM.
- **Effect**: Enrollment fails; process exits with non‑zero.

### 1.4 SPIFFE Trust Domain Invalid
- **Symptom**: `cannot parse URI ... invalid domain`.
- **Cause**: TRUST_DOMAIN contains trailing dot or invalid DNS name.
- **Effect**: Enrollment fails; certificate parse fails.

### 1.5 CA Key Usage Missing
- **Symptom**: `CA certificate missing key usage`.
- **Cause**: Internal CA missing KeyUsageCertSign.
- **Effect**: Enrollment fails due to strict CA validation.

## 2. TLS / mTLS Failures

### 2.1 Unknown Authority
- **Symptom**: `x509: certificate signed by unknown authority`.
- **Cause**: Client not using the same CA as controller/connector.
- **Effect**: gRPC connection fails.

### 2.2 Missing IP SAN
- **Symptom**: `x509: cannot validate certificate for <IP> because it doesn't contain any IP SANs`.
- **Cause**: Certificate missing IP SAN for connector’s private IP.
- **Effect**: Tunneler cannot connect to connector via IP address.

### 2.3 SPIFFE ID Mismatch
- **Symptom**: `SPIFFE trust domain mismatch` or `unexpected SPIFFE role`.
- **Cause**: Trust domain mismatch or incorrect SPIFFE role in cert.
- **Effect**: mTLS connection rejected.

## 3. Control Plane Failures

### 3.1 Control Plane Stream Resets
- **Symptom**: repeated `control-plane stream connected` logs.
- **Cause**: cert rotation, network flaps, or controller restart.
- **Effect**: transient ONLINE/OFFLINE flapping in UI.

### 3.2 Heartbeat Failures
- **Symptom**: CONNECTOR/TUNNELER shows OFFLINE in UI.
- **Cause**: heartbeats not reaching controller; stream down.
- **Effect**: last_seen stale, status flips to OFFLINE.

### 3.3 Tunneler Not Allowed
- **Symptom**: `tunneler not allowed` from connector.
- **Cause**: tunneler not in controller‑approved allowlist.
- **Effect**: connector rejects tunneler mTLS session.

## 4. Certificate Rotation Failures

### 4.1 Renewal Too Late
- **Symptom**: connection drops at expiry, re‑enrollment loops.
- **Cause**: renewal scheduled too close to expiry.
- **Effect**: gRPC streams reset and ONLINE/OFFLINE flapping.

### 4.2 CA Mismatch During Renewal
- **Symptom**: `internal CA mismatch during renewal`.
- **Cause**: controller rotates CA but client has stale CA.
- **Effect**: renewal rejected; client eventually expires.

## 5. Systemd / Runtime Failures

### 5.1 EnvironmentFile Missing
- **Symptom**: `Failed to load environment files: No such file or directory`.
- **Cause**: connector.conf or tunneler.conf not created.
- **Effect**: service fails to start.

### 5.2 Permission Denied on CA file
- **Symptom**: `open /etc/.../ca.crt: permission denied`.
- **Cause**: directory permissions too strict for DynamicUser.
- **Effect**: service fails at startup.

### 5.3 Admin HTTP Port Conflict
- **Symptom**: `bind: address already in use`.
- **Cause**: port :8080 already occupied.
- **Effect**: admin API fails; controller exits.

## 6. UI / API Failures

### 6.1 404 on /api/admin/* in Next.js
- **Symptom**: `404 Not Found` in UI logs.
- **Cause**: missing Next.js API route or wrong proxy config.
- **Effect**: UI cannot fetch data.

### 6.2 Missing ADMIN_API_URL or ADMIN_AUTH_TOKEN
- **Symptom**: `ADMIN_API_URL or ADMIN_AUTH_TOKEN not configured`.
- **Cause**: missing environment variables.
- **Effect**: UI cannot fetch admin endpoints.

---

If you want remediation steps or severity tags for each failure mode, tell me and I’ll extend this document.
