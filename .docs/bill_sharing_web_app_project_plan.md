# Project Plan: Bill-Sharing & Group Expense Management Web App

## 1. Project Overview
A lightweight, self-hosted web application designed to manage shared expenses among individuals and composite groups (e.g., couples, families). The service prioritizes clear financial splitting (both even and ratio-based), conflict-free participant selection, and verifiable bill evidence through multi-file attachments.

---

## 2. Technology Stack
* **Git:**
  * **Repo:** Init new repo for this in account WilliamTrung 
* **Deployement:**
  * **Docker:** Use docker compose for web app backend stack deployment in MSI server
  * **CICD:** Use cicd-polling-job in MSI server, register cloudflare service and config for domain: bill-sharing.williamthanhtrung.id.vn
* **Backend:** Go (Golang 1.22+)
  * **Router:** Standard library `net/http` (utilizing Go 1.22+ enhanced routing) or `chi` / `echo`.
  * **Database:** PostgreSQL - use database container in msi server.
  * **File Storage:** Local filesystem (`./uploads/bills/{bill_id}/`) served via authenticated/controlled static file endpoints.
* **Frontend:** Vanilla HTML5, CSS3, and Modern JavaScript (ES6+)
  * **Styling:** Custom CSS with CSS Variables (or lightweight utility CSS via CDN if desired, keeping zero build-step architecture).
  * **State & API:** Native `fetch()` API, modular JS files (`api.js`, `bill-splitter.js`, `admin.js`), and client-side validation.
* **Test:**
  * **Test:** Test directly on msi server, no local run
---

## 3. Core Functional Requirements

### 3.1. Member Management (Admin Page)
* **Create Member:** Add new individuals with attributes (`name`, `phone`, `email`, `notes`, `is_active`).
* **Modify Member:** Edit existing member details or soft-delete/deactivate members so historical bills remain intact.
* **Member Detail View & Quick Group Creation:**
  * Inside a specific member's detail page, provide a **"Create Group with this Member"** action.
  * Triggering this opens a side pane/modal listing all other active members.
  * Admin selects co-members, names the group (e.g., *"Alex & Sam"*), and optionally sets internal default split ratios within that group.

### 3.2. Group Member Management (Admin Page)
* **Group Concept:** A Group acts as a single selectable entity (alias) representing multiple underlying individual members.
* **Create & Modify Groups:**
  * Assign a custom Group Name.
  * Add/remove members from the group.
  * Configure internal group split behavior (default: **Even Split** among group members; optional: **Custom Ratio** within the group, e.g., 60/40).

### 3.3. Bill Creation & Splitting Engine
* **Bill Metadata:**
  * Total Bill Amount (stored as integer cents/smallest currency unit to avoid floating-point precision bugs).
  * Description / Purpose (what the bill paid for).
  * Date of expense & Payer selection (who paid upfront).
* **Participant Selection & Conflict Prevention:**
  * Users can select a mix of **Individual Members** and **Group Members**.
  * **Overlap Validation (Crucial Business Rule):**
    * Prevent selecting two groups that share a common member.
    * Prevent selecting an individual member if a group containing that member is already selected (and vice versa).
    * *UI Behavior:* Dynamically disable/grey out conflicting checkboxes in real time with a tooltip explaining the overlap (e.g., *"Disabled: John is already included in 'John & Jane'"*).
