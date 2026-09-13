# Contributing to Sailbox

Thanks for your interest in contributing. Here's how to get started.

## Getting started

1. Fork the repo and clone it locally
2. Install dependencies:
   - **Backend:** Go 1.22+, Docker (for PostgreSQL)
   - **Frontend:** Bun
3. Copy `apps/api/.env.example` to `apps/api/.env` and set `JWT_SECRET` and
   `SETUP_SECRET` — the API refuses to start without them

Then pick the setup that matches what you are working on.

### Without Kubernetes (most changes)

Leave `KUBECONFIG` unset and the API runs against the *noop orchestrator*: it
logs what it would have done instead of talking to a cluster. Apps and databases
can be created and will sit in `pending`, which is enough for work on the API,
the data model, or the UI.

The root `docker-compose.yml` is PostgreSQL on its own, published on port
`54321` — note that `.env.example` ships the port used by the full stack below,
so override it here:

```bash
DB_PASSWORD=sailbox docker compose up -d
export DATABASE_URL="postgres://sailbox:sailbox@localhost:54321/sailbox?sslmode=disable"

cd apps/api && go run ./cmd/migrate up
cd apps/api && go run ./cmd/server            # http://localhost:8080
cd apps/web && bun dev                        # http://localhost:3000
```

The web dev server proxies `/api` and `/ws` to `localhost:8080`. If that port is
taken, run the API elsewhere and point the proxy at it:

```bash
SERVER_PORT=8090 go run ./cmd/server
API_PROXY_TARGET=http://localhost:8090 bun dev
```

Open http://localhost:3000 — the first account you register becomes the owner,
and registration closes after it.

### With Kubernetes (deploys, builds, cluster pages)

`make dev` brings up the full stack from `deploy/docker-compose.yml`, which
includes K3s, and then runs the API and web dev servers together. Use this when
a change actually has to reach a cluster — building images, rolling deploys, pod
logs, the cluster pages.

## Making changes

- Create a branch from `main`
- Keep commits focused — one change per commit
- Write clear commit messages
- Make sure `make fmt` and `make lint` pass before pushing
- Add or update tests if applicable

## Pull requests

- Open a PR against `main`
- Fill out the PR template
- Keep PRs small and focused — easier to review, faster to merge
- Link related issues with `Closes #123`

## Reporting bugs

Use the [bug report template](https://github.com/sailboxhq/sailbox/issues/new?template=bug_report.yml) on GitHub Issues.

## Requesting features

Use the [feature request template](https://github.com/sailboxhq/sailbox/issues/new?template=feature_request.yml) on GitHub Issues.

## License

By contributing, you agree that your contributions will be licensed under [AGPL-3.0](LICENSE).
