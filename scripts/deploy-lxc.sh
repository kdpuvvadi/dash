#!/usr/bin/env bash
# Build the full multi-stage image on the LXC (UI + Go inside Docker) and recreate dash.
# Usage: ./scripts/deploy-lxc.sh [user@host]
set -euo pipefail

HOST="${1:-root@10.20.20.24}"
REMOTE_DIR="${REMOTE_DIR:-/root/dash}"
IMAGE="${IMAGE:-ghcr.io/kdpuvvadi/dash:latest}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "Syncing source → ${HOST}:${REMOTE_DIR}"
rsync -az --delete \
  --exclude '.git' \
  --exclude 'web/node_modules' \
  --exclude 'web/dist' \
  --exclude 'internal/embedui/dist/assets' \
  --exclude 'bin' \
  --exclude 'dist' \
  --exclude 'caddy-data' \
  --exclude 'caddy-config' \
  --exclude 'config/data' \
  --exclude 'config/ssh' \
  --exclude 'config/app.yaml' \
  --exclude 'config/secrets.yaml' \
  --exclude '.env' \
  "${ROOT}/" "${HOST}:${REMOTE_DIR}/"

echo "Docker build on ${HOST} (UI + Go in-image)"
ssh -o BatchMode=yes "${HOST}" "cd ${REMOTE_DIR} && docker build -t ${IMAGE} . && docker compose up -d --force-recreate dash && docker image prune -f >/dev/null"
echo "Done → ${IMAGE}"
