# API de administración (superadmin)

Gestión de empresas (tenants) y de sus usuarios. Prefijo: `/api/v1/admin`.
**Todas las rutas exigen un access token de un usuario con rol `superadmin`** (de la empresa de sistema `platform`). Cualquier otro rol recibe `403 FORBIDDEN`; sin token, `401 UNAUTHENTICATED`.

Formato de errores, paginación y `code` estables: ver `docs/API_CONVENCIONES.md`.

## Empresas

| Método | Ruta | Qué hace | Respuesta |
|---|---|---|---|
| POST | `/admin/tenants` | Crea empresa con su owner | 201 `{id, slug, name, active, system, user_count, created_at, owner{...}}` |
| GET | `/admin/tenants` | Lista empresas vivas (`q`, `active`, `limit`, `offset`, `include_total`) | 200 página |
| GET | `/admin/tenants/{id}` | Detalle con `user_count` (usuarios vivos) | 200 |
| PATCH | `/admin/tenants/{id}` | Cambia `name` y/o `active` | 200. Desactivar revoca todas sus sesiones |
| DELETE | `/admin/tenants/{id}` | Elimina (soft delete): bloquea sus logins y revoca sus sesiones | 204 |
| POST | `/admin/tenants/{id}/restore` | Restaura una empresa eliminada | 200 detalle |

## Usuarios de una empresa

| Método | Ruta | Qué hace | Respuesta |
|---|---|---|---|
| GET | `/admin/tenants/{id}/users` | Lista usuarios vivos (`q`, `active`, paginación) | 200 página |
| POST | `/admin/tenants/{id}/users` | Crea usuario con roles asignables | 201 |
| PATCH | `/admin/tenants/{id}/users/{userId}` | Cambia nombre, roles, estado o contraseña | 200. Cambiar contraseña o desactivar revoca sus sesiones |
| DELETE | `/admin/tenants/{id}/users/{userId}` | Elimina (soft delete) y revoca sus sesiones | 204 |
| POST | `/admin/tenants/{id}/users/{userId}/restore` | Restaura un usuario eliminado | 200 usuario |

Roles asignables desde la API: `owner`, `admin`, `cobrador`, `viewer`. `superadmin` no se puede asignar.

## Eliminación y restauración

- Eliminar nunca borra filas: se marca `deleted_at` y `deleted_by`. La fila queda en la BD para auditoría.
- Una empresa eliminada no acepta logins ni refresh: el login responde `401 INVALID_CREDENTIALS`, igual que una contraseña incorrecta (no se revela el motivo). Su access token ya emitido vive como máximo 15 min.
- Un usuario eliminado tampoco puede iniciar sesión ni renovar tokens.
- Eliminar un recurso ya eliminado, o restaurar uno que no está eliminado, responde `404 NOT_FOUND`.
- Restaurar a un usuario cuyo correo ya usa otro usuario vivo de la misma empresa responde `409 EMAIL_TAKEN`.
- Restaurar una empresa no reactiva sus sesiones: los usuarios deben iniciar sesión de nuevo.
- Mientras la empresa está eliminada no se pueden crear ni eliminar sus usuarios (`404`).
- El **slug** de una empresa es único global y **no se reutiliza**, ni siquiera tras eliminarla (`409 TENANT_SLUG_TAKEN`). La API no permite cambiar el slug.
- Cada operación queda en `audit_log` con acción `tenant.create|update|delete|restore` o `user.create|update|delete|restore`.

## Protecciones

| Regla | Respuesta |
|---|---|
| La empresa de sistema (`system: true`, la plataforma) no se puede eliminar ni suspender (`active: false`) | `409 PLATFORM_TENANT_PROTECTED` |
| No se pueden crear usuarios en la empresa de sistema | `409 PLATFORM_TENANT_PROTECTED` |
| No se puede eliminar al último superadmin activo de la plataforma | `409 LAST_SUPERADMIN` |
| Un superadmin inactivo se puede eliminar aunque sea el último (no cuenta como activo) | `204` |

