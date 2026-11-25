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