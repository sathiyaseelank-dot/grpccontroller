#!/usr/bin/env bash
set -euo pipefail

# Zero-Trust gRPC Connector one-time installer (non-interactive).
# - Installs connector binary
# - Runs one-time enrollment
# - Enables and starts systemd service

if [[ "${EUID}" -ne 0 ]]; then
  echo "ERROR: setup must be run as root." >&2
  exit 1
fi

required_envs=(MY_CONNECTOR_TOKEN)
for var in "${required_envs[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    echo "ERROR: ${var} is required." >&2
    exit 1
  fi
done

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

if [[ "${os}" != "linux" ]]; then
  echo "ERROR: unsupported OS '${os}'. Linux only." >&2
  exit 1
fi

case "${arch}" in
  x86_64|amd64)
    arch="amd64"
    ;;
  aarch64|arm64)
    arch="arm64"
    ;;
  *)
    echo "ERROR: unsupported architecture '${arch}'." >&2
    exit 1
    ;;
esac

binary="grpcconnector-${os}-${arch}"
release_url="https://github.com/sathiyaseelank-dot/grpccontroller/releases/latest/download/${binary}"
unit_url="https://raw.githubusercontent.com/sathiyaseelank-dot/grpccontroller/main/backend/systemd/grpcconnector.service"

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "${tmpdir}"
}
trap cleanup EXIT

echo "Downloading connector binary..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "${release_url}" -o "${tmpdir}/grpcconnector"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "${tmpdir}/grpcconnector" "${release_url}"
else
  echo "ERROR: curl or wget is required for download." >&2
  exit 1
fi

install -m 0755 "${tmpdir}/grpcconnector" /usr/bin/grpcconnector

systemd_dst="/etc/systemd/system/grpcconnector.service"

echo "Downloading systemd unit..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "${unit_url}" -o "${tmpdir}/grpcconnector.service"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "${tmpdir}/grpcconnector.service" "${unit_url}"
else
  echo "ERROR: curl or wget is required for download." >&2
  exit 1
fi

install -m 0644 "${tmpdir}/grpcconnector.service" "${systemd_dst}"

echo "Running one-time enrollment..."
MY_CONNECTOR_TOKEN="${MY_CONNECTOR_TOKEN}" /usr/bin/grpcconnector enroll

systemctl daemon-reload
systemctl enable grpcconnector.service
systemctl start grpcconnector.service

# Unset sensitive env vars.
unset MY_CONNECTOR_TOKEN

echo "Setup completed."
