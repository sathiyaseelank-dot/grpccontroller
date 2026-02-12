# Twingate Connector Study (Binary String Analysis Only)

This document summarizes observable behavior of the **twingate-connector** binary based **only** on string analysis of `/usr/bin/twingate-connector` on this host.  
No source code or reverse‑engineering beyond string extraction was used.

## Evidence Source

Commands used:

```bash
file /usr/bin/twingate-connector
strings -n 8 /usr/bin/twingate-connector | rg -i "enroll|auth|token|state|watchdog|systemd|grpc|spiffe|tls|connector|controller|network|offline|online|error"
```

Binary characteristics:
- ELF 64‑bit PIE, stripped
- Dynamically linked
- Uses `libsystemd.so.0` and `sd_watchdog_enabled`

---

## Service Lifecycle / systemd Behavior (Observed)

### Watchdog
Strings indicate systemd watchdog integration:
- `systemd_watchdog`
- `sd_watchdog_enabled`
- `systemd-watchdog`
- `pinged systemd watchdog`
- `unable to ping systemd watchdog`

### Service State Machine
Strings indicate explicit state transitions and logging:
- `Offline`
- `Authentication`
- `Online`
- `Unrecoverable error`
- `UnknownState`

This aligns with the observed runtime logs showing state transitions:
```
State: Offline → Authentication → Error → Online
```

### Service/CLI runtime
Strings show it expects to run as a service and supports a watchdog flag:
- `Run as a service and output to the event log`
- `Enable systemd watchdog`
- `systemd-watchdog`

---

## Enrollment / Authentication Flow (Inferred from Strings)

The binary includes strings referencing:

### Tokens and Auth
- `TWINGATE_ACCESS_TOKEN`
- `TWINGATE_REFRESH_TOKEN`
- `auth_token`
- `token_verification_error`
- `auth_required`
- `auth_started`
- `auth_finished`
- `saving token state to secure storage`

These strings suggest:
- A token‑based authentication flow.
- Token refresh or rotation.
- Secure storage for tokens.

### Controller and Network Configuration
Strings suggest the connector uses:
- `CONTROLLER_URL`
- `controller_url`
- `CONTROLLER_NETWORK`
- `TWINGATE_NETWORK`
- `controller_domain`
- `controller_urlstarting`

### Certificate / CA Path
Strings include:
- `TWINGATE_CA_PATH`
- `ca_path`
- `TLS CA`

This implies:
- The connector loads a CA path from configuration or env.

### Enrollment / Bootstrap Mode
Strings include:
- `controller_bootstrap_mode_change`
- `controller_state_machine_change`
- `configure_controller`

This suggests a bootstrap mode followed by a steady‑state controller connection.

---

## Secure Storage (Observed Strings Only)

The binary contains explicit strings indicating a secure‑storage abstraction with a fallback to file storage:

- `secure storage not available`
- `Secure storage is not set, using file storage as a fallback (insecure) storage`
- `saving token state to secure storage`
- `failed to save auth data to secure storage`
- `loaded public key and certificate from secure storage`
- `secure storage doesn't have node's certificate`
- `secure storage doesn't have node's public key`

**What can be concluded from strings only:**

- There is a “secure storage” backend that can store tokens and node cert/key material.
- If secure storage is not configured/available, it falls back to “file storage”.

**What cannot be concluded from strings only:**

- Which OS facility is used (e.g., TPM, keyring, libsecret, etc.).
- File paths or formats for the “file storage” fallback.

---

## Observed Config Inputs (Strings)

The binary exposes multiple configurable flags/env names:
- `TWINGATE_NETWORK`
- `TWINGATE_DOMAIN`
- `TWINGATE_URL`
- `CONTROLLER_URL`
- `CONTROLLER_NETWORK`
- `TWINGATE_CA_PATH`
- `TWINGATE_LOG_LEVEL`
- `TWINGATE_METRICS_PORT`
- `systemd_watchdog`

---

## Limitations

This analysis is **not** source‑level and cannot assert:
- Exact protocol steps
- Exact enrollment handshake
- Exact storage format or security guarantees

It only documents **observable identifiers and behavior signals** from binary strings.

---

## Summary

From string analysis, the Twingate connector:
- Runs as a systemd service with watchdog support.
- Uses a state machine: Offline → Authentication → Online.
- Uses token‑based authentication with refresh/storage.
- Accepts controller/network configuration and CA path settings.
- Supports a bootstrap phase and then steady‑state controller connection.

---

## If You Want Similar Behavior Under `/etc/twingate-connector`

From the strings alone, the connector appears to:
1. Prefer a **secure storage** backend for tokens and node certs.
2. Fall back to **file storage** when secure storage is unavailable.

To emulate this model in your own connector while keeping DynamicUser and `ProtectSystem=full`, you typically need:

1. A root‑owned directory like `/etc/twingate-connector` (or `/etc/grpcconnector`) containing only **configuration**.
2. A separate writable runtime directory owned by the service user (e.g. `/var/lib/...` or a systemd `RuntimeDirectory=`) for **state** if you implement a file‑storage fallback.
3. A secure‑storage backend (e.g., TPM, kernel keyring, OS keychain) **if** you want secrets to avoid disk entirely.

Because this document is based only on binary strings, the exact storage backend and file locations cannot be confirmed without source or official documentation.

This matches the high‑level Twingate operational model but should be validated against official documentation for exact semantics.
