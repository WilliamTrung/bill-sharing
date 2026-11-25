-- Legacy: fold members into users and re-point foreign keys (runs once when members exists).
DO $$
BEGIN
    IF to_regclass('public.members') IS NULL THEN
        RETURN;
    END IF;

    UPDATE users u
    SET display_name = COALESCE(NULLIF(m.name, ''), u.display_name, u.username),
        phone = m.phone,
        notes = m.notes,
        avatar_color = CASE WHEN u.avatar_color = '' THEN m.avatar_color ELSE u.avatar_color END,
        email = CASE WHEN m.email <> '' THEN m.email ELSE u.email END,
        is_active = m.is_active,
        updated_at = now()
    FROM members m
    WHERE m.user_id = u.id;

    INSERT INTO users (username, email, password_hash, display_name, phone, notes, avatar_color, is_active)
    SELECT
        'member' || m.id::text,
        CASE WHEN m.email <> '' THEN m.email ELSE ('member' || m.id::text || '@users.invalid') END,
        NULL,
        m.name,
        m.phone,
        m.notes,
        m.avatar_color,
        m.is_active
    FROM members m
    WHERE m.user_id IS NULL
      AND NOT EXISTS (
        SELECT 1 FROM users u
        WHERE m.email <> '' AND lower(u.email) = lower(m.email)
      );

    CREATE TEMP TABLE _member_user_map ON COMMIT DROP AS
    SELECT
        m.id AS old_member_id,
        COALESCE(
            m.user_id,
            (SELECT u.id FROM users u WHERE m.email <> '' AND lower(u.email) = lower(m.email) LIMIT 1),
            (SELECT u.id FROM users u WHERE u.username = 'member' || m.id::text LIMIT 1)
        ) AS user_id
    FROM members m;

    UPDATE bills b
    SET paid_by_member_id = map.user_id
    FROM _member_user_map map
    WHERE b.paid_by_member_id = map.old_member_id;

    UPDATE group_members gm
    SET member_id = map.user_id
    FROM _member_user_map map
    WHERE gm.member_id = map.old_member_id;

    UPDATE bill_participants bp
    SET member_id = map.user_id
    FROM _member_user_map map
    WHERE bp.member_id = map.old_member_id;

    UPDATE bill_member_shares bms
    SET member_id = map.user_id
    FROM _member_user_map map
    WHERE bms.member_id = map.old_member_id;

    ALTER TABLE group_members DROP CONSTRAINT IF EXISTS group_members_member_id_fkey;
    ALTER TABLE bills DROP CONSTRAINT IF EXISTS bills_paid_by_member_id_fkey;
    ALTER TABLE bill_participants DROP CONSTRAINT IF EXISTS bill_participants_member_id_fkey;
    ALTER TABLE bill_member_shares DROP CONSTRAINT IF EXISTS bill_member_shares_member_id_fkey;

    DROP TABLE members;

    ALTER TABLE group_members
        ADD CONSTRAINT group_members_member_id_fkey
        FOREIGN KEY (member_id) REFERENCES users(id) ON DELETE CASCADE;
    ALTER TABLE bills
        ADD CONSTRAINT bills_paid_by_member_id_fkey
        FOREIGN KEY (paid_by_member_id) REFERENCES users(id);
    ALTER TABLE bill_participants
        ADD CONSTRAINT bill_participants_member_id_fkey
        FOREIGN KEY (member_id) REFERENCES users(id);
    ALTER TABLE bill_member_shares
        ADD CONSTRAINT bill_member_shares_member_id_fkey
        FOREIGN KEY (member_id) REFERENCES users(id);
END $$;