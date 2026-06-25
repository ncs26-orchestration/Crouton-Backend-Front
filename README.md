# ncs26 solution

Production monorepo: React 19 / TanStack Start frontend, Go (Echo) + sqlc
backend, Postgres 18, all behind an nginx reverse proxy and fully dockerized.
Every push to `main` builds images and deploys to a GCE VM.

## Layout

```
apps/
  api/                Go/Echo backend (sqlc-generated db access)
  web/                TanStack Start (React 19) SSR frontend
db/
  migrations/         dbmate migrations
  queries/            sqlc query input
deploy/
  docker-compose.yml  production stack (images from Artifact Registry)
  nginx/              reverse-proxy config + image
infra/                one-time GCP provisioning scripts
.github/workflows/    ci.yml (PRs) and deploy.yml (push to main)
docker-compose.yml    local full-stack (builds from source)
sqlc.yaml
```

## Request routing

nginx is the single entrypoint. `/api/*` goes to the Go backend; everything
else goes to the TanStack Start Node server (SSR). The browser only ever talks
to nginx, so the frontend calls the API with relative `/api/...` URLs (no CORS).

## Local development

```
make install        # install web deps
make up             # build + run full stack at http://localhost:8088
make logs
make down
```

Backend-only loop:

```
make test-go
make sqlc            # regenerate Go after editing db/queries or db/migrations
make new-migration name=add_widget
make migrate         # apply to the dev compose postgres
```

## Deployment

GCP infrastructure is provisioned once with the scripts in `infra/` (see
`infra/README.md`). After that, pushing to `main` triggers `.github/workflows/
deploy.yml`, which builds the `api`, `web`, `migrate`, and `nginx` images,
pushes them to Artifact Registry, then runs migrations and restarts the stack
on the VM over an IAP SSH tunnel. CI (`ci.yml`) runs on pull requests.
