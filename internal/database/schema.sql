CREATE TABLE IF NOT EXISTS members (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    phone TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    avatar_color TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

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
    member_id BIGINT NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    internal_ratio DOUBLE PRECISION NOT NULL DEFAULT 1.0 CHECK (internal_ratio > 0),
    PRIMARY KEY (group_id, member_id)
);

CREATE TABLE IF NOT EXISTS bills (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    total_amount_cents BIGINT NOT NULL CHECK (total_amount_cents > 0),
    paid_by_member_id BIGINT NOT NULL REFERENCES members(id),
    split_mode TEXT NOT NULL DEFAULT 'EVEN' CHECK (split_mode IN ('EVEN', 'RATIO')),
    bill_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bill_participants (
    id BIGSERIAL PRIMARY KEY,
    bill_id BIGINT NOT NULL REFERENCES bills(id) ON DELETE CASCADE,
    participant_type TEXT NOT NULL CHECK (participant_type IN ('MEMBER', 'GROUP')),
    member_id BIGINT REFERENCES members(id),
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
    member_id BIGINT NOT NULL REFERENCES members(id),
    source_group_id BIGINT REFERENCES member_groups(id) ON DELETE SET NULL,
    owed_amount_cents BIGINT NOT NULL CHECK (owed_amount_cents >= 0)
);

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
