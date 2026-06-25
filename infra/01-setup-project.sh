#!/usr/bin/env bash
# Create the GCP project, link billing, enable APIs, and create the Artifact
# Registry Docker repo. Idempotent: safe to re-run.
set -euo pipefail
cd "$(dirname "$0")"
source ./config.sh

# Create the project if it doesn't exist.
if ! gcloud projects describe "$PROJECT_ID" >/dev/null 2>&1; then
  echo "Creating project $PROJECT_ID"
  gcloud projects create "$PROJECT_ID" --name="ncs26 solution"
else
  echo "Project $PROJECT_ID already exists"
fi

gcloud config set project "$PROJECT_ID"

# Link billing. Picks the first open billing account if not provided.
if [[ -z "${BILLING_ACCOUNT:-}" ]]; then
  BILLING_ACCOUNT="$(gcloud billing accounts list --filter='open=true' --format='value(name)' | head -n1)"
fi
if [[ -z "$BILLING_ACCOUNT" ]]; then
  echo "ERROR: no open billing account found. Set BILLING_ACCOUNT=billingAccounts/XX: and re-run." >&2
  exit 1
fi
echo "Linking billing account $BILLING_ACCOUNT"
gcloud billing projects link "$PROJECT_ID" --billing-account="$BILLING_ACCOUNT"

echo "Enabling APIs"
gcloud services enable \
  compute.googleapis.com \
  artifactregistry.googleapis.com \
  iam.googleapis.com \
  iamcredentials.googleapis.com \
  sts.googleapis.com \
  iap.googleapis.com \
  storage.googleapis.com

# Artifact Registry Docker repo.
if ! gcloud artifacts repositories describe "$AR_REPO" --location="$REGION" >/dev/null 2>&1; then
  echo "Creating Artifact Registry repo $AR_REPO in $REGION"
  gcloud artifacts repositories create "$AR_REPO" \
    --repository-format=docker \
    --location="$REGION" \
    --description="ncs26 solution images"
else
  echo "Artifact Registry repo $AR_REPO already exists"
fi

echo "Done. Registry: $REGISTRY"
