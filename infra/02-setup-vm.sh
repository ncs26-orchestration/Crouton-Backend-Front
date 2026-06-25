#!/usr/bin/env bash
# Create the runtime service account, backup bucket, firewall rules, and the
# GCE VM that runs the docker-compose stack. Idempotent.
set -euo pipefail
cd "$(dirname "$0")"
source ./config.sh
gcloud config set project "$PROJECT_ID"

# Runtime service account attached to the VM (pulls images, writes backups).
if ! gcloud iam service-accounts describe "$VM_SA_EMAIL" >/dev/null 2>&1; then
  gcloud iam service-accounts create "$VM_SA_NAME" --display-name="ncs26 VM runtime"
fi
retry gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:${VM_SA_EMAIL}" \
  --role="roles/artifactregistry.reader" --condition=None >/dev/null

# Backup bucket + object access for the VM SA.
if ! gcloud storage buckets describe "gs://${BACKUP_BUCKET}" >/dev/null 2>&1; then
  gcloud storage buckets create "gs://${BACKUP_BUCKET}" --location="$REGION" --uniform-bucket-level-access
fi
retry gcloud storage buckets add-iam-policy-binding "gs://${BACKUP_BUCKET}" \
  --member="serviceAccount:${VM_SA_EMAIL}" --role="roles/storage.objectAdmin" >/dev/null

# Project-wide OS Login so the deploy SA can SSH in (see 03-setup-wif.sh).
gcloud compute project-info add-metadata --metadata=enable-oslogin=TRUE

# Firewall: public HTTP, and SSH only from the IAP range.
if ! gcloud compute firewall-rules describe allow-http >/dev/null 2>&1; then
  gcloud compute firewall-rules create allow-http \
    --direction=INGRESS --action=ALLOW --rules=tcp:80 \
    --source-ranges=0.0.0.0/0 --target-tags=http-server
fi
if ! gcloud compute firewall-rules describe allow-iap-ssh >/dev/null 2>&1; then
  gcloud compute firewall-rules create allow-iap-ssh \
    --direction=INGRESS --action=ALLOW --rules=tcp:22 \
    --source-ranges=35.235.240.0/20
fi

# Render the startup script with config baked in.
STARTUP="$(mktemp)"
trap 'rm -f "$STARTUP"' EXIT
cat > "$STARTUP" <<EOF
#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

# Docker + compose plugin
if ! command -v docker >/dev/null 2>&1; then
  curl -fsSL https://get.docker.com | sh
fi

mkdir -p /opt/app

# Let docker pull from Artifact Registry using the VM service account.
gcloud auth configure-docker ${REGION}-docker.pkg.dev -q

# Generate .env once, with a random DB password that never leaves the VM.
if [[ ! -f /opt/app/.env ]]; then
  DB_PW="\$(openssl rand -hex 24)"
  cat > /opt/app/.env <<ENV
REGISTRY=${REGISTRY}
TAG=latest
POSTGRES_USER=${POSTGRES_USER}
POSTGRES_PASSWORD=\${DB_PW}
POSTGRES_DB=${POSTGRES_DB}
ENV
  chmod 600 /opt/app/.env
fi

# Nightly DB backup to GCS.
cat > /opt/app/backup.sh <<'BACKUP'
#!/usr/bin/env bash
set -euo pipefail
cd /opt/app
set -a; source ./.env; set +a
TS="\$(date -u +%Y%m%dT%H%M%SZ)"
docker compose exec -T postgres pg_dump -U "\$POSTGRES_USER" -d "\$POSTGRES_DB" \
  | gzip \
  | docker run --rm -i google/cloud-sdk:slim \
      gcloud storage cp - "gs://${BACKUP_BUCKET}/\${TS}.sql.gz"
BACKUP
chmod +x /opt/app/backup.sh
echo "0 3 * * * root /opt/app/backup.sh >> /var/log/ncs26-backup.log 2>&1" > /etc/cron.d/ncs26-backup
EOF

# Create or update the VM.
if ! gcloud compute instances describe "$VM_NAME" --zone="$ZONE" >/dev/null 2>&1; then
  echo "Creating VM $VM_NAME"
  gcloud compute instances create "$VM_NAME" \
    --zone="$ZONE" \
    --machine-type="$MACHINE_TYPE" \
    --image-family=debian-12 --image-project=debian-cloud \
    --boot-disk-size=30GB \
    --service-account="$VM_SA_EMAIL" \
    --scopes=cloud-platform \
    --tags=http-server \
    --metadata-from-file=startup-script="$STARTUP"
else
  echo "VM $VM_NAME exists; updating startup script"
  gcloud compute instances add-metadata "$VM_NAME" --zone="$ZONE" \
    --metadata-from-file=startup-script="$STARTUP"
fi

VM_IP="$(gcloud compute instances describe "$VM_NAME" --zone="$ZONE" \
  --format='get(networkInterfaces[0].accessConfigs[0].natIP)')"
echo "Done. VM_NAME=$VM_NAME ZONE=$ZONE IP=$VM_IP"
echo "App will be reachable at http://$VM_IP once the first deploy runs."
