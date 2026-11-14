-- +goose Up
-- Listado ordenado por nombre: el índice cubre filtro por tenant + ORDER BY + desempate (reemplaza al de 00002).
-- NOTA: no se crean índices pg_trgm para ILIKE. Medido: con RLS, Postgres NO puede usar un índice para
-- operadores no "leakproof" (textlike/texticlike), así que esos índices jamás se usarían y solo frenarían
-- las escrituras. La búsqueda con ILIKE recorre solo las filas del tenant (ver docs/API_CONVENCIONES.md).
DROP INDEX customers_tenant_name_idx;
CREATE INDEX customers_tenant_name_id_idx ON customers (tenant_id, lower(full_name), id);

-- +goose Down
DROP INDEX customers_tenant_name_id_idx;
CREATE INDEX customers_tenant_name_idx ON customers (tenant_id, lower(full_name));
