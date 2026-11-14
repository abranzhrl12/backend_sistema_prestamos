# Convenciones de la API

Contrato estable para el frontend. Prefijo: `/api/v1`. Todo en JSON UTF-8.

## 1. Errores

**Un solo formato** para todos los errores (incluidos 404 de ruta, 413, 500):

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Hay datos inválidos en la solicitud",
    "fields": [
      { "field": "full_name", "message": "El nombre es obligatorio" },
      { "field": "document_id", "message": "El documento es obligatorio" }
    ],
    "request_id": "6f1c2d9e-..."
  }
}
```

- `code`: **estable**, en inglés `UPPER_SNAKE`. El frontend decide la lógica con `code`, nunca con `message`.
- `message`: en **español**, listo para mostrar al usuario.
- `fields`: solo en `VALIDATION_ERROR`; `field` coincide con el nombre del campo JSON. Se devuelven **todos** los campos con error de una vez.
- `request_id`: también viaja en el header `X-Request-Id`; úsalo al reportar un problema.
- Los errores 5xx **nunca** exponen detalles internos (solo van al log, con el mismo `request_id`).

| HTTP | `code` | Cuándo | Acción sugerida en el frontend |
|---|---|---|---|
| 400 | `VALIDATION_ERROR` | Datos inválidos (ver `fields`), paginación fuera de rango | Marcar los campos |
| 400 | `INVALID_JSON` | Cuerpo no es JSON válido | Error genérico |
| 401 | `UNAUTHENTICATED` | Falta token/cookie | Ir a login |
| 401 | `TOKEN_INVALID` | Token inválido o expirado (Bearer; en cookies el backend renueva solo) | Bearer: intentar `/auth/refresh`; si falla, login |
| 401 | `INVALID_CREDENTIALS` | Login incorrecto (no revela si falló usuario o empresa) | Mostrar mensaje |
| 401 | `SESSION_EXPIRED` | Refresh token inválido/usado/expirado | Ir a login |
| 403 | `CSRF_INVALID` | Cookie sin header `X-CSRF-Token` en POST/PUT/PATCH/DELETE | Reenviar leyendo la cookie `csrf_token` |
| 403 | `FORBIDDEN` | Rol sin permiso | Ocultar acción / mensaje |
| 404 | `NOT_FOUND` | Recurso o ruta inexistente (también si es de otro tenant) | Mensaje |
| 405 | `METHOD_NOT_ALLOWED` | Método incorrecto | — |
| 409 | `DUPLICATE_DOCUMENT` (y otros específicos: `TENANT_SLUG_TAKEN`, `EMAIL_TAKEN`, `PLATFORM_TENANT_PROTECTED`, `LAST_SUPERADMIN`) | Conflicto de unicidad o regla de negocio | Mostrar mensaje |
| 413 | `PAYLOAD_TOO_LARGE` | Cuerpo > 1 MiB | — |
| 429 | `TOO_MANY_REQUESTS` | Límite de tasa | Reintentar luego |
| 500 | `INTERNAL_ERROR` | Error no controlado | Mensaje genérico + mostrar `request_id` |

Códigos nuevos por módulo se **agregan** a esta tabla; un `code` publicado no se renombra.

### Cómo se implementa (backend)
- Los handlers devuelven `*httpx.AppError` (constructores en `internal/platform/httpx/apperror.go`); `httpx.ErrorHandler` los serializa.
- El **dominio** devuelve errores propios (`ErrNotFound`, `ValidationError`...) sin saber de HTTP; el **adaptador** los traduce en `mapErr`.
- Mensajes en español viven en el adaptador HTTP, no en el dominio.

## 2. Paginación

Todos los listados usan la misma envoltura:

```json
{
  "items": [ ... ],
  "meta": { "limit": 20, "offset": 40, "has_more": true }
}
```

- Parámetros: `?limit=` (1–100, por defecto 20) y `?offset=` (≥ 0). Fuera de rango → `400 VALIDATION_ERROR` indicando el campo.
- **`total` NO viene por defecto.** Contar toda la tabla en cada página es lo más caro de un listado (medido: sin contar, 124 → 946 req/s). Si el frontend necesita el total (p. ej. "1 250 resultados"), lo pide **una vez** con `?include_total=true` — al abrir la pantalla o al cambiar filtros — y reutiliza el valor al paginar:
  `"meta": { "total": 1250, "limit": 20, "offset": 0, "has_more": true }`
- `has_more` siempre está presente; se calcula pidiendo `limit+1` filas (sin `COUNT`).
- `items` es **siempre un arreglo** (`[]`, nunca `null`).
- Filtros y búsqueda por query string (`q`, `status`, ...). El orden es estable y determinista (con desempate por `id`, calificado como `tabla.id` para que use el índice).
- **Listados grandes de solo-agregado** (pagos, movimientos de capital, auditoría): usar **paginación por cursor** (keyset: `?after=<id>&limit=`, orden por UUIDv7) porque `OFFSET` se degrada con páginas profundas (medido: p95 ≈ 320 ms en offset aleatorio vs ≈ 40 ms en las primeras páginas). Respuesta: `meta: { limit, next_cursor, has_more }`. Se aplica cuando se implementen esos módulos.

## 3. Rendimiento (qué se definió y por qué)

Todo se mide con `tests/perf` (20 000 clientes, 32 usuarios concurrentes, Postgres real con RLS). Resultados de referencia en un equipo de desarrollo (Docker en la misma máquina):

| Escenario | req/s | p50 | p95 |
|---|---|---|---|
| Listado (páginas 1–10) | ~1 150 | 25 ms | 42 ms |
| Mixto 80 % lectura / 20 % escritura | ~1 150 | 24 ms | 50 ms |
| Get por id | ~1 800 | 17 ms | 28 ms |
| Crear cliente | ~760 | 38 ms | 62 ms |
| Búsqueda (8 usuarios concurrentes) | ~85 | 85 ms | 125–180 ms |
| Login (Argon2id, 4 usuarios) | ~13 | 290 ms | 420 ms |

**Presupuesto: p95 < 250 ms, 0 errores** (el test falla si se supera).

| Área | Decisión |
|---|---|
| JSON | `bytedance/sonic` (medido: ~1.6× más rápido al serializar y ~2.8× al leer que `encoding/json`) |
| Pool BD | `pgxpool`: `MaxConns = max(8, CPUs×4)`, `MinConns=2`, vida 1 h, inactividad 15 min, health-check 1 min. Configurable: `DB_MAX_CONNS` / `DB_MIN_CONNS` |
| Consultas | Sin ORM, SQL explícito, columnas listadas (nunca `SELECT *`), una consulta por listado, filtros solo si se piden |
| Ordenamiento | `ORDER BY lower(full_name), customers.id` + índice `(tenant_id, lower(full_name), id)`. **Calificar `tabla.id`**: si la consulta devuelve `id::text`, un `ORDER BY id` ordena por el texto y descarta el índice |
| Total | Opcional (`include_total`), y sin `COUNT` cuando la página incompleta ya lo determina |
| Índices | `(tenant_id, ...)` primero; PK UUIDv7 (inserciones ordenadas) |
| HTTP | Cuerpo máx. 1 MiB; timeouts lectura/escritura 10 s, idle 60 s |
| Auth | Validar JWT ≈ 12 µs; Argon2id (≈150 ms) **solo en login** |

### Búsqueda (limitación conocida)
- `ILIKE '%texto%'` **no usa índices bajo RLS**: Postgres solo permite índices con operadores *leakproof* cuando hay política de seguridad, y `textlike`/`texticlike` no lo son (verificado con `pg_proc`). Por eso **no se crean índices `pg_trgm`** (no se usarían y frenarían las escrituras).
- Costo: recorre las filas **del tenant**, no de toda la tabla. Aceptable hasta ~50 000 clientes por empresa; es una búsqueda de uso humano (baja concurrencia).
- Se reduce el costo eligiendo columnas según lo escrito: solo dígitos → documento/teléfono; si no → nombre/documento.
- Si una empresa supera ese tamaño: búsqueda por **prefijo** con rangos sobre índice btree (operadores leakproof), o motor de búsqueda dedicado. No optimizar antes de que haga falta.

Regla: **no optimizar sin medir**. Toda optimización se justifica con `tests/perf`, `EXPLAIN (ANALYZE)` o un benchmark, antes y después.

## 4. Eliminación (soft delete)

- `DELETE /recurso/:id` responde **204** y marca la fila (`deleted_at`, `deleted_by`); nunca borra la fila. Los registros financieros son append-only y se anulan con un movimiento nuevo.
- Un recurso eliminado es **invisible**: `GET`, listados, `PUT`/`PATCH` y `DELETE` responden **404 `NOT_FOUND`**, igual que un recurso de otra empresa.
- Se puede volver a crear algo con el mismo documento o correo que uno eliminado. El `slug` de una empresa **no** se reutiliza nunca.
- Restaurar (`POST .../restore`) es solo para superadmin; si el correo ya lo usa otro usuario vivo, `409 EMAIL_TAKEN`.
- Detalle de empresas y usuarios: `docs/ADMIN_API.md`.

## 5. Seguridad de la API (resumen)
Ver `CLAUDE.md`: cookies HttpOnly + CSRF (doble envío) para web, Bearer JWT para móvil, RLS por tenant, rotación de refresh tokens.

## 6. Autenticación web (cookies) y renovación de sesión

Aplica al transporte **cookie** (`"transport": "cookie"` en el login). El móvil/servicios (Bearer) no cambia.

**Cookies emitidas por login y `POST /auth/refresh`:**

| Cookie | Path | Flags | Contenido |
|---|---|---|---|
| `access_token` | `/` | HttpOnly | JWT de vida corta (`ACCESS_TTL`) |
| `refresh_token` | `/api/v1` | HttpOnly | Refresh opaco; viaja en toda la API |
| `csrf_token` | `/` | legible por JS | Valor que hay que reenviar en `X-CSRF-Token` |

**Renovación transparente (lo que debe saber el frontend):**
- El frontend solo usa `credentials: 'include'`. **No llama a `/auth/refresh`** en web.
- Si el `access_token` falta o venció y la petición trae `refresh_token`, el backend rota la sesión en esa misma petición. La respuesta trae `Set-Cookie` con `access_token`, `refresh_token` y `csrf_token` nuevos; la petición se procesa normalmente.
- En mutaciones (POST/PUT/PATCH/DELETE) se valida `X-CSRF-Token` **antes** de renovar. Sin CSRF válido → `403 CSRF_INVALID` y no se renueva.
- Si la sesión no se puede renovar (refresh inválido, ya usado, expirado, usuario o empresa eliminados o suspendidos) → `401 SESSION_EXPIRED` y cookies limpiadas: ir a login.
- Un `access_token` manipulado (firma inválida) responde `401 TOKEN_INVALID` y **no** dispara renovación.
- Con Bearer, un access vencido responde `401 TOKEN_INVALID` y no renueva: el cliente móvil llama a `/auth/refresh` con su `refresh_token`.

**Concurrencia:** el refresh rota el token, así que varias peticiones simultáneas con la misma cookie vieja reciben la misma rotación (se reutiliza el resultado durante 10 s). Después de esa ventana, la cookie vieja se rechaza.

**Límite conocido:** esta deduplicación vive en memoria de **una sola instancia**. Con varias réplicas, dos peticiones paralelas pueden caer en instancias distintas y una de ellas cerraría la sesión. Antes de escalar horizontalmente hay que usar sticky sessions por cookie o una ventana de gracia en la BD.

**Logout y cookies antiguas:** `POST /auth/logout` limpia `refresh_token` en `/api/v1` y en el path antiguo `/api/v1/auth` (cookies emitidas antes de este cambio). Login y refresh también expiran la del path antiguo, así que no quedan dos cookies de refresh en el navegador.
