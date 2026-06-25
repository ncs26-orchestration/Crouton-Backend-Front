# Infrastructure setup

One-time GCP provisioning for the ncs26 solution. Run the numbered scripts in
order from a machine with `gcloud` (project owner / billing access) and `gh`
(admin on the GitHub repo). Re-running is safe; the scripts are idempotent.

```
cd infra
./01-setup-project.sh    # project, billing, APIs, Artifact Registry
./02-setup-vm.sh         # runtime SA, backup bucket, firewall, GCE VM
./03-setup-wif.sh        # Workload Identity Federation + repo variables
```

Edit `config.sh` first if you want a different project id, region, VM size, or
GitHub repo. If `ncs26-solution` is already taken globally, set a different
`PROJECT_ID` and re-run.

## What gets created

- Project `ncs26-solution`, region `europe-west1`.
- Artifact Registry Docker repo `solution`.
- GCE VM `ncs26-vm` (`e2-medium`, Debian 12) with Docker + compose installed by
  a startup script. The startup script also writes `/opt/app/.env` with a random
  Postgres password and installs a nightly backup cron to a GCS bucket.
- Firewall: tcp:80 from anywhere, tcp:22 only from the IAP range.
- Service accounts: `vm-runtime` (attached to the VM) and `github-deployer`
  (impersonated by GitHub Actions via WIF, no JSON keys).
- GitHub Actions repo variables: `GCP_PROJECT_ID`, `GCP_REGION`, `REGISTRY`,
  `WIF_PROVIDER`, `DEPLOY_SA`, `VM_NAME`, `VM_ZONE`, `VM_IP`.

After provisioning, push to `main` and the deploy workflow takes over.

## Backups

`/opt/app/backup.sh` runs daily at 03:00 UTC on the VM, dumping Postgres and
uploading a gzip to `gs://<project>-backups/`. Restore with:

```
gcloud storage cp gs://<project>-backups/<file>.sql.gz - \
  | gunzip \
  | docker compose -f /opt/app/docker-compose.yml exec -T postgres psql -U app -d app
```
