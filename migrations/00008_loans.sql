-- +goose Up
ALTER TABLE loans DROP CONSTRAINT loans_interest_type_check;
ALTER TABLE loans ADD CONSTRAINT loans_interest_type_check CHECK (interest_type IN ('simple','flat','french','german'));
ALTER TABLE loans DROP CONSTRAINT loans_frequency_check;
ALTER TABLE loans ADD CONSTRAINT loans_frequency_check CHECK (frequency IN ('daily','weekly','biweekly','monthly','single'));
ALTER TABLE loans ADD COLUMN product_id uuid REFERENCES loan_products(id);
ALTER TABLE loans ADD COLUMN rate numeric(9,4);
ALTER TABLE loans ADD COLUMN rate_period text;
ALTER TABLE loans ADD COLUMN disbursement_date date;
CREATE INDEX loans_tenant_status_idx ON loans (tenant_id, status, id);

-- +goose Down
DROP INDEX loans_tenant_status_idx;
ALTER TABLE loans DROP COLUMN disbursement_date, DROP COLUMN rate_period, DROP COLUMN rate, DROP COLUMN product_id;
ALTER TABLE loans DROP CONSTRAINT loans_frequency_check;
ALTER TABLE loans ADD CONSTRAINT loans_frequency_check CHECK (frequency IN ('weekly','biweekly','monthly'));
ALTER TABLE loans DROP CONSTRAINT loans_interest_type_check;
ALTER TABLE loans ADD CONSTRAINT loans_interest_type_check CHECK (interest_type IN ('simple','french','german'));