Una empresa de sistema se identifica por `system: true` en las respuestas; el frontend debe ocultar sus acciones de eliminar y suspender.

## Códigos de error propios

| HTTP | `code` | Cuándo |
|---|---|---|
| 404 | `NOT_FOUND` | Empresa o usuario inexistente, eliminado, de otra empresa, o id que no es UUID |
| 409 | `TENANT_SLUG_TAKEN` | El slug ya existe (también si la empresa está eliminada) |
| 409 | `EMAIL_TAKEN` | El correo ya lo usa un usuario vivo de la empresa |
| 409 | `PLATFORM_TENANT_PROTECTED` | Operación prohibida sobre la empresa de sistema |
| 409 | `LAST_SUPERADMIN` | Se intenta eliminar al último superadmin activo |

## Empresa de sistema y seed

La empresa `platform` y su superadmin se crean con el seed. Es idempotente y no tiene contraseña por defecto:

```
go run ./cmd/seed
```

Variables obligatorias (del entorno o de `.env`): `DATABASE_URL`, `SEED_ADMIN_EMAIL`, `SEED_ADMIN_PASSWORD` (mínimo 10 caracteres, con letra y número). Si falta alguna, el seed falla y no crea nada. `SEED_ADMIN_NAME` es opcional.

- Si la empresa `platform` ya existe pero no está marcada como de sistema, el seed la marca (`is_system = true`).
- Si el superadmin ya existe, el seed no modifica nada.
- **Un superadmin eliminado no se restaura con el seed**: el seed crea uno nuevo con ese correo. Para restaurar el original, usa `POST .../restore`.
- `go run ./cmd/seed --demo` añade datos de desarrollo y se rechaza con `APP_ENV=production`.

### Arranque automático (AUTO_MIGRATE / AUTO_SEED)

`go run ./cmd/api` puede aplicar las migraciones y el seed antes de abrir el pool de la app y de escuchar, controlado por flags:

| Variable | Def. | Efecto |
|---|---|---|
| `AUTO_MIGRATE` | `false` | Aplica las migraciones pendientes y otorga DML a `APP_DB_USER`. Permitido en production. |
| `AUTO_SEED` | `false` | Asegura el superadmin de plataforma (idempotente). **Rechazado con `APP_ENV=production`**: en producción se ejecuta a mano con `go run ./cmd/seed`. |
| `MIGRATE_DATABASE_URL` | vacío | Sobrescritura opcional solo para desarrollo cuando las migraciones usan un rol dueño distinto. En producción se deja vacío y se usa `DATABASE_URL`. |
| `APP_DB_USER` | `app_user` | Rol que recibe los permisos DML tras migrar. |
| `SEED_ADMIN_EMAIL`, `SEED_ADMIN_PASSWORD` | vacío | Obligatorias si `AUTO_SEED=true` (sin ellas el arranque falla con un error de configuración). |
| `SEED_ADMIN_NAME` | `Platform Admin` | Nombre del superadmin. |

- Orden: migraciones y permisos, después el seed; luego se abre el pool y se escucha. Cualquier fallo aborta el arranque.
- En producción, la aplicación, las migraciones y el seed usan la misma `DATABASE_URL`. En desarrollo, `MIGRATE_DATABASE_URL` puede sustituirla únicamente para migrar con un rol dueño.
- El log indica qué migraciones se aplicaron y si el superadmin se creó o ya existía. Nunca registra contraseñas ni el email del superadmin.
- `Config` no revela secretos en `String()`, `%v`, `%+v` ni `%#v`.

## Ejemplo

```
POST /api/v1/admin/tenants
{ "slug": "acme", "name": "Acme S.A.C.",
  "owner": { "email": "owner@acme.com", "full_name": "Ana Ruiz", "password": "Secreto12345" } }

DELETE /api/v1/admin/tenants/{id}                  -> 204
POST   /api/v1/admin/tenants/{id}/restore          -> 200
```
