#!/usr/bin/env bash
# Cross-compiles the binary for linux/amd64 and scp's it to a VPS.
# Host-specific settings come from .env (gitignored, not this script).
#
# Required in .env:
#   VPS_HOST      user@host (or a Host alias from ~/.ssh/config)
#   VPS_PATH      destination path for the binary on the VPS
# Optional in .env:
#   VPS_SSH_OPTS  extra flags passed to scp, e.g. "-i ~/.ssh/vps_key -P 2222"
#                 (scp-only; for a non-default port/identity to also apply to
#                 the ssh mkdir step below, configure a Host block in
#                 ~/.ssh/config instead and use its alias as VPS_HOST)
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

if [ -f .env ]; then
	set -a
	# shellcheck disable=SC1091
	source .env
	set +a
fi

: "${VPS_HOST:?set VPS_HOST in .env (see .env.example)}"
: "${VPS_PATH:?set VPS_PATH in .env (see .env.example)}"
VPS_SSH_OPTS="${VPS_SSH_OPTS:-}"

BIN=./bin/docker-app-updater

echo "compiling for linux/amd64..."
GOOS=linux GOARCH=amd64 go build -o "$BIN" ./cmd/docker-app-updater

echo "ensuring remote directory exists..."
ssh "$VPS_HOST" mkdir -p "$(dirname "$VPS_PATH")"

echo "copying to $VPS_HOST:$VPS_PATH..."
# shellcheck disable=SC2086
scp $VPS_SSH_OPTS "$BIN" "$VPS_HOST:$VPS_PATH"

echo "done."
