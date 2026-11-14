-- +goose Up
ALTER TABLE loan_schedule ADD COLUMN late_fee_accrued numeric(19,4) NOT NULL DEFAULT 0 CHECK (late_fee_accrued >= 0);
ALTER TABLE payments ADD COLUMN idempotency_key text;
ALTER TABLE payments ADD COLUMN request_hash text;
ALTER TABLE payments ADD COLUMN status text NOT NULL DEFAULT 'valid' CHECK (status IN ('valid','voided'));
CREATE UNIQUE INDEX payments_tenant_idempotency_uq ON payments (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE payment_parts (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    payment_id uuid NOT NULL REFERENCES payments(id),
    method text NOT NULL,
    amount numeric(19,4) NOT NULL CHECK (amount > 0),
    reference text,
    bank text,
    phone text
);
CREATE UNIQUE INDEX payment_parts_reference_uq ON payment_parts (tenant_id, method, reference) WHERE reference IS NOT NULL;
CREATE INDEX payment_parts_payment_idx ON payment_parts (tenant_id, payment_id);
ALTER TABLE payment_parts ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment_parts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON payment_parts
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- +goose Down
DROP TABLE payment_parts;
DROP INDEX payments_tenant_idempotency_uq;
ALTER TABLE payments DROP COLUMN status, DROP COLUMN request_hash, DROP COLUMN idempotency_key;
ALTER TABLE loan_schedule DROP COLUMN late_fee_accrued;
