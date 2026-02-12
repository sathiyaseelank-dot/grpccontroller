# Twingate `/etc/twingate` Behavior (From setup.sh)

This document describes how the provided Twingate `setup.sh` uses `/etc/twingate`.
It is based **only** on the script content you pasted.

## Files and Permissions

- Directory: `/etc/twingate`
  - Created with `0700` permissions.
- Config file: `/etc/twingate/connector.conf`
  - Created or overwritten by `setup.sh`.
  - If tokens are written, file permissions are set to `0600`.

## Setup Behavior

- If `/etc/twingate/connector.conf` exists and `-f` is **not** passed:
  - The script exits without changes.
- If `-f` is passed and the file exists:
  - It is renamed to `connector.conf.<timestamp>` before writing a new one.

## What Gets Written

`/etc/twingate/connector.conf` is a simple `KEY=VALUE` file. The script writes:

- `TWINGATE_NETWORK=...` **or** `TWINGATE_URL=...` (only one)
- `TWINGATE_ACCESS_TOKEN=...`
- `TWINGATE_REFRESH_TOKEN=...`
- `TWINGATE_LOG_ANALYTICS=...` (only if set)

Tokens are written **only** when both `TWINGATE_ACCESS_TOKEN` and `TWINGATE_REFRESH_TOKEN` are present.

## Service Start Behavior

- If tokens are present, the script enables and starts `twingate-connector` via systemd:
  - `systemctl enable --now twingate-connector`

## How the Connector Reads This File

The script itself does **not** show the connector reading `/etc/twingate/connector.conf` directly.
The expected model (based on typical systemd usage) is:

- systemd uses `EnvironmentFile=/etc/twingate/connector.conf`
- the connector binary reads only environment variables
- the connector does **not** need to open `/etc/twingate/connector.conf`

## Summary

- `/etc/twingate/connector.conf` is a root‑owned bootstrap config.
- It is written once by `setup.sh` and is not dynamically updated by the connector.
- It likely feeds systemd environment variables rather than being read by the binary.
