-- +goose Up
CREATE TABLE loan_extensions (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL REFERENCES tenants(id),
 loan_id uuid NOT NULL REFERENCES loans(id),
 created_by uuid NOT NULL REFERENCES users(id),
 previous_schedule jsonb NOT NULL,
 first_payment_date date NOT NULL,
 additional_interest numeric(19,4) NOT NULL CHECK (additional_interest >= 0),
 note text NOT NULL,
 idempotency_key text NOT NULL,
 request_hash text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,idempotency_key)
);
CREATE INDEX loan_extensions_loan_idx ON loan_extensions(tenant_id,loan_id,id);
ALTER TABLE loan_extensions ENABLE ROW LEVEL SECURITY;
ALTER TABLE loan_extensions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON loan_extensions
 USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
 WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- +goose Down
DROP TABLE loan_extensions;
