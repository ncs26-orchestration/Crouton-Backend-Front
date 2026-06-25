# Build context: db/
# Migration runner: dbmate with the migrations baked in, so the VM needs no
# migration files on disk. Run as a one-shot service on each deploy.
FROM ghcr.io/amacneil/dbmate:2.28
COPY migrations /db/migrations
