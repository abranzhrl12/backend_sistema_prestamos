-- +goose Up
ALTER TABLE customers
    ADD COLUMN phone2       text,
    ADD COLUMN whatsapp     text,
    ADD COLUMN email        text,
    ADD COLUMN occupation   text,
    ADD COLUMN address_reference text,
    ADD COLUMN notes       text,
    ADD COLUMN status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive','blocked')),
    ADD COLUMN assigned_collector_id uuid REFERENCES users(id);

CREATE INDEX customers_tenant_name_idx ON customers (tenant_id, lower(full_name));
CREATE INDEX customers_tenant_status_idx ON customers (tenant_id, status);

-- +goose Down
DROP INDEX customers_tenant_status_idx, customers_tenant_name_idx;
ALTER TABLE customers
    DROP COLUMN phone2, DROP COLUMN whatsapp, DROP COLUMN email, DROP COLUMN occupation, DROP COLUMN address_reference,
    DROP COLUMN notes, DROP COLUMN status, DROP COLUMN assigned_collector_id;
