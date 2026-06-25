#!/usr/bin/env bash
# Set up keyless GitHub Actions -> GCP auth (Workload Identity Federation),
# create the deploy service account with the roles the pipeline needs, and
# push the resulting config to the GitHub repo as Actions variables.
# Requires: gcloud (project owner) and gh (admin on the repo). Idempotent.
set -euo pipefail
cd "$(dirname "$0")"
source ./config.sh
gcloud config set project "$PROJECT_ID"

PROJECT_NUMBER="$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')"

# Deploy service account.
if ! gcloud iam service-accounts describe "$DEPLOY_SA_EMAIL" >/dev/null 2>&1; then
  gcloud iam service-accounts create "$DEPLOY_SA_NAME" --display-name="GitHub Actions deployer"
fi

# Least privilege: push images, describe + IAP-tunnel + sudo-SSH the VM. No
# instance create/delete, no broad serviceAccountUser at the project level.
for role in \
  roles/artifactregistry.writer \
  roles/compute.viewer \
  roles/iap.tunnelResourceAccessor \
  roles/compute.osAdminLogin; do
  retry gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:${DEPLOY_SA_EMAIL}" --role="$role" --condition=None >/dev/null
done

# OS Login into an instance that runs as the VM SA needs serviceAccountUser on
# that SA specifically (not the whole project).
retry gcloud iam service-accounts add-iam-policy-binding "$VM_SA_EMAIL" \
  --member="serviceAccount:${DEPLOY_SA_EMAIL}" \
  --role="roles/iam.serviceAccountUser" >/dev/null

# Workload Identity pool + GitHub OIDC provider (restricted to our repo).
if ! gcloud iam workload-identity-pools describe "$WIF_POOL" --location=global >/dev/null 2>&1; then
  gcloud iam workload-identity-pools create "$WIF_POOL" --location=global \
    --display-name="GitHub Actions pool"
fi

if ! gcloud iam workload-identity-pools providers describe "$WIF_PROVIDER" \
      --location=global --workload-identity-pool="$WIF_POOL" >/dev/null 2>&1; then
  gcloud iam workload-identity-pools providers create-oidc "$WIF_PROVIDER" \
    --location=global --workload-identity-pool="$WIF_POOL" \
    --display-name="GitHub provider" \
    --issuer-uri="https://token.actions.githubusercontent.com" \
    --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository" \
    --attribute-condition="assertion.repository=='${GITHUB_REPO}'"
fi

# Let workflows from this repo impersonate the deploy SA.
retry gcloud iam service-accounts add-iam-policy-binding "$DEPLOY_SA_EMAIL" \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${WIF_POOL}/attribute.repository/${GITHUB_REPO}" >/dev/null

WIF_PROVIDER_RESOURCE="projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${WIF_POOL}/providers/${WIF_PROVIDER}"
VM_IP="$(gcloud compute instances describe "$VM_NAME" --zone="$ZONE" \
  --format='get(networkInterfaces[0].accessConfigs[0].natIP)' 2>/dev/null || echo '')"

echo "Setting GitHub Actions variables on ${GITHUB_REPO}"
gh variable set GCP_PROJECT_ID --repo "$GITHUB_REPO" --body "$PROJECT_ID"
gh variable set GCP_REGION     --repo "$GITHUB_REPO" --body "$REGION"
gh variable set REGISTRY       --repo "$GITHUB_REPO" --body "$REGISTRY"
gh variable set WIF_PROVIDER   --repo "$GITHUB_REPO" --body "$WIF_PROVIDER_RESOURCE"
gh variable set DEPLOY_SA      --repo "$GITHUB_REPO" --body "$DEPLOY_SA_EMAIL"
gh variable set VM_NAME        --repo "$GITHUB_REPO" --body "$VM_NAME"
gh variable set VM_ZONE        --repo "$GITHUB_REPO" --body "$ZONE"
[[ -n "$VM_IP" ]] && gh variable set VM_IP --repo "$GITHUB_REPO" --body "$VM_IP"

echo "Done."
echo "  WIF provider: $WIF_PROVIDER_RESOURCE"
echo "  Deploy SA:    $DEPLOY_SA_EMAIL"
echo "  VM IP:        ${VM_IP:-<none yet>}"
