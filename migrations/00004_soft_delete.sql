-- +goose Up
-- Soft delete (CLAUDE.md, "Eliminación: SOLO soft delete"): nunca borrado físico de tablas de negocio.
-- tenants.is_system marca la empresa de la plataforma: no se puede eliminar ni suspender.
-- Las columnas nuevas heredan los GRANT de tabla de migrate.GrantApp: no hace falta otorgar nada.

ALTER TABLE tenants
    ADD COLUMN is_system  boolean     NOT NULL DEFAULT false,
    ADD COLUMN deleted_at timestamptz NULL,
    ADD COLUMN deleted_by uuid        NULL;

ALTER TABLE users
    ADD COLUMN deleted_at timestamptz NULL,
    ADD COLUMN deleted_by uuid        NULL;

ALTER TABLE customers
    ADD COLUMN deleted_at timestamptz NULL,
    ADD COLUMN deleted_by uuid        NULL;

-- Unicidad solo entre filas vivas: un correo o documento se puede reutilizar tras eliminarlo.
-- El slug de tenants NO cambia: sigue siendo UNIQUE global, incluso si la empresa está eliminada.
DROP INDEX users_tenant_email_uq;
CREATE UNIQUE INDEX users_tenant_email_uq ON users (tenant_id, lower(email)) WHERE deleted_at IS NULL;

ALTER TABLE customers DROP CONSTRAINT customers_tenant_id_document_id_key;
CREATE UNIQUE INDEX customers_tenant_document_uq ON customers (tenant_id, document_id) WHERE deleted_at IS NULL;

-- Índices de listado parciales. Las consultas deben repetir "deleted_at IS NULL" para usarlos.
DROP INDEX customers_tenant_name_id_idx;
CREATE INDEX customers_tenant_name_id_idx ON customers (tenant_id, lower(full_name), id) WHERE deleted_at IS NULL;

DROP INDEX customers_tenant_status_idx;
CREATE INDEX customers_tenant_status_idx ON customers (tenant_id, status) WHERE deleted_at IS NULL;

CREATE INDEX users_tenant_id_idx ON users (tenant_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX tenants_live_id_idx ON tenants (id DESC) WHERE deleted_at IS NULL;

-- +goose Down
-- Puede fallar si hay filas eliminadas que duplican documento o correo: esas deben resolverse antes.
DROP INDEX tenants_live_id_idx;
DROP INDEX users_tenant_id_idx;

DROP INDEX customers_tenant_status_idx;
CREATE INDEX customers_tenant_status_idx ON customers (tenant_id, status);

DROP INDEX customers_tenant_name_id_idx;
CREATE INDEX customers_tenant_name_id_idx ON customers (tenant_id, lower(full_name), id);

DROP INDEX customers_tenant_document_uq;
ALTER TABLE customers ADD CONSTRAINT customers_tenant_id_document_id_key UNIQUE (tenant_id, document_id);

DROP INDEX users_tenant_email_uq;
CREATE UNIQUE INDEX users_tenant_email_uq ON users (tenant_id, lower(email));

ALTER TABLE customers DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE users DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE tenants DROP COLUMN deleted_by, DROP COLUMN deleted_at, DROP COLUMN is_system;
