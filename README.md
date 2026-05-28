# Velane

**AI Agent Code Runtime** — write Bun or Python snippets, expose them as POST APIs, run them at scale in secure sandboxed runtimes.

Velane is an open-source, multi-tenant platform that lets AI agent engineers deploy code snippets as callable HTTP endpoints — with versioning, secrets injection, canary traffic splitting, streaming, an admin dashboard, and native MCP integration for Cursor and Claude Code.

---

## Features

- **Snippet runtime** — execute Bun (TypeScript) or Python snippets via `POST /v1/invoke/{tenant}/{snippet}`
- **Sync, async, and streaming** — synchronous blocking, background async with webhook callbacks, and `text/event-stream` streaming
- **Three environments** — `dev` → `staging` → `prod` promotion flow
- **Versioned deployments** — each publish creates an immutable version; instant rollback to any prior version
- **Canary traffic splitting** — send X% of prod traffic to a new version while the rest hits stable
- **Secrets injection** — AES-256-GCM encrypted key-value pairs injected as env vars at invocation time; values never returned by the API
- **Egress policy** — per-tenant IP/CIDR and domain blocklist enforced inside the executor
- **Observability** — per-invocation logs (S3/MinIO), metrics (ClickHouse), and invocation replay
- **Admin portal** — self-serve dashboard for snippet editing, API key management, team invites, branding, usage, and egress config
- **Embeddable dashboard** — iframe-safe read-only snippet browser with white-label theming
- **MCP server** — connect Cursor or Claude Code directly to Velane to generate and deploy snippets without leaving your IDE
- **Git push-to-deploy** — connect a GitHub repo; push to `main` → staging, tag `v*` → prod
- **JWT auth** — RS256 access tokens (15 min) + refresh tokens; JWKS endpoint for third-party verification
- **Firecracker executor** — optional VM-boundary isolation via AWS Firecracker (requires KVM; enabled with `EXECUTOR_TYPE=firecracker`)
- **Seccomp profiles** — syscall allowlist applied to executor containers in production

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                        Clients                           │
│   Admin Portal  ·  Embed Dashboard  ·  CLI  ·  MCP      │
└───────────────────────────┬──────────────────────────────┘
                            │
┌───────────────────────────▼──────────────────────────────┐
│                Control Plane (Go)                        │
│   chi router · pgx/v5 · JWT auth · API key auth          │
│   Scheduler · Async Worker · Observability Pipeline      │
└──────┬────────────────────┬────────────────┬─────────────┘
       │                    │                │
  ┌────▼────┐        ┌──────▼─────┐   ┌──────▼─────┐
  │Postgres │        │   Redis    │   │ClickHouse  │
  │ (state) │        │(queue +    │   │ (metrics)  │
  └─────────┘        │ warm pool) │   └────────────┘
                     └──────┬─────┘
                            │
          ┌─────────────────▼─────────────────┐
          │           Executor Pool            │
          │  ┌──────────┐   ┌──────────────┐  │
          │  │   Bun    │   │    Python    │  │
          │  │  :8081   │   │    :8082     │  │
          │  └──────────┘   └──────────────┘  │
          └───────────────────────────────────┘
```

---

## Quickstart

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) + Docker Compose
- Node.js 18+ and npm (frontend dev only)
- Go 1.22+ (CLI or control plane dev only)

### 1. Clone and configure

```bash
git clone https://github.com/abskrj/velane.git
cd velane
```

Open `docker-compose.yml` and uncomment the bootstrap vars to seed your first admin user:

```yaml
# services > control-plane > environment
BOOTSTRAP_EMAIL: admin@example.com
BOOTSTRAP_PASSWORD: changeme123
BOOTSTRAP_TENANT: myorg
```

### 2. Start everything

```bash
docker compose up --build
```

All services start, database migrations run automatically, and the admin user + tenant are created on first boot. Once `control-plane` logs `http server listening`, you're ready.

### 3. Open the UIs

| Interface | URL | Description |
|-----------|-----|-------------|
| **Admin portal** | http://localhost:8092 | Snippet editor, team, branding, usage |
| **Embed dashboard** | http://localhost:8091 | Read-only iframe snippet viewer |
| API | http://localhost:8080 | REST API |
| MCP server | http://localhost:8090 | Cursor / Claude Code integration |
| MinIO console | http://localhost:9001 | Log storage (minioadmin / minioadmin) |

Log into the admin portal at http://localhost:8092 with the credentials from the bootstrap step.

---

## Writing your first snippet

### Via the admin portal

1. Open http://localhost:8092 and log in
2. Go to **Snippets → New Snippet**
3. Name it, pick a language (Bun or Python), write your code in the Monaco editor
4. Click **Publish → dev**
5. Use the **Test** panel to invoke it inline

### Via the API

```bash
KEY=vl_xxxx   # your API key
TENANT=myorg

