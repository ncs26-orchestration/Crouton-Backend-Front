#!/usr/bin/env bash
# Runs on the VM (as root) to roll out a new image tag. Invoked by the deploy
# workflow as: sudo bash /tmp/remote-deploy.sh <image-tag>
set -euo pipefail

SHA="${1:?usage: remote-deploy.sh <image-tag>}"

cd /opt/app

# Point the stack at the new tag (REGISTRY and DB creds already live in .env).
sed -i "s/^TAG=.*/TAG=${SHA}/" .env

docker compose pull
# -T: no TTY/stdin, so the one-shot migration can't swallow the caller's stdin.
docker compose run --rm -T migrate
docker compose up -d
docker image prune -f
