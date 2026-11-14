-- +goose Up
-- Caja única por empresa: libro de movimientos append-only. El saldo se obtiene sumando filas.
-- Los errores se corrigen con un movimiento adjustment_*; nunca se edita ni se borra una fila.
-- amount es siempre positivo: la dirección (entra/sale) la define el tipo (ver internal/capital/domain).
CREATE TABLE capital_movements (
    id          uuid PRIMARY KEY,
    tenant_id   uuid NOT NULL REFERENCES tenants(id),
    type        text NOT NULL CHECK (type IN ('contribution','withdrawal','adjustment_in','adjustment_out',
                                              'disbursement','principal_collection','interest_collection',
                                              'late_fee_collection','expense')),
    amount      numeric(19,4) NOT NULL CHECK (amount > 0),
    note        text NULL,
    reference   text NULL,
    loan_id     uuid NULL REFERENCES loans(id),
    payment_id  uuid NULL REFERENCES payments(id),
    created_by  uuid NULL REFERENCES users(id),
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX capital_movements_tenant_id_idx ON capital_movements (tenant_id, id DESC);
CREATE INDEX capital_movements_tenant_type_idx ON capital_movements (tenant_id, type, id DESC);

-- Inmutabilidad en BD (defensa en profundidad): ni la app ni un superusuario descuidado pueden
-- modificar ni vaciar el libro. Los triggers se disparan también para superusuarios.
CREATE FUNCTION capital_movements_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'capital_movements es append-only';
END;
$$;

CREATE TRIGGER capital_movements_no_update_delete
    BEFORE UPDATE OR DELETE ON capital_movements
    FOR EACH ROW EXECUTE FUNCTION capital_movements_append_only();

CREATE TRIGGER capital_movements_no_truncate
    BEFORE TRUNCATE ON capital_movements
    FOR EACH STATEMENT EXECUTE FUNCTION capital_movements_append_only();

-- Row Level Security: mismo estilo que 00001_init.sql.
ALTER TABLE capital_movements ENABLE ROW LEVEL SECURITY;
ALTER TABLE capital_movements FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON capital_movements
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- +goose Down
DROP TABLE capital_movements;
DROP FUNCTION capital_movements_append_only();