# Create a snippet
SNIPPET=$(curl -s -X POST http://localhost:8080/v1/snippets \
  -H "Authorization: Bearer $KEY" \
  -H "X-Tenant: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{"name":"hello","language":"bun"}' | jq -r .id)

# Push code as a new version
VER=$(curl -s -X POST http://localhost:8080/v1/snippets/$SNIPPET/versions \
  -H "Authorization: Bearer $KEY" \
  -H "X-Tenant: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{"code":"export default async (req) => ({ hello: req.name })"}' | jq -r .version_number)

# Publish to dev
curl -s -X POST "http://localhost:8080/v1/snippets/$SNIPPET/versions/$VER/publish?env=dev" \
  -H "Authorization: Bearer $KEY" -H "X-Tenant: $TENANT"

# Invoke it
curl -s -X POST http://localhost:8080/v1/invoke/$TENANT/$SNIPPET \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"world"}' | jq
# → { "hello": "world" }
```

### Via the CLI

```bash
cd services/cli && go build -o velane .

./velane login --key vl_xxxx --tenant myorg --api-url http://localhost:8080
./velane snippets push handler.ts --publish dev
./velane invoke <snippet-id> --input '{"name":"world"}'
./velane invoke <snippet-id> --stream   # SSE streaming
```

---

## Invocation modes

```bash
# Synchronous (default) — blocks until result
curl -X POST http://localhost:8080/v1/invoke/myorg/$SNIPPET \
  -H "Authorization: Bearer $KEY" -d '{"name":"world"}'

# Async — returns 202 immediately, fires webhook on completion
curl -X POST http://localhost:8080/v1/invoke/myorg/$SNIPPET \
  -H "Authorization: Bearer $KEY" \
  -H "X-Invoke-Mode: async" \
  -d '{"name":"world","callback_url":"https://yourserver.com/webhook"}'

# Streaming — text/event-stream
curl -X POST http://localhost:8080/v1/invoke/myorg/$SNIPPET \
  -H "Authorization: Bearer $KEY" \
  -H "X-Invoke-Mode: stream" \
  -d '{}'

# Pinned version
curl -X POST "http://localhost:8080/v1/invoke/myorg/$SNIPPET?version=v2" \
  -H "Authorization: Bearer $KEY" -d '{}'
```

---

## MCP Integration (Cursor / Claude Code)

Add to `.cursor/mcp.json` or `~/.claude/mcp.json`:

```json
{
  "mcpServers": {
    "velane": {
      "url": "http://localhost:8090/mcp",
      "headers": { "Authorization": "Bearer vl_xxxx" }
    }
  }
}
```

Available tools: `list_snippets`, `get_snippet`, `create_snippet`, `update_draft`, `publish_snippet`, `invoke_snippet`, `get_logs`, `list_secrets`, `set_secret`, `get_metrics`.

---

## Embeddable Dashboard

Issue a short-lived embed token and drop it into any page:

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/v1/embed/tokens \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{"tenant_slug":"myorg","ttl_seconds":86400}' | jq -r .token)
```

```html
<iframe
  src="http://localhost:8091?token=et_xxxx"
  width="100%"
  height="700"
  frameborder="0"
/>
```

Theming (logo, accent colour, font) is configured in the admin portal under **Branding** and applied automatically.

---

## Git Push-to-Deploy

Connect a snippet to a GitHub repo:

```bash
curl -s -X POST http://localhost:8080/v1/snippets/$SNIPPET/git-integration \
  -H "Authorization: Bearer $KEY" -H "X-Tenant: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{"provider":"github","repo_url":"https://github.com/you/repo"}' | jq
# returns { "secret": "whsec_..." } — add as a GitHub webhook secret
```

Configure your GitHub webhook to point at `http://your-host:8080/v1/webhooks/git/$SNIPPET_ID`.

| Push target | Deploys to |
|-------------|-----------|
| `main` / `master` | staging |
| Any other branch | dev |
| Tag `v*` | prod |

---

## API Reference

### Authentication

| Token type | Header | Used for |
|-----------|--------|---------|
| API key | `Authorization: Bearer vl_xxxx` | All management + invocation |
| JWT access token | `Authorization: Bearer <jwt>` | Admin portal sessions |
| Embed token | `Authorization: Bearer et_xxxx` | Embed endpoints only |

Tenant context: `X-Tenant: myorg` header (required for snippet and invocation endpoints).

### Endpoints

```
# Snippets
POST   /v1/snippets
GET    /v1/snippets
GET    /v1/snippets/{id}
DELETE /v1/snippets/{id}

# Versions
POST   /v1/snippets/{id}/versions
GET    /v1/snippets/{id}/versions
POST   /v1/snippets/{id}/versions/{n}/publish   ?env=dev|staging|prod

# Invocation
POST   /v1/invoke/{tenant}/{snippet}             X-Invoke-Mode: sync|async|stream
GET    /v1/invocations/{id}                      poll async result
POST   /v1/invocations/{id}/replay

# Canary
POST   /v1/snippets/{id}/canary                  { version_id, percent }
DELETE /v1/snippets/{id}/canary

# Secrets
POST   /v1/secrets
GET    /v1/secrets
DELETE /v1/secrets/{id}

# Observability
GET    /v1/logs/snippets/{id}                    ?limit&env&status
GET    /v1/metrics/snippets/{id}                 ?window=1h|24h|7d

# Tenants
POST   /v1/tenants
GET    /v1/tenants/{slug}/egress
PUT    /v1/tenants/{slug}/egress
GET    /v1/tenants/{slug}/branding
PUT    /v1/tenants/{slug}/branding
GET    /v1/tenants/{slug}/members
POST   /v1/tenants/{slug}/members/invite
DELETE /v1/tenants/{slug}/members/{userID}
GET    /v1/tenants/{slug}/usage                  ?window=24h|7d|30d
GET    /v1/tenants/{slug}/audit-log              ?limit&action&before (admin scope)
GET    /v1/tenants/{slug}/api-keys
POST   /v1/tenants/{slug}/api-keys
DELETE /v1/tenants/{slug}/api-keys/{id}

# Embed
POST   /v1/embed/tokens
GET    /v1/embed/bootstrap
GET    /v1/embed/snippets
GET    /v1/embed/snippets/{id}
GET    /v1/embed/snippets/{id}/metrics
GET    /v1/embed/snippets/{id}/logs

# Auth
POST   /v1/admin/auth/register
POST   /v1/admin/auth/login
POST   /v1/admin/auth/logout
POST   /v1/admin/auth/refresh
GET    /v1/admin/auth/me
GET    /.well-known/jwks.json
```

---

## Project Structure

```
velane/
├── services/
│   ├── control-plane/          # Go API server (chi, pgx, zap)
│   │   ├── cmd/server/         # main.go, bootstrap.go
│   │   └── internal/
│   │       ├── api/            # router, handlers, middleware
│   │       ├── auth/           # JWT + password auth providers
│   │       ├── audit/          # append-only audit logger
│   │       ├── config/         # env-based config
│   │       ├── executor/       # Executor interface + remote/firecracker impls
│   │       ├── models/         # domain types
│   │       ├── observability/  # post-invocation pipeline
│   │       ├── scheduler/      # sync/async/stream invocation
│   │       ├── store/
│   │       │   ├── postgres/   # all DB queries + migrations (001–008)
│   │       │   └── redis/      # job queue + warm pool
│   │       ├── tenantprovider/ # shared vs namespaced executor routing
│   │       └── worker/         # async job processor + webhook delivery
│   ├── executor-runtime/
│   │   ├── bun/                # Bun HTTP server (runner.ts)
│   │   └── python/             # Python FastAPI server (runner.py)
│   ├── mcp-server/             # MCP protocol server (HTTP/SSE + stdio)
│   └── cli/                    # velane CLI (cobra)
└── apps/
    ├── admin/                  # Admin portal — snippet editor + management (Vite + React)
    └── embed-dashboard/        # Embeddable iframe viewer (Vite + React)
```

---

## Development

### Running tests

```bash
# Control plane unit tests (no external services required)
cd services/control-plane
go test ./...

# With integration tests (requires Postgres + Redis)
TEST_DATABASE_URL=postgres://velane:velane@localhost:5432/velane \
TEST_REDIS_URL=localhost:6379 \
go test ./...

# MCP server
cd services/mcp-server && go test ./...
```

### Running frontends locally

```bash
# Admin portal — http://localhost:5173
cd apps/admin && npm install && npm run dev

# Embed dashboard — http://localhost:5174
cd apps/embed-dashboard && npm install && npm run dev
```

Both proxy `/api/*` to `http://localhost:8080` automatically.

### Useful make targets

```bash
make up      # docker compose up --build -d
make down    # docker compose down -v
make logs    # tail control-plane logs
make build   # compile control-plane locally
make tidy    # go mod tidy
```

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://velane:velane@localhost:5432/velane` | Postgres DSN |
| `REDIS_URL` | `localhost:6379` | Redis address |
| `BUN_EXECUTOR_URL` | `http://localhost:8081` | Bun executor URL |
| `PYTHON_EXECUTOR_URL` | `http://localhost:8082` | Python executor URL |
| `PORT` | `8080` | API listen port |
| `WORKER_COUNT` | `5` | Async worker concurrency |
| `ENCRYPTION_KEY` | *(ephemeral)* | 64-char hex AES-256 key for secret encryption |
| `JWT_PRIVATE_KEY` | *(ephemeral)* | RS256 private key PEM |
| `EXECUTOR_TYPE` | `process` | `process` or `firecracker` |
| `S3_ENDPOINT` | *(AWS)* | S3-compatible endpoint (e.g. `http://minio:9000`) |
| `S3_BUCKET` | `velane-logs` | Bucket for invocation logs |
| `CLICKHOUSE_ADDR` | — | ClickHouse native address |
| `REPLAY_ENABLED` | `false` | Store input payloads for invocation replay |
| `BOOTSTRAP_EMAIL` | — | First-boot admin email (remove after first start) |
| `BOOTSTRAP_PASSWORD` | — | First-boot admin password (remove after first start) |
| `BOOTSTRAP_TENANT` | `default` | First-boot tenant slug |

> **Production:** Always set `ENCRYPTION_KEY` and `JWT_PRIVATE_KEY` to stable values. Ephemeral fallbacks mean secrets cannot be decrypted and all JWTs are invalidated on every restart.

---

## Firecracker (VM-boundary isolation)

For hardware-level isolation, Velane supports [AWS Firecracker](https://firecracker-microvm.github.io/) microVMs:

```yaml
EXECUTOR_TYPE: firecracker
FIRECRACKER_BINARY: /usr/local/bin/firecracker
FIRECRACKER_JAILER_BINARY: /usr/local/bin/jailer
FIRECRACKER_BUN_ROOTFS: /images/bun-rootfs.ext4
FIRECRACKER_PYTHON_ROOTFS: /images/python-rootfs.ext4
FIRECRACKER_KERNEL_IMAGE: /images/vmlinux
```

Requires `/dev/kvm` — runs on bare metal servers, AWS metal instances (e.g. `c5.metal`), GCP bare metal, and Hetzner dedicated. Falls back gracefully with a clear error on non-KVM hosts.

---

## Roadmap

- [ ] OIDC / OAuth2 social login (Google, GitHub)
- [ ] SAML for enterprise SSO
- [ ] Real ClickHouse metrics writes (currently Postgres-backed stubs)
- [ ] Real S3/MinIO log writes (interfaces wired, writes stubbed)
- [ ] Firecracker rootfs image builder
- [ ] Kubernetes Helm chart
- [ ] Terraform modules (AWS / GCP)
- [ ] OpenAPI spec generation from snippet code

---

## Contributing

1. Fork the repo
2. Create a branch: `git checkout -b feat/my-feature`
3. Run tests: `cd services/control-plane && go test ./...`
4. Open a pull request

All Go changes must pass `go vet ./...` and `go test ./...`. Frontend changes should build without TypeScript errors (`tsc --noEmit`).

---

## License

Velane is dual-licensed:

- **AGPLv3** (open source) — free to use, modify, and self-host under the terms of the [GNU Affero General Public License v3.0](LICENSE). Any modifications you deploy over a network must also be released under AGPLv3.
- **Commercial license** — if you want to use Velane in a proprietary product, embed it in a SaaS offering, or cannot comply with AGPLv3's copyleft requirements, a commercial license is available. Contact **abskrj@icloud.com** to discuss pricing.

> **In short:** build on it freely if you open-source your work. Pay for a commercial license if you keep your code closed.
