-- +goose Up

-- 1. Пользователи: временные баны
ALTER TABLE users
    ADD COLUMN ban_expires_at TIMESTAMPTZ;

-- 2. События: скрытие без отмены
ALTER TABLE events
    ADD COLUMN hidden BOOLEAN NOT NULL DEFAULT false;

-- 3. Жалобы: резолюция
ALTER TABLE complaints
    ADD COLUMN resolution_comment TEXT,
    ADD COLUMN resolved_at TIMESTAMPTZ,
    ADD COLUMN resolved_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- 4. Тикеты: резолюция + связь с жалобой + ответы
ALTER TABLE support_tickets
    ADD COLUMN complaint_id UUID REFERENCES complaints(id) ON DELETE SET NULL,
    ADD COLUMN resolution_comment TEXT,
    ADD COLUMN resolved_at TIMESTAMPTZ,
    ADD COLUMN resolved_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- Расширяем статус tickets: добавляем rejected
ALTER TABLE support_tickets DROP CONSTRAINT IF EXISTS support_tickets_status_check;
ALTER TABLE support_tickets
    ADD CONSTRAINT support_tickets_status_check
    CHECK (status IN ('open', 'resolved', 'rejected'));

-- 5. История ответов на тикеты
CREATE TABLE ticket_replies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticket_id UUID NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    admin_id UUID REFERENCES users(id) ON DELETE SET NULL,
    admin_email TEXT NOT NULL,       -- снимок на момент ответа
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ticket_replies_ticket ON ticket_replies(ticket_id, created_at);

-- 6. Аудит действий администраторов
CREATE TABLE admin_audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    admin_id UUID REFERENCES users(id) ON DELETE SET NULL,
    admin_email TEXT NOT NULL,       -- снимок
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id UUID,
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_admin_created ON admin_audit_log(admin_id, created_at DESC);
CREATE INDEX idx_audit_log_target ON admin_audit_log(target_type, target_id);
CREATE INDEX idx_audit_log_created ON admin_audit_log(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS admin_audit_log;
DROP TABLE IF EXISTS ticket_replies;
ALTER TABLE support_tickets DROP CONSTRAINT IF EXISTS support_tickets_status_check;
ALTER TABLE support_tickets
    ADD CONSTRAINT support_tickets_status_check
    CHECK (status IN ('open', 'resolved'));
ALTER TABLE support_tickets
    DROP COLUMN IF EXISTS complaint_id,
    DROP COLUMN IF EXISTS resolution_comment,
    DROP COLUMN IF EXISTS resolved_at,
    DROP COLUMN IF EXISTS resolved_by;
ALTER TABLE complaints
    DROP COLUMN IF EXISTS resolution_comment,
    DROP COLUMN IF EXISTS resolved_at,
    DROP COLUMN IF EXISTS resolved_by;
ALTER TABLE events DROP COLUMN IF EXISTS hidden;
ALTER TABLE users DROP COLUMN IF EXISTS ban_expires_at;