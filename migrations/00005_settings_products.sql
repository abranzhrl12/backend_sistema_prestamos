-- +goose Up
-- Configuración de la empresa (una fila por tenant) y productos de préstamo (soft delete).
-- Dinero y porcentajes en NUMERIC; nunca float.

CREATE TABLE tenant_settings (
    tenant_id                  uuid PRIMARY KEY REFERENCES tenants(id),
    currency                   text NOT NULL DEFAULT 'PEN' CHECK (currency IN ('PEN','USD')),
    timezone                   text NOT NULL DEFAULT 'America/Lima',
    late_fee_mode              text NOT NULL DEFAULT 'fixed' CHECK (late_fee_mode IN ('fixed','proportional')),
    late_fee_amount            numeric(19,4) NOT NULL DEFAULT 10 CHECK (late_fee_amount >= 0),
    late_fee_per_day           boolean NOT NULL DEFAULT false,
    grace_days                 int NOT NULL DEFAULT 0 CHECK (grace_days BETWEEN 0 AND 30),
    allocation_order           text NOT NULL DEFAULT 'late_fee_interest_principal'
                               CHECK (allocation_order IN ('late_fee_interest_principal','interest_principal_late_fee')),
    max_active_loans_per_customer int NOT NULL DEFAULT 3 CHECK (max_active_loans_per_customer BETWEEN 1 AND 50),
    allow_negative_capital     boolean NOT NULL DEFAULT false,
    updated_by                 uuid,
    updated_at                 timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE loan_products (
    id              uuid PRIMARY KEY,
    tenant_id       uuid NOT NULL REFERENCES tenants(id),
    name            text NOT NULL,
    frequency       text NOT NULL CHECK (frequency IN ('daily','weekly','biweekly','monthly','single')),
    interest_type   text NOT NULL DEFAULT 'flat' CHECK (interest_type IN ('flat','french','german')),
    rate_period     text NOT NULL CHECK (rate_period IN ('daily','weekly','biweekly','monthly','annual')),
    rate_default    numeric(9,4) NOT NULL,
    rate_min        numeric(9,4) NOT NULL,
    rate_max        numeric(9,4) NOT NULL,
    amount_min      numeric(19,4) NOT NULL,
    amount_max      numeric(19,4) NOT NULL,
    terms_min       int NOT NULL,
    terms_max       int NOT NULL,
    -- Overrides: NULL = usar la configuración de la empresa (tenant_settings).
    grace_days      int NULL CHECK (grace_days BETWEEN 0 AND 30),
    late_fee_mode   text NULL CHECK (late_fee_mode IN ('fixed','proportional')),
    late_fee_amount numeric(19,4) NULL CHECK (late_fee_amount >= 0),
    late_fee_per_day boolean NULL,
    fee_type        text NOT NULL DEFAULT 'none' CHECK (fee_type IN ('none','fixed','percent')),
    fee_value       numeric(19,4) NOT NULL DEFAULT 0 CHECK (fee_value >= 0),
    active          boolean NOT NULL DEFAULT true,
    created_by      uuid,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz NULL,
    deleted_by      uuid NULL,
    CONSTRAINT loan_products_rate_chk CHECK (0 <= rate_min AND rate_min <= rate_default
                                             AND rate_default <= rate_max AND rate_max <= 1000),
    CONSTRAINT loan_products_amount_chk CHECK (amount_min > 0 AND amount_min <= amount_max),
    CONSTRAINT loan_products_terms_chk CHECK (terms_min >= 1 AND terms_min <= terms_max AND terms_max <= 1000)
);
-- Unicidad de nombre entre productos vivos (se puede reutilizar tras eliminar).
CREATE UNIQUE INDEX loan_products_tenant_name_uq ON loan_products (tenant_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX loan_products_tenant_name_id_idx ON loan_products (tenant_id, lower(name), id) WHERE deleted_at IS NULL;

-- Row Level Security: mismo patrón que 00001_init.sql.
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['tenant_settings','loan_products']
    LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format($p$CREATE POLICY tenant_isolation ON %I
            USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
            WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)$p$, t);
    END LOOP;
END $$;

-- +goose Down
DROP TABLE loan_products;
DROP TABLE tenant_settings;
