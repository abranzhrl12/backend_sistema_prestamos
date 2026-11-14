-- +goose Up
-- IDs: UUIDv7 generados en la app (google/uuid NewV7) o uuidv7() nativo en PG18.
-- IMPORTANTE: la app debe conectarse con un rol NO superusuario y NO dueño de las tablas;
-- si no, RLS se omite. FORCE ROW LEVEL SECURITY cubre al dueño, no a superusuarios.

CREATE TABLE tenants (
    id         uuid PRIMARY KEY,
    slug       text NOT NULL UNIQUE,
    name       text NOT NULL,
    active     boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id            uuid PRIMARY KEY,
    tenant_id     uuid NOT NULL REFERENCES tenants(id),
    email         text NOT NULL,
    full_name     text NOT NULL,
    password_hash text NOT NULL,
    roles         text[] NOT NULL DEFAULT '{}',
    active        boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_tenant_email_uq ON users (tenant_id, lower(email));

CREATE TABLE refresh_tokens (
    token_hash text PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants(id),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);

CREATE TABLE customers (
    id          uuid PRIMARY KEY,
    tenant_id   uuid NOT NULL REFERENCES tenants(id),
    full_name   text NOT NULL,
    document_id text NOT NULL,
    phone       text,
    address     text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, document_id)
);

CREATE TABLE loans (
    id            uuid PRIMARY KEY,
    tenant_id     uuid NOT NULL REFERENCES tenants(id),
    customer_id   uuid NOT NULL REFERENCES customers(id),
    created_by    uuid NOT NULL REFERENCES users(id),
    principal     numeric(19,4) NOT NULL CHECK (principal > 0),
    annual_rate   numeric(9,6)  NOT NULL CHECK (annual_rate >= 0),
    interest_type text NOT NULL CHECK (interest_type IN ('simple','french','german')),
    term_count    int NOT NULL CHECK (term_count > 0),
    frequency     text NOT NULL CHECK (frequency IN ('weekly','biweekly','monthly')),
    status        text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','paid','defaulted','cancelled')),
    disbursed_at  timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX loans_tenant_customer_idx ON loans (tenant_id, customer_id);

CREATE TABLE loan_schedule (
    id               uuid PRIMARY KEY,
    tenant_id        uuid NOT NULL REFERENCES tenants(id),
    loan_id          uuid NOT NULL REFERENCES loans(id) ON DELETE CASCADE,
    installment_no   int NOT NULL,
    due_date         date NOT NULL,
    principal_due    numeric(19,4) NOT NULL,
    interest_due     numeric(19,4) NOT NULL,
    principal_paid   numeric(19,4) NOT NULL DEFAULT 0,
    interest_paid    numeric(19,4) NOT NULL DEFAULT 0,
    late_fee_paid    numeric(19,4) NOT NULL DEFAULT 0,
    UNIQUE (loan_id, installment_no)
);
CREATE INDEX loan_schedule_due_idx ON loan_schedule (tenant_id, due_date);

CREATE TABLE payments (
    id          uuid PRIMARY KEY,
    tenant_id   uuid NOT NULL REFERENCES tenants(id),
    loan_id     uuid NOT NULL REFERENCES loans(id),
    received_by uuid NOT NULL REFERENCES users(id),
    amount      numeric(19,4) NOT NULL CHECK (amount > 0),
    method      text NOT NULL,
    paid_at     timestamptz NOT NULL DEFAULT now(),
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payments_loan_idx ON payments (tenant_id, loan_id);

CREATE TABLE payment_allocations (
    id          uuid PRIMARY KEY,
    tenant_id   uuid NOT NULL REFERENCES tenants(id),
    payment_id  uuid NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    schedule_id uuid NOT NULL REFERENCES loan_schedule(id),
    principal   numeric(19,4) NOT NULL DEFAULT 0,
    interest    numeric(19,4) NOT NULL DEFAULT 0,
    late_fee    numeric(19,4) NOT NULL DEFAULT 0
);

CREATE TABLE audit_log (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants(id),
    user_id    uuid,
    action     text NOT NULL,
    entity     text NOT NULL,
    entity_id  uuid,
    data       jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Row Level Security: aislamiento por tenant en todas las tablas con tenant_id.
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['users','refresh_tokens','customers','loans','loan_schedule',
                             'payments','payment_allocations','audit_log']
    LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format($p$CREATE POLICY tenant_isolation ON %I
            USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
            WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)$p$, t);
    END LOOP;
END $$;

-- +goose Down
DROP TABLE audit_log, payment_allocations, payments, loan_schedule, loans, customers, refresh_tokens, users, tenants;