* **Split Calculation Modes:**
  * **Default (Even Share):** Total amount divided equally among all selected top-level entities (individuals and groups). If a group is selected, its allocated share is then subdivided among its members (evenly or by the group's internal ratio).
  * **Ratio Set Share:** Assign custom weights/ratios (e.g., `2 : 1 : 1` or percentages) to each selected participant/group.
  * **Remainder Handling:** Penny/cent remainders from uneven divisions are deterministically assigned (e.g., to the first participant or largest shareholder) so the sum of splits strictly equals the total bill amount.
* **Multi-File Evidence Upload:**
  * Support uploading multiple files (images: `.jpg`, `.png`, `.webp`, and `.pdf` receipts) via `multipart/form-data`.
  * Preview thumbnails before submission and view/download attachments on the bill detail page.

---

## 4. Database Schema Design

```sql
-- Individual Members
CREATE TABLE members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    phone TEXT,
    avatar_color TEXT,
    is_active BOOLEAN DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Composite Groups (Aliases)
CREATE TABLE member_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    internal_split_type TEXT DEFAULT 'EVEN', -- 'EVEN' or 'RATIO'
    is_active BOOLEAN DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Group Membership Mapping
CREATE TABLE group_members (
    group_id INTEGER NOT NULL,
    member_id INTEGER NOT NULL,
    internal_ratio REAL DEFAULT 1.0,
    PRIMARY KEY (group_id, member_id),
    FOREIGN KEY (group_id) REFERENCES member_groups(id) ON DELETE CASCADE,
    FOREIGN KEY (member_id) REFERENCES members(id) ON DELETE CASCADE
);

-- Bills Header
CREATE TABLE bills (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT,
    total_amount_cents INTEGER NOT NULL,
    paid_by_member_id INTEGER NOT NULL,
    split_mode TEXT NOT NULL DEFAULT 'EVEN', -- 'EVEN' or 'RATIO'
    bill_date DATE NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (paid_by_member_id) REFERENCES members(id)
);

-- Top-Level Bill Participants (Tracks how the bill was split at the UI selection level)
CREATE TABLE bill_participants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    bill_id INTEGER NOT NULL,
    participant_type TEXT NOT NULL, -- 'MEMBER' or 'GROUP'
    member_id INTEGER,              -- Populated if participant_type == 'MEMBER'
    group_id INTEGER,               -- Populated if participant_type == 'GROUP'
    ratio_weight REAL DEFAULT 1.0,
    allocated_amount_cents INTEGER NOT NULL,
    FOREIGN KEY (bill_id) REFERENCES bills(id) ON DELETE CASCADE
);

-- Resolved Individual Ledger (Flattened per-member breakdown for debt calculation)
CREATE TABLE bill_member_shares (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    bill_id INTEGER NOT NULL,
    member_id INTEGER NOT NULL,
    source_group_id INTEGER, -- NULL if added directly, otherwise references group_id
    owed_amount_cents INTEGER NOT NULL,
    FOREIGN KEY (bill_id) REFERENCES bills(id) ON DELETE CASCADE,
    FOREIGN KEY (member_id) REFERENCES members(id)
);

-- Evidence Attachments
CREATE TABLE bill_attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    bill_id INTEGER NOT NULL,
    original_filename TEXT NOT NULL,
    stored_filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    file_size_bytes INTEGER NOT NULL,
    uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (bill_id) REFERENCES bills(id) ON DELETE CASCADE
);
```

---

## 5. REST API Endpoints Specification

### Members & Groups (Admin)
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/members` | List all active members (with their associated group IDs). |
| `POST` | `/api/members` | Create a new member. |
| `GET` | `/api/members/{id}` | Get member details and groups they belong to. |
| `PUT` | `/api/members/{id}` | Update member info or active status. |
| `GET` | `/api/groups` | List all groups with nested member arrays. |
| `POST` | `/api/groups` | Create a group with member IDs and internal ratios. |
| `PUT` | `/api/groups/{id}` | Update group name, members, or internal split settings. |

### Bills & Evidence
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/bills` | List bills with summary info and attachment counts. |
| `POST` | `/api/bills` | Create bill (`multipart/form-data`: JSON payload + multiple files). |
| `GET` | `/api/bills/{id}` | Get full bill details, group/individual breakdown, and attachment URLs. |
| `DELETE` | `/api/bills/{id}` | Delete a bill and clean up associated uploaded files. |
| `GET` | `/uploads/{bill_id}/{file}` | Serve uploaded evidence images/PDFs safely. |

---

## 6. Project Directory Structure

```text
bill-splitter/
├── cmd/
│   └── server/
│       └── main.go              # Entry point, router setup, DB initialization
├── internal/
│   ├── config/
│   │   └── config.go            # Port, DB path, upload dir, max upload size
│   ├── database/
│   │   ├── db.go                # SQLite connection & migration runner
│   │   └── schema.sql           # Embedded SQL DDL
│   ├── models/
│   │   └── models.go            # Go structs for JSON & DB mapping
│   ├── handlers/
│   │   ├── member_handler.go    # CRUD for members & quick-group creation
│   │   ├── group_handler.go     # CRUD for member groups
│   │   └── bill_handler.go      # Bill creation, multipart upload, validation
│   └── service/
│       ├── splitter.go          # Core math engine (Even, Ratio, Group breakdown, Cent rounding)
│       └── validator.go         # Overlap detection (prevent duplicate members across groups)
├── web/                         # Static Frontend served by Go's http.FileServer
│   ├── index.html               # Dashboard & Bill List
│   ├── create-bill.html         # Bill Entry, Dynamic Split Calculator & File Dropzone
│   ├── bill-detail.html         # Receipt viewer & itemized split breakdown
│   ├── admin.html               # Manage Members & Groups (with slide-over pane)
│   ├── css/
│   │   └── styles.css           # Responsive layout, cards, slide-over drawer, disabled states
│   └── js/
│       ├── api.js               # Fetch wrappers & error handling
│       ├── splitter-ui.js       # Real-time conflict detection & live split preview
│       └── admin-ui.js          # Member/Group management & slide-over selection pane
├── uploads/                     # Git-ignored directory for receipt evidence
├── go.mod
└── README.md
```

---

## 7. Key Business Logic & Edge Cases

### 7.1. Overlap Validation Algorithm (Backend & Frontend)
When a bill is submitted with `selected_member_ids` and `selected_group_ids`:
1. Initialize an empty set `seen_members = map[int]string{}` (maps `member_id` -> source name).
2. Iterate through `selected_member_ids`:
   * Add `member_id` to `seen_members` with source `"Individual"`.
3. Iterate through `selected_group_ids`:
   * Fetch all `member_id`s belonging to the group.
   * For each `member_id`:
     * If `member_id` already exists in `seen_members`, **reject request (HTTP 400)** with message: *"Conflict: Member [Name] is selected multiple times via [Source 1] and [Group Name]."*
     * Otherwise, record `seen_members[member_id] = group.Name`.

### 7.2. Two-Tier Split Calculation (`internal/service/splitter.go`)
1. **Tier 1 (Top-Level Entities):**
   * Convert `total_amount` to integer cents (e.g., `$100.00` $\rightarrow$ `10000`).
   * Calculate each selected entity's share based on `EVEN` ($1$ weight each) or `RATIO` (user-defined weights).
   * Distribute any remainder cents (`total_cents % count` or rounding residuals) so $\sum \text{EntityShares} = \text{TotalCents}$.
2. **Tier 2 (Group Expansion):**
   * If a top-level entity is an **Individual**, their `owed_amount_cents` equals their Tier 1 share.
   * If a top-level entity is a **Group**, take the group's Tier 1 allocated cents and run the same deterministic splitter across the group's members using the group's internal split rule (`EVEN` or `RATIO`).
   * Store both the Tier 1 (`bill_participants`) and Tier 2 (`bill_member_shares`) records so users can see *"Couple A owes $50.00 ($25.00 each)"*.

### 7.3. File Upload Security & Handling
* Limit total request size (e.g., `r.Body = http.MaxBytesReader(w, r.Body, 25 << 20)` for 25MB max per bill).
* Sniff actual MIME types using `http.DetectContentType` (allow only `image/jpeg`, `image/png`, `image/webp`, `application/pdf`).
* Rename stored files using UUIDs (`uuid.New().String() + ext`) to prevent path traversal and filename collisions, while preserving `original_filename` in SQLite for display.

---

## 8. Phased Implementation Roadmap

### Phase 1: Foundation & Database (Days 1–2)
* Initialize Go module and directory layout.
* Set up SQLite database connection and run `schema.sql` migrations on startup.
* Implement basic Go `http.FileServer` serving `./web` alongside `/api/` routes.

### Phase 2: Admin APIs & Management UI (Days 3–5)
* Build CRUD endpoints for `members` and `member_groups`.
* Create `admin.html` and `admin-ui.js`:
  * Member table with inline/modal editing.
  * Member Detail view featuring the **Slide-Over Pane** to pick other members and instantiate a named Group directly.

### Phase 3: Splitting Engine & Conflict Detection (Days 6–8)
* Implement `validator.go` (member overlap guard) and `splitter.go` (exact cent distribution + two-tier group splitting) with comprehensive Go unit tests (`splitter_test.go`).
* Build `create-bill.html` and `splitter-ui.js`:
  * Live preview table updating as amounts, ratios, or checkboxes change.
  * Real-time disabling of overlapping individuals/groups in the UI.

### Phase 4: Evidence Uploads & Bill Views (Days 9–11)
* Implement `multipart/form-data` parsing in `bill_handler.go` to save multiple attachments atomically with the database transaction (rollback file writes if DB insert fails).
* Build `index.html` (bill history & net balances summary) and `bill-detail.html` (lightbox gallery for uploaded receipts + full itemized split breakdown).

### Phase 5: Polish & Deployment (Days 12–14)
* Add mobile-responsive CSS styling (important for snapping receipt photos on phones).
* Configure single-binary build (`go embed` for static frontend assets optional, or Docker Compose setup with persistent volumes for SQLite and `./uploads`).

---

## 9. Implementation Notes (as built)

These points differ from the original draft above and reflect the shipped code:

* **Database is PostgreSQL, not SQLite.** The MSI server hosts a Postgres container, so `internal/database/schema.sql` uses Postgres types (`BIGSERIAL`, `TIMESTAMPTZ`, `BOOLEAN`) and the app connects through `pgx`. Set `DATABASE_URL` accordingly.
* **Frontend is served from disk, not embedded.** `WEB_DIR` (default `./web`) is read at startup via `os.DirFS`; the Docker image copies `web/` next to the binary. Embedding remains optional.
* **Account authentication exists.** Users live in the `users` table with bcrypt password hashes and sign in with **username or email** plus password. A single seed account is created on startup from `SEED_USERNAME`/`SEED_EMAIL`/`SEED_PASSWORD`. On success the server issues an HMAC-signed `bs_session` cookie (`SESSION_SECRET`) that carries the user id. Login attempts are rate limited per client IP.
* **One table for people.** `members` was folded into `users` (migration in `internal/database/merge_members.sql`); bill-only people are user rows with a NULL `password_hash` and a `display_name`. The `/api/members` API keeps its JSON shape over that table.
* **Self-service profile.** Signed-in users edit display name, username, email, default language and password at `profile.html` via `GET/PUT /api/profile` and `PUT /api/profile/password` (current password required). Username/email collisions return 409.
* **Invitation signup.** A member added from the admin page gets a password-less `users` row, so they can be included in bills but cannot log in yet. The server emails a signed activation link (`activate.html?token=…`, HMAC with `SESSION_SECRET`, 7-day TTL, single use) built from `PUBLIC_BASE_URL`; `GET /api/public/invite` describes it and `POST /api/public/invite/accept` sets the first password and signs the member in. `POST /api/members/{id}/invite` re-sends it.
* **Public and authenticated routes.** `/`, `/api/public/bills`, `/api/public/bills/{id}`, `/uploads/{billID}/{file}` and `bill-detail.html` are public read-only. Members, groups, bill creation, deletion and `PUT /api/bills/{id}/status` require a session; other HTML pages redirect to `/login.html`.
* **Admin-only management.** `ADMIN_EMAILS` decides who may write members/groups and resend invites. The Go service resolves the caller from the session cookie, re-reads the account, and checks its email against the list on every request: `/admin` returns 302 to the dashboard for non-admins and the write endpoints return 403. Hiding buttons in the admin page is cosmetic only.
* **Bill status.** `bills.status` is `OPEN` or `SETTLED`. New bills start `OPEN`; signed-in users can toggle it from the bill detail page, and the public listing defaults to ongoing bills with an "All" filter.
* **Bilingual UI (English + Vietnamese).** `web/js/i18n.js` holds both dictionaries, detects the language from storage / `?lang=` / the account default / `navigator.languages`, and mounts a switcher in every top bar. Each account's `language` (default `en`) is the fallback for devices with no local choice, read from `/api/session`. Switching re-renders dynamic tables and toasts in place without a reload.
* **Deployment files:** `Dockerfile` (multi-stage, non-root Alpine runtime) and `docker-compose.yml` (Postgres + app with `db-data` and `uploads` volumes). Secrets are required via `.env`; see `.env.example`.
