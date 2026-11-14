-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION loan_extensions_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'loan_extensions es append-only';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER loan_extensions_no_update_delete BEFORE UPDATE OR DELETE ON loan_extensions
 FOR EACH ROW EXECUTE FUNCTION loan_extensions_append_only();
CREATE TRIGGER loan_extensions_no_truncate BEFORE TRUNCATE ON loan_extensions
 FOR EACH STATEMENT EXECUTE FUNCTION loan_extensions_append_only();

-- +goose Down
DROP TRIGGER loan_extensions_no_update_delete ON loan_extensions;
DROP TRIGGER loan_extensions_no_truncate ON loan_extensions;
DROP FUNCTION loan_extensions_append_only();
