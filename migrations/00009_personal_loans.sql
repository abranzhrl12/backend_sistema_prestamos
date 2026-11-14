-- +goose Up
-- Existing product-based loans retain their original fields and schedule.
ALTER TABLE loans ADD COLUMN interest_value numeric(19,4);
ALTER TABLE loans ADD COLUMN term_value int;
ALTER TABLE loans ADD COLUMN term_unit text;
ALTER TABLE loans DROP CONSTRAINT loans_interest_type_check;
ALTER TABLE loans ADD CONSTRAINT loans_interest_type_check
 CHECK (interest_type IN ('simple','flat','french','german','none','fixed','percentage'));
ALTER TABLE loans ADD CONSTRAINT loans_personal_term_check
 CHECK ((term_value IS NULL AND term_unit IS NULL) OR
        (term_value BETWEEN 1 AND 1000 AND term_unit IN ('days','weeks','months')));

-- +goose Down
-- Personal loan rows cannot be represented by the old constraint.
ALTER TABLE loans DROP CONSTRAINT loans_personal_term_check;
ALTER TABLE loans DROP CONSTRAINT loans_interest_type_check;
ALTER TABLE loans ADD CONSTRAINT loans_interest_type_check CHECK (interest_type IN ('simple','flat','french','german')) NOT VALID;
ALTER TABLE loans DROP COLUMN term_unit, DROP COLUMN term_value, DROP COLUMN interest_value;
