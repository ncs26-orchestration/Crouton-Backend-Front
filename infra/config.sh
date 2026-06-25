#!/usr/bin/env bash
# Shared configuration for the infra scripts. Edit values here, then run the
# numbered scripts in order. Sourced by each script.

set -euo pipefail

# Project / location
export PROJECT_ID="${PROJECT_ID:-ncs26-solution}"
export REGION="${REGION:-europe-west1}"
export ZONE="${ZONE:-europe-west1-b}"

# Artifact Registry
export AR_REPO="${AR_REPO:-solution}"
export REGISTRY="${REGION}-docker.pkg.dev/${PROJECT_ID}/${AR_REPO}"

# Compute
export VM_NAME="${VM_NAME:-ncs26-vm}"
export MACHINE_TYPE="${MACHINE_TYPE:-e2-medium}"
export VM_SA_NAME="vm-runtime"
export VM_SA_EMAIL="${VM_SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

# CI/CD (Workload Identity Federation)
export GITHUB_REPO="${GITHUB_REPO:-ncs26-orchestration/solution}"
export WIF_POOL="github-pool"
export WIF_PROVIDER="github-provider"
export DEPLOY_SA_NAME="github-deployer"
export DEPLOY_SA_EMAIL="${DEPLOY_SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

# Backups
export BACKUP_BUCKET="${BACKUP_BUCKET:-${PROJECT_ID}-backups}"

# App database settings (POSTGRES_PASSWORD is generated on the VM)
export POSTGRES_USER="${POSTGRES_USER:-app}"
export POSTGRES_DB="${POSTGRES_DB:-app}"

# Retry a command a few times. New service accounts and IAM resources are
# eventually consistent, so bindings can fail for a few seconds after create.
retry() {
  local n=0
  until "$@"; do
    n=$((n + 1))
    if [[ $n -ge 8 ]]; then
      echo "retry: giving up after $n attempts: $*" >&2
      return 1
    fi
    echo "retry $n: $*" >&2
    sleep 5
  done
}

echo "config: project=${PROJECT_ID} region=${REGION} registry=${REGISTRY}"
