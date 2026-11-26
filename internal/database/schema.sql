CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    email TEXT NOT NULL,
    password_hash TEXT,
    display_name TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    avatar_color TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_color TEXT NOT NULL DEFAULT '';
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS language TEXT NOT NULL DEFAULT 'en';
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_language_check;
ALTER TABLE users ADD CONSTRAINT users_language_check CHECK (language IN ('en', 'vi'));

-- When false (the default), only the account owner and admins see contact details.
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users (lower(username));
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (lower(email));

-- Projects are the tenant boundary: every bill and group belongs to exactly one,
-- and a user may belong to many. Accounts stay global (a user is a person, not a
-- tenant), so joining another project is a membership row rather than a new user.
CREATE TABLE IF NOT EXISTS projects (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    -- Unguessable token behind the anonymous read-only share link.
    public_slug TEXT NOT NULL,
    owner_user_id BIGINT NOT NULL,
    public_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_slug ON projects (public_slug);
CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_owner_name ON projects (owner_user_id, lower(name));

-- Who can see a project. role='owner' may rename, remove members and delete it;
-- role='member' may add members and work with bills and groups.
CREATE TABLE IF NOT EXISTS project_members (
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'member')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members (user_id);

CREATE TABLE IF NOT EXISTS member_groups (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    internal_split_type TEXT NOT NULL DEFAULT 'EVEN' CHECK (internal_split_type IN ('EVEN', 'RATIO')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS group_members (
    group_id BIGINT NOT NULL REFERENCES member_groups(id) ON DELETE CASCADE,
    member_id BIGINT NOT NULL,
    internal_ratio DOUBLE PRECISION NOT NULL DEFAULT 1.0 CHECK (internal_ratio > 0),
    PRIMARY KEY (group_id, member_id)
);

-- The account that owns (and may edit) a group. NULL for legacy groups, which
-- only admins can manage.
ALTER TABLE member_groups ADD COLUMN IF NOT EXISTS leader_user_id BIGINT;

-- Tenant owner of the group. Nullable only until the Phase 3 backfill below runs.
ALTER TABLE member_groups ADD COLUMN IF NOT EXISTS project_id BIGINT;

CREATE TABLE IF NOT EXISTS bills (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    total_amount_cents BIGINT NOT NULL CHECK (total_amount_cents > 0),
    paid_by_member_id BIGINT NOT NULL,
    split_mode TEXT NOT NULL DEFAULT 'EVEN' CHECK (split_mode IN ('EVEN', 'RATIO')),
    bill_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE bills ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'OPEN';
ALTER TABLE bills DROP CONSTRAINT IF EXISTS bills_status_check;
ALTER TABLE bills ADD CONSTRAINT bills_status_check CHECK (status IN ('OPEN', 'SETTLED'));

-- Tenant owner of the bill. Nullable only until the Phase 3 backfill below runs.
ALTER TABLE bills ADD COLUMN IF NOT EXISTS project_id BIGINT;

CREATE TABLE IF NOT EXISTS bill_participants (
    id BIGSERIAL PRIMARY KEY,
    bill_id BIGINT NOT NULL REFERENCES bills(id) ON DELETE CASCADE,
    participant_type TEXT NOT NULL CHECK (participant_type IN ('MEMBER', 'GROUP')),
    member_id BIGINT,
    group_id BIGINT REFERENCES member_groups(id),
    ratio_weight DOUBLE PRECISION NOT NULL DEFAULT 1.0 CHECK (ratio_weight > 0),
    allocated_amount_cents BIGINT NOT NULL CHECK (allocated_amount_cents >= 0),
    CHECK (
        (participant_type = 'MEMBER' AND member_id IS NOT NULL AND group_id IS NULL) OR
        (participant_type = 'GROUP' AND group_id IS NOT NULL AND member_id IS NULL)
    )
);

-- Optional per-participant note for this bill (e.g. "extra dessert").
ALTER TABLE bill_participants ADD COLUMN IF NOT EXISTS note TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS bill_member_shares (
    id BIGSERIAL PRIMARY KEY,
    bill_id BIGINT NOT NULL REFERENCES bills(id) ON DELETE CASCADE,
    member_id BIGINT NOT NULL,
    source_group_id BIGINT REFERENCES member_groups(id) ON DELETE SET NULL,
    owed_amount_cents BIGINT NOT NULL CHECK (owed_amount_cents >= 0)
);

-- A single member's share can be settled while the bill stays open.
ALTER TABLE bill_member_shares ADD COLUMN IF NOT EXISTS is_settled BOOLEAN NOT NULL DEFAULT FALSE;

-- Set when the debtor says "I have paid"; the payer then approves or rejects.
ALTER TABLE bill_member_shares ADD COLUMN IF NOT EXISTS settle_requested_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS bill_attachments (
    id BIGSERIAL PRIMARY KEY,
    bill_id BIGINT NOT NULL REFERENCES bills(id) ON DELETE CASCADE,
    original_filename TEXT NOT NULL,
    stored_filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    file_size_bytes BIGINT NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bills_date ON bills (bill_date DESC, id DESC);
-- Tenant-scoped list queries: the bill list is always filtered by project.
CREATE INDEX IF NOT EXISTS idx_bills_project_date ON bills (project_id, bill_date DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_member_groups_project ON member_groups (project_id, name);
CREATE INDEX IF NOT EXISTS idx_bill_participants_bill ON bill_participants (bill_id);
CREATE INDEX IF NOT EXISTS idx_bill_member_shares_bill ON bill_member_shares (bill_id);
CREATE INDEX IF NOT EXISTS idx_bill_member_shares_member ON bill_member_shares (member_id);
CREATE INDEX IF NOT EXISTS idx_bill_attachments_bill ON bill_attachments (bill_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bill_attachments_stored ON bill_attachments (bill_id, stored_filename);
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_members_member ON group_members (member_id, group_id);
CREATE INDEX IF NOT EXISTS idx_member_groups_leader ON member_groups (leader_user_id);

-- FKs to users (member_id columns hold user ids).
DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'group_members_member_id_fkey'
    ) THEN
        ALTER TABLE group_members
            ADD CONSTRAINT group_members_member_id_fkey
            FOREIGN KEY (member_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'bills_paid_by_member_id_fkey'
    ) THEN
        ALTER TABLE bills
            ADD CONSTRAINT bills_paid_by_member_id_fkey
            FOREIGN KEY (paid_by_member_id) REFERENCES users(id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'bill_participants_member_id_fkey'
    ) THEN
        ALTER TABLE bill_participants
            ADD CONSTRAINT bill_participants_member_id_fkey
            FOREIGN KEY (member_id) REFERENCES users(id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'bill_member_shares_member_id_fkey'
    ) THEN
        ALTER TABLE bill_member_shares
            ADD CONSTRAINT bill_member_shares_member_id_fkey
            FOREIGN KEY (member_id) REFERENCES users(id);
    END IF;
END $$;

UPDATE users SET display_name = username WHERE display_name = '' AND username <> '';

-- Adopt pre-project data into a single default project so nothing is orphaned.
--
-- Only runs while legacy rows exist (bills or groups with no project_id), so it
-- adopts once and then never again — a default project the user later deletes
-- does not reappear on the next boot. On a fresh install there is no data and no
-- user yet (this file runs before the seed account is created), so it returns
-- immediately; projects are created in-app from then on.
--
-- Idempotent on repeated boots: the project is keyed by a fixed slug and the
-- UPDATEs only touch rows whose project_id is still NULL.
DO $$
DECLARE
    default_project_id BIGINT;
    default_owner_id BIGINT;
    has_legacy_rows BOOLEAN;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM bills WHERE project_id IS NULL
        UNION ALL
        SELECT 1 FROM member_groups WHERE project_id IS NULL
    ) INTO has_legacy_rows;
    IF NOT has_legacy_rows THEN
        RETURN;
    END IF;

    SELECT id INTO default_owner_id FROM users ORDER BY is_active DESC, id ASC LIMIT 1;
    IF default_owner_id IS NULL THEN
        RETURN;
    END IF;

    INSERT INTO projects (name, public_slug, owner_user_id)
    VALUES ('My bills', 'default', default_owner_id)
    ON CONFLICT (public_slug) DO NOTHING;

    SELECT id INTO default_project_id FROM projects WHERE public_slug = 'default';

    UPDATE bills SET project_id = default_project_id WHERE project_id IS NULL;
    UPDATE member_groups SET project_id = default_project_id WHERE project_id IS NULL;

    -- Everyone who existed before projects becomes a member of the default one;
    -- its owner keeps the owner role.
    INSERT INTO project_members (project_id, user_id, role)
    SELECT default_project_id, u.id,
           CASE WHEN u.id = default_owner_id THEN 'owner' ELSE 'member' END
    FROM users u
    ON CONFLICT (project_id, user_id) DO NOTHING;
END $$;

-- FKs for the tenant owner columns. Added after the backfill so the constraint
-- can be validated against already-populated rows.
DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'bills_project_id_fkey'
    ) THEN
        ALTER TABLE bills
            ADD CONSTRAINT bills_project_id_fkey
            FOREIGN KEY (project_id) REFERENCES projects(id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'member_groups_project_id_fkey'
    ) THEN
        ALTER TABLE member_groups
            ADD CONSTRAINT member_groups_project_id_fkey
            FOREIGN KEY (project_id) REFERENCES projects(id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'projects_owner_user_id_fkey'
    ) THEN
        ALTER TABLE projects
            ADD CONSTRAINT projects_owner_user_id_fkey
            FOREIGN KEY (owner_user_id) REFERENCES users(id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'member_groups_leader_user_id_fkey'
    ) THEN
        ALTER TABLE member_groups
            ADD CONSTRAINT member_groups_leader_user_id_fkey
            FOREIGN KEY (leader_user_id) REFERENCES users(id);
    END IF;
END $$;

-- Every bill and group must now name its project. The backfill above ensured no
-- legacy row is left NULL, and the create paths write the column, so this is the
-- point where the tenant boundary becomes structural rather than a convention.
ALTER TABLE bills ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE member_groups ALTER COLUMN project_id SET NOT NULL;

-- The old global ordering index is superseded by idx_bills_project_date.
DROP INDEX IF EXISTS idx_bills_date;

-- Repair bills settled by an older build, which set only bills.status and left the
-- shares flagged open. Those rows kept showing up in balances and "To settle" as
-- money still owed, so bring the shares in line with the bill status. Idempotent:
-- once a bill's shares are settled this matches nothing.
UPDATE bill_member_shares s
SET is_settled = TRUE, settle_requested_at = NULL
FROM bills b
WHERE b.id = s.bill_id
  AND b.status = 'SETTLED'
  AND (s.is_settled = FALSE OR s.settle_requested_at IS NOT NULL);