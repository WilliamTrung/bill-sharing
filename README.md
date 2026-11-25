# Bill Sharing

Self-hosted web app for splitting shared expenses between individuals and composite groups (couples, families). Bills can be split evenly or by ratio, with multi-file receipt evidence attached.

## Stack

- **Backend:** Go 1.23, standard library `net/http` routing, PostgreSQL via `pgx`
- **Frontend:** Vanilla HTML/CSS/ES2020, zero build step, served by the Go binary
- **Auth:** Password sign-in issuing an HMAC-signed session cookie (HttpOnly, SameSite=Lax)
- **Languages:** English and Vietnamese with a UI switcher

## Running

```sh
cp .env.example .env   # then edit the secrets
docker compose up -d --build
```

The app is published on `APP_HOST_PORT` (default `8080`). Postgres data and uploaded receipts live in named volumes (`db-data`, `uploads`).

## Deploy (MSI cicd-polling)

- Host: **msi**, domain: `http://bill-sharing.williamthanhtrung.id.vn`
- Secrets: `dev/shared/bill-sharing/secrets/deploy.env` on MSI (see `deploy/deploy.env.example`)
- One-time edge: `bash deploy/msi-bootstrap.sh` on MSI after the repo is cloned under `/home/tuf/repo/bill-sharing`
- Release: merge to `main`, tag `vX.Y.Z`, push branch + tag; monitor  
  `ssh msi "docker logs --tail 300 cicd-polling-job 2>&1 | grep bill-sharing"`

### Configuration

| Variable | Required | Default | Notes |
| --- | --- | --- | --- |
| `DATABASE_URL` | yes | – | Postgres connection string |
| `ADMIN_PASSWORD` | yes | – | Minimum 8 characters |
| `SESSION_SECRET` | yes | – | Minimum 16 characters, signs session cookies |
| `PORT` | no | `8080` | Listen port |
| `UPLOAD_DIR` | no | `./uploads` | Receipt storage root |
| `WEB_DIR` | no | `./web` | Static frontend root |
| `COOKIE_SECURE` | no | `true` | Set `false` for plain HTTP only |
| `MAX_UPLOAD_BYTES` | no | `26214400` | Per-bill upload limit (25 MB) |
| `CURRENCY` | no | `$` | Symbol shown in the UI |

`docker-compose.yml` reads `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, `APP_PORT` from the environment as well.

## Development

No local toolchain is required if Docker is available:

```sh
docker run --rm -v "$PWD:/app" -w /app golang:1.23 go test ./...
docker run --rm -v "$PWD:/app" -w /app golang:1.23 go vet ./...
```

Or, with Go installed: `go test ./... && go run ./cmd/server`.

The database schema in `internal/database/schema.sql` is embedded in the binary and applied on startup, so no migration step is needed.

## Layout

```text
cmd/server/main.go              Entry point, graceful shutdown
internal/auth/                  Session cookie signing and validation
internal/config/                Environment-backed configuration
internal/database/              Connection pool, embedded schema
internal/handlers/              HTTP handlers (members, groups, bills, uploads, auth)
internal/models/                JSON/DB structs
internal/service/               Split math engine and overlap validator (unit tested)
web/                            Static frontend (pages, css/, js/)
```

## How splitting works

1. **Tier 1** divides the bill total across the selected top-level participants (individuals and groups) using even weights or user-provided ratios. Amounts are integers in cents and the largest-remainder method guarantees the shares sum exactly to the total.
2. **Tier 2** expands each selected group: the group's allocated share is divided among its own members by the group's internal rule (`EVEN` or `RATIO`).

Both tiers are stored, so the UI can show `"Couple A owes $50.00 ($25.00 each)"`.

Overlapping selections are rejected server-side with HTTP 400 and disabled client-side in real time — an individual cannot appear both directly and inside a selected group, nor can two selected groups share a member.

## API

| Method | Endpoint |
| --- | --- |
| `POST` | `/api/login`, `/api/logout` |
| `GET` | `/api/session`, `/api/config` |
| `GET`/`POST` | `/api/members` |
| `GET`/`PUT` | `/api/members/{id}` |
| `GET`/`POST` | `/api/groups` |
| `GET`/`PUT` | `/api/groups/{id}` |
| `GET`/`POST` | `/api/bills` |
| `GET`/`DELETE` | `/api/bills/{id}` |
| `GET` | `/api/balances` |
| `GET` | `/uploads/{billID}/{filename}` |

`POST /api/bills` accepts `multipart/form-data` with a `payload` JSON field and up to 10 `files` (JPEG, PNG, WebP, PDF; MIME sniffed server-side, stored under random names).

All `/api` and `/uploads` routes require a valid session. The public home page is `/`; the bills dashboard is `/dashboard.html`; members and groups live at `/admin`. Other HTML pages redirect to `/login.html` when signed out.
