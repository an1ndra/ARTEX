# Deploying ARTEX on Dokploy

This fork adds everything needed to run ARTEX on [Dokploy](https://dokploy.com) (or any other
PaaS that builds a Dockerfile / Compose stack). Upstream cannot be deployed as-is: see
[Why the upstream image cannot be built](#why-the-upstream-image-cannot-be-built-here).

| File | Purpose |
|---|---|
| `Dockerfile.dokploy` | Multi-stage build: Next.js static export → Go single binary (`-tags embedui`) → runtime image. Two runtime targets: `runtime` (full recon toolbox) and `runtime-slim` (app only). |
| `docker-compose.dokploy.yml` | Compose stack for Dokploy: app + Postgres, named volumes, Traefik labels, healthchecks, no published host ports. |
| `.env.dokploy.example` | Every variable the stack reads. Paste it into Dokploy's Environment tab. |

## Why the upstream image cannot be built here

`Dockerfile` (upstream) installs tools and then does:

```dockerfile
COPY dist/${TARGETARCH}/artex /app/artex
```

`dist/` is in `.gitignore` and is produced only by the maintainer's CI release job. A fresh
`git clone` has no `dist/`, so `docker build` fails at that `COPY`, and
`docker-compose.yml` sidesteps the problem by pulling the maintainer's prebuilt
`autumn27/artex` image — which is built from the **Chinese** upstream source, not from this
English fork. `Dockerfile.dokploy` fixes both problems by compiling from this repo's source.

## Prerequisites

* A Dokploy server (tested on v0.30.10) with Traefik enabled and ports 80/443 reachable.
* A DNS A record for the app host pointing at the Dokploy server IP.
* ~4 GB free disk for the `runtime` image build (recon toolbox), or ~1 GB for `runtime-slim`.

## Deploy

### 1. Environment

Dokploy → project → environment → **Create Service → Docker Compose**, then:

| Field | Value |
|---|---|
| Name | `artex` |
| Provider | GitHub → repo `an1ndra/ARTEX`, branch `main` |
| Compose Path | `docker-compose.dokploy.yml` |

Open the **Environment** tab and paste `.env.dokploy.example`, then set at minimum:

```env
ARTEX_HOST=artex.example.com        # the DNS name Traefik will serve
ARTEX_CERT_RESOLVER=letsencrypt     # Traefik resolver name in Dokploy (default install)
POSTGRES_PASSWORD=<openssl rand -base64 24 | tr -dc 'A-Za-z0-9'>
ARTEX_BUILD_TARGET=runtime          # or runtime-slim for a fast first boot
```

Add an LLM key (`ANTHROPIC_API_KEY` / `OPENAI_API_KEY`) or leave it empty and configure it in
the UI under **LLM Configuration** after the first login.

### 2. DNS

Add `A artex.example.com → <Dokploy server IP>` and keep it **proxied (orange cloud)** if the zone
is on Cloudflare. This deployment's Traefik install has no ACME resolver configured
(`traefik.yml` defines no `certificatesResolvers`); the edge certificate comes from Cloudflare and
Traefik serves the default Cloudflare origin certificate behind it. A DNS-only record would expose
that origin certificate directly to browsers, which do not trust it. Verify before deploying:

```bash
dig +short artex.example.com
```

### 3. Deploy

Hit **Deploy**. First build with `ARTEX_BUILD_TARGET=runtime` takes roughly 15–25 minutes
(Go + Node compilation, then nmap/nuclei/httpx/subfinder/katana/ffuf/dalfox/sqlmap/bbot and
Playwright + Chromium). `runtime-slim` builds in about 5 minutes and is enough to validate the
stack end to end.

Watch progress in the Dokploy deployment log; **Logs** on the service shows runtime output.

### 4. First login

1. Open `https://artex.example.com` → redirected to `/setup`.
2. Set the admin password (nothing is seeded; the first visit creates it).
3. Optional: **LLM Configuration** → add provider + key, or rely on the env vars.
4. Optional: **System Settings** → number of worker agents (default 3).
5. Install the browser extension from `extension/` and point it at the deployed URL + API token
   if you want to capture your own authenticated browsing as evidence.

## Verification

```bash
# health (what the container healthcheck uses)
curl -s https://artex.example.com/api/health
# → {"ok":true,"service":"artex","version":"0.4.0"}

# the SPA itself
curl -s -o /dev/null -w '%{http_code}\n' https://artex.example.com/

# SSE stream must not be buffered by the proxy (this should print events, not hang silently)
curl -N https://artex.example.com/api/exploration/activity/stream
```

Locally (no Dokploy) the same stack is verifiable with Docker or Podman:

```bash
cp .env.dokploy.example .env   # fill POSTGRES_PASSWORD and ARTEX_HOST
docker compose --env-file .env -f docker-compose.dokploy.yml up -d --build
# or: podman-compose -f docker-compose.dokploy.yml up -d --build
curl -s localhost:8787/api/health     # only reachable with ports: published
```

## What this stack changes versus upstream

| Upstream | Here | Why |
|---|---|---|
| `image: autumn27/artex` | `build: Dockerfile.dokploy` | English fork source, no registry credential needed |
| `./data:/app/data`, `./skills:/app/skills` bind mounts | named volumes `artex-data`, `artex-state`; skills baked into the image | Dokploy re-clones the repo on every deploy, so bind mounts into the checkout are not durable |
| publishes `8787:8787` and `127.0.0.1:8788:8788` | `expose: 8787` + Traefik labels; proxy stays private | Traefik terminates TLS and routes; the MITM proxy must not be internet-facing |
| no healthcheck on `artex` | `GET /api/health` healthcheck | Dokploy/Traefik need a readiness signal, and the DB-backed boot is slow on first start |

Routing is declared with Traefik **docker labels** (`traefik.enable`, `traefik.docker.network`,
router rule on `ARTEX_HOST`, `tls: true`), so no entry in Dokploy's *Domains* tab is required and
no per-host dynamic config file is generated. Do not add the same host in both places: that
creates a second router for the same `Host()` rule. If your Dokploy install does have a working
ACME resolver, add `traefik.http.routers.artex.tls.certresolver=<name>` back — but a resolver name
that does not exist in `traefik.yml` makes Traefik discard the router entirely.

The `jwt.key` detail is worth knowing: ARTEX stores the signing key next to the executable
(`/app`), deliberately *outside* the agent-browsable `/app/data` workspace, so a redeploy
otherwise invalidates every session. The Dockerfile symlinks `/app/jwt.key` →
`/app/.state/jwt.key` and the stack mounts a tiny volume there. Do not "fix" this by moving the
key into `/app/data` — the workspace file manager would then expose it.

## Operations

* **Upgrades**: change `ARTEX_VERSION` (and the release tag if you track one), then redeploy.
  `artex` re-runs `schema.sql` idempotently on every start, so no manual migration; the
  Postgres volume and both app volumes are preserved.
* **Backups**: `scripts/backup.sh` expects the upstream Compose layout. In Dokploy, dump instead:
  ```bash
  docker exec <postgres-container> pg_dump -U artex artex | gzip > artex-db-$(date +%F).sql.gz
  ```
  Add the two app volumes (`artex-data`, `artex-state`) to your backup set as well.
* **Traffic proxy**: bound to `127.0.0.1:8788` inside the container; worker Bash/HTTP traffic
  flows through it and lands in the Traffic view. Publish it only if you want external tools to
  use it, and bind the published port to loopback on the host.
* **Scale**: one stack per host is assumed (fixed Traefik router names `artex` / `artex-http`).
  Running two stacks on one Dokploy host requires giving each its own router names.
* **Concurrency/memory**: each worker runs real CLI tools; size the server accordingly
  (~1 GB RAM idle, more per concurrent worker).
* **Cloudflare in front**: SSE (`/api/logs/stream`, `/api/exploration/activity/stream`) works
  through the proxy, but the free plan drops idle connections after ~100 s. If the activity
  stream stalls on long-idle tasks, that is the edge, not ARTEX — bypass the proxy for that host
  or add a keepalive.

## Security notes

ARTEX is offensive-security tooling with a login and a shell-capable agent. Treat the host as
sensitive:

* Serve it over HTTPS only (the Traefik labels force an HTTP → HTTPS redirect).
* Do not expose port 8788 (traffic proxy) or Postgres publicly; this stack publishes neither.
* Set a strong admin password on `/setup`; there is no rate limiting you can rely on at the edge.
* The container runs as root (raw-socket recon tools such as `nmap` need it) and has full network
  egress. Keep it on a host you control, and keep the LLM keys in Dokploy secrets/env, not in git.
* Only point tasks at systems you own or are explicitly authorised to test.
