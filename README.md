# Bill Sharing

Self-hosted web app for splitting shared expenses between individuals and composite groups (couples, families). Bills can be split evenly or by ratio, with multi-file receipt evidence attached.

## Stack

- **Backend:** Go 1.23, standard library `net/http` routing, PostgreSQL via `pgx`
- **Frontend:** Vanilla HTML/CSS/ES2020, zero build step, served by the Go binary
- **Auth:** Email/password accounts (bcrypt), signing in with username **or** email; HMAC-signed session cookie (HttpOnly, SameSite=Lax). Reading bills is public; writing requires a session.
- **Languages:** English and Vietnamese with a UI switcher; each account stores a default language (English if unset)

## Running

```sh
cp .env.example .env   # then edit the secrets
docker compose up -d --build
```

The app is published on `APP_HOST_PORT` (default `8080`). Uploaded receipts use the `uploads` volume. Set `DATABASE_URL` to your Postgres (on MSI: `shared-postgres` via `db-net`; run `deploy/msi-init-db.sh` once).

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
| `SEED_USERNAME` | no | email local-part | Seed account username (min 3 chars) |
| `SEED_EMAIL` | yes | – | Seed account email, also used to sign in |
| `SEED_PASSWORD` | yes | – | Seed account password (min 8 chars) |
| `SESSION_SECRET` | yes | – | Minimum 16 characters, signs session cookies |
| `PORT` | no | `8080` | Listen port |
| `UPLOAD_DIR` | no | `./uploads` | Receipt storage root |
| `WEB_DIR` | no | `./web` | Static frontend root |
| `COOKIE_SECURE` | no | `true` | Set `false` for plain HTTP only |
| `MAX_UPLOAD_BYTES` | no | `26214400` | Per-bill upload limit (25 MB) |
| `CURRENCY` | no | `$` | Symbol shown in the UI |
| `SMTP_HOST` | no | `smtp.gmail.com` | SMTP server (Gmail) |
| `SMTP_PORT` | no | `587` | SMTP port (STARTTLS) |
| `SMTP_USER` | no | – | Gmail address; with `SMTP_PASSWORD`, enables member welcome email |
| `SMTP_PASSWORD` | no | – | Gmail [app password](https://support.google.com/accounts/answer/185833) |
| `SMTP_FROM` | no | `SMTP_USER` | From header |
| `SMTP_FROM_NAME` | no | `Share da Bill` | Display name recipients see instead of the address |
| `PUBLIC_BASE_URL` | no | `http://bill-sharing.williamthanhtrung.id.vn` | Public origin used to build invitation links |
| `ADMIN_EMAILS` | no | seed email | JSON array or CSV of emails allowed to manage members/groups |

`docker-compose.yml` contains no secrets or environment values of its own — it reads everything from `env_file` (the deploy env file, `deploy/deploy.env.example` as a template). Only `PORT`, `UPLOAD_DIR` and `WEB_DIR` are set inside the container.

`ADMIN_EMAILS` (JSON array or comma-separated) lists the accounts allowed to add/edit members and groups and to resend invitations. Authorization is enforced by the Go service: the account is re-read from the database on every request and checked against the list, so `/admin` redirects non-admins to the dashboard and member/group write endpoints return `403` regardless of what the browser does. The seed email is always an admin. The admin page still hides its controls for non-admins, but that is a convenience, not the security boundary.

People live in a single `users` table: login rows have a `password_hash`; bill-only people are created with a null password and a `display_name` for splitting. The `/api/members` routes still use that same table (column names in JSON stay `name`, `member_id`, etc.). Each row also stores a `language` (`en`/`vi`, default `en`) used as the interface language on devices that have not picked one; `/api/session` returns it to the client.

Every account also has `is_public` (default `false`). While it is off, `/api/members` returns `email`, `phone` and `notes` only to admins and to the account owner; other signed-in users get those fields blanked. The display name always stays visible because it is how participants are picked. The owner flips it from `/profile.html`.

### How a new member signs in

Adding a member creates a **password-less** account, so it can take part in bills immediately but cannot log in yet. When the member has an email and SMTP is configured, the server emails a signed activation link (`http://bill-sharing.williamthanhtrung.id.vn/activate.html?token=…`, from `PUBLIC_BASE_URL`, valid 7 days). The member opens it, sets a password, and is signed in; from then on they sign in with that username or email. Admins can re-send the link from the members list or `POST /api/members/{id}/invite`. Tokens are HMAC-signed with `SESSION_SECRET` and stop working once a password is set.

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

Public (no session):

| Method | Endpoint |
| --- | --- |
| `POST` | `/api/login` (`{"identifier","password"}` — username or email) |
| `GET` | `/api/config` |
| `GET` | `/api/public/bills` |
| `GET` | `/api/public/bills/{id}` |
| `GET` | `/api/public/invite?token=…` |
| `POST` | `/api/public/invite/accept?token=…` |
| `GET` | `/uploads/{billID}/{filename}` |

Authenticated:

| Method | Endpoint |
| --- | --- |
| `POST` | `/api/logout` |
| `GET` | `/api/session` |
| `GET`/`PUT` | `/api/profile` |
| `PUT` | `/api/profile/password` |
| `GET`/`POST` | `/api/members` |
| `GET`/`PUT` | `/api/members/{id}` |
| `POST` | `/api/members/{id}/invite` |
| `GET`/`POST` | `/api/groups` |
| `GET`/`PUT` | `/api/groups/{id}` |
| `GET`/`POST` | `/api/bills` |
| `GET`/`DELETE` | `/api/bills/{id}` |
| `PUT` | `/api/bills/{id}/status` (`{"status":"OPEN"\|"SETTLED"}`) |
| `GET` | `/api/balances` |

`POST /api/bills` accepts `multipart/form-data` with a `payload` JSON field and up to 10 `files` (JPEG, PNG, WebP, PDF; MIME sniffed server-side, stored under random names).

The public home page is `/` and lists every bill with its status and shared members; `/bill-detail.html?id=N` and its receipt evidence are also public. Adding bills and managing members, groups and statuses requires signing in at `/login.html`; those pages redirect there when signed out. Signed-in users edit their display name, username, email, default language and password at `/profile.html`.
