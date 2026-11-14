# Préstamos — API multitenant (Go)

## Prioridad funcional

El alcance vigente está en `docs/PRESTAMOS_PERSONALES.md`: préstamo directo sin producto obligatorio, interés opcional TOTAL, número de pagos, frecuencia y fecha del primer pago. El plazo se deriva; no pedir una duración adicional. Conservar el historial y la compatibilidad; no reintroducir planes obligatorios, rangos de tasas ni cálculos financieros técnicos en el flujo principal.

## Stack
Go · Fiber v3 · pgx v5 (+ sqlc para queries) · PostgreSQL · UUIDv7 · sonic (JSON) · swaggo/swag (OpenAPI) · goose (migraciones) · slog.

## Arquitectura: Hexagonal + DDD táctico, monolito modular
```
authkit/                 # SDK de auth (módulo Go separado, sin Fiber ni BD)
cmd/api/                 # composition root: aquí se cablea TODO (DI manual)
internal/
  platform/              # config, db (WithTenant), httpauth (middleware/cookies)
  auth/                  # adaptadores de authkit: Postgres + handlers Fiber
  <modulo>/              # loans, customers, payments...
    domain/              # entidades, value objects, errores, puertos (interfaces)
    app/                 # casos de uso
    adapters/http|postgres/
migrations/              # goose, SQL puro
```
Dependencias: `adapters → app → domain`. `domain` NO importa fiber, pgx, sonic ni nada externo (salvo decimal/uuid).
Los módulos no se importan entre sí por tipos internos; se comunican por interfaces pequeñas o eventos.

## Multitenancy
- Una BD, `tenant_id` en toda tabla de negocio + **RLS** (`FORCE ROW LEVEL SECURITY`).
- Todo acceso a datos de negocio pasa por `db.WithTenant(ctx, pool, tenantID, fn)`; el `tenantID` sale SIEMPRE de los claims del token, jamás del body/query.
- La app se conecta con un rol sin superusuario ni dueño de tablas (si no, RLS se omite).
- `tenants` no tiene RLS. El login web resuelve la empresa desde el correo bajo contextos RLS; rechaza correos ambiguos. Ver `docs/ADMINISTRACION_ACCESOS.md`.

## Auth (authkit)
- Híbrido: **Bearer JWT** (móvil/servicios) o **cookies HttpOnly** (web). Se elige con `transport` en el login.
- Access JWT HS256 de vida corta (15 min). Refresh opaco, rotativo, guardado como SHA-256, formato `<tenantID>.<secreto>`.
- Cookies: `access_token` (HttpOnly), `refresh_token` (HttpOnly, path `/api/v1`), `csrf_token` (legible). Cookie ⇒ en POST/PUT/PATCH/DELETE exigir header `X-CSRF-Token` igual a la cookie. Bearer no requiere CSRF.
- Web (cookies): **renovación transparente en el backend**. Si el `access_token` falta o venció (`authkit.ErrTokenExpired`) y hay `refresh_token`, el middleware (`httpauth.Authenticate`) rota la sesión en la misma petición y responde con las 3 cookies nuevas. El frontend solo usa `credentials: 'include'` y nunca llama a `/auth/refresh`. La validación CSRF de mutaciones va ANTES de renovar. Si la rotación falla → 401 `SESSION_EXPIRED` y cookies limpiadas.
- Concurrencia: la rotación consume el refresh; `httpauth.Renewer` deduplica peticiones con la misma cookie vieja (singleflight por hash) y reutiliza el resultado 10 s. **Límite:** en memoria de una sola instancia; con varias réplicas hacen falta sticky sessions o ventana de gracia en BD.
- Bearer (móvil/servicios): nunca renueva; un access vencido responde 401 `TOKEN_INVALID` y el cliente llama a `/auth/refresh`.
- Path del refresh: `/api/v1`. Login/refresh expiran la cookie del path legado `/api/v1/auth`; logout limpia ambos paths.
- Passwords: **Argon2id** (hash, no encriptación), formato PHC, rehash automático si cambian los parámetros.
- Nunca revelar si falló el usuario o la contraseña; mismo error y mismo tiempo.
- Producción: `COOKIE_SECURE=true`, HTTPS, `JWT_SECRET` ≥ 32 bytes aleatorios.

## Datos
- IDs UUIDv7 generados en la app (`uuid.NewV7()`); nunca autoincrementales.
- Dinero: `NUMERIC(19,4)` en BD y `shopspring/decimal` en Go. **Prohibido `float`.**
- Todas las tablas: `id`, `tenant_id`, `created_at`, `updated_at`. Fechas en `timestamptz` UTC.
- Migraciones inmutables: no editar una ya aplicada, crear una nueva.

## HTTP
- Prefijo `/api/v1`. JSON con sonic (inyectado en `fiber.Config`).
- DTOs de request/response viven en el adaptador HTTP, nunca se exponen entidades de dominio.
- Validar forma en el borde (`go-playground/validator`); invariantes de negocio en el dominio.
- Errores de dominio tipados → mapeo a HTTP solo en el adaptador. Formato: `{"error": "mensaje"}`.
- Documentar cada handler con anotaciones swag; regenerar el contrato embebido con `swag init -g cmd/api/main.go -o internal/platform/openapi --parseInternal`.

## Reglas de código
- **KISS / YAGNI:** nada de abstracciones hasta tener 2 usos reales. Sin generics ni reflexión innecesaria.
- **DRY con criterio:** duplicar es mejor que una abstracción equivocada.
- **SOLID en Go:** interfaces pequeñas (1–3 métodos) definidas por el consumidor; composición, no herencia; una responsabilidad por caso de uso.
- **Patrones permitidos:** Repository, Unit of Work (`WithTenant`), Strategy (cálculo de interés: simple/francés/alemán), Factory, Decorator. Evitar Singleton y estado global.
- **Anti code smells:** funciones ≤ ~30 líneas, archivos ≤ ~300, ≤ 4 parámetros, sin `else` tras `return`, sin comentarios que expliquen "qué" (solo "por qué"), sin números/strings mágicos.
- `context.Context` siempre primer parámetro. Errores envueltos con `%w`. Sin `panic` fuera de `main`.
- Logs con `slog` estructurado; nunca registrar passwords, tokens ni datos sensibles.
- Inyección de dependencias manual en `cmd/api/main.go`; sin frameworks de DI.

## Inyección de dependencias (regla central)
- **Todo se inyecta por constructor y depende de interfaces**, nunca de implementaciones concretas. Aplica a: repositorios, servicios, reloj (`authkit.Clock`), generador de IDs (`ids.Generator`), ejecutor de transacciones (`db.TxRunner`), middleware de auth, clientes externos.
- **Prohibido en el código de negocio:** `time.Now()`, `uuid.New*()`, `pgxpool`, `os.Getenv`, variables globales y `init()` con estado. Se piden por constructor.
- **Las interfaces las define quien las consume** (pequeñas, 1–3 métodos) y viven en su paquete: `domain` (puertos de persistencia), `app` o el handler (lo que necesitan de otros servicios).
- Constructores: `NewX(dep1, dep2) *X` devuelven structs concretos; aceptan interfaces.
- **Raíz de composición única:** `internal/bootstrap` es el único paquete que importa implementaciones de todos los módulos y las cablea. `cmd/api/main.go` solo carga config, abre recursos y arranca.
- Sin contenedores/frameworks de DI (wire, fx, dig): el cableado manual es explícito y suficiente. Reconsiderar solo si `bootstrap` supera ~300 líneas → dividir en `bootstrap/<modulo>.go`.
- Los tests inyectan fakes (`fakes_test.go`); no se usan monkey-patching ni globals para sustituir dependencias.
- Excepción aceptada: utilidades puras sin efectos (hashing, cálculos de interés, `strings`, `errors`) se llaman directo.

## Tamaño de archivos y organización de tests
- **Ningún archivo `.go` ni `.sql` supera 500 líneas** (`scripts/check-file-size.sh`, se corre en CI). Al acercarse a ~400, refactorizar *antes* de añadir más:
  - separar por responsabilidad (un caso de uso o un handler por archivo: `create.go`, `update.go`, `list.go`),
  - extraer tipos/DTOs a `dto.go`, mapeos a `mapper.go`, errores a `errors.go`,
  - si un paquete crece, dividirlo en subpaquetes por concepto del dominio (ej. `loans/schedule`, `loans/interest`), aplicando Strategy para variantes (plano/francés/alemán, un archivo por estrategia).
- **Tests nunca en el mismo archivo que el código.** Un `*_test.go` por caso de uso o comportamiento (`create_test.go`, `status_test.go`), con pruebas de caja negra (`package x_test`).
- Dobles de prueba (fakes) en `fakes_test.go` del paquete; no mezclar fakes con tests.
- Tests de integración (Postgres real, RLS) en `tests/integration/`, con build tag `integration`.
- Nombres de tests: `Test<Unidad>_<Comportamiento>`.

## Calidad
- `golangci-lint` (gocyclo, funlen, dupl, revive, gosec, errcheck), `go vet`, `go test -race ./...`.
- Dominio: tests unitarios puros. Repos: tests de integración con testcontainers contra Postgres real (verificar aislamiento RLS entre tenants).
- Todo cambio en auth requiere test que cubra cookie+CSRF y bearer.

## Eliminación: SOLO soft delete (regla estricta)
- Para el usuario/frontend existe `DELETE /recurso/:id` (204), pero **nunca** se ejecuta `DELETE FROM` sobre tablas de negocio: se marca `deleted_at = now()` (+ `deleted_by`). Única excepción: tablas técnicas efímeras (`refresh_tokens`). `scripts/check-no-hard-delete.sh` lo verifica en CI.
- Tablas de negocio con `deleted_at`: `tenants`, `users`, `customers` (y las que se añadan). Registros financieros (`payments`, `capital_movements`, `loan_adjustments`, `audit_log`) son **append-only**: no se borran ni siquiera lógicamente; se **anulan/revierten** con un movimiento nuevo.
- **Toda** consulta de lectura/actualización incluye `deleted_at IS NULL` (listados, get, búsqueda, login, refresh). Los índices de listado son **parciales** (`WHERE deleted_at IS NULL`) y las consultas deben repetir ese predicado para usarlos.
- Unicidad con soft delete: `customers(tenant_id, document_id)` y `users(tenant_id, lower(email))` son índices únicos **parciales** (`WHERE deleted_at IS NULL`) → se puede reutilizar un documento/correo tras eliminarlo. El `slug` de un tenant **nunca** se reutiliza (único global, incluso eliminado).
- Eliminar o desactivar un usuario o tenant bloquea su acceso; el middleware verifica el estado actual. Restablecer o cambiar la contraseña incrementa `session_version` y revoca refresh tokens para invalidar también los access tokens anteriores.
- Eliminar queda en `audit_log` (`<entidad>.delete`). Eliminar un recurso ya eliminado → 404 `NOT_FOUND`. Restaurar: solo superadmin (`POST /admin/.../restore`).
- Pendiente al crear préstamos: eliminar un cliente con préstamos activos → 409 `CUSTOMER_HAS_ACTIVE_LOANS`.

## Errores y paginación (contrato con el frontend)
- Detalle completo en `docs/API_CONVENCIONES.md`. Resumen: error único `{"error":{code,message,fields?,request_id}}`, `code` estable en inglés, `message` en **español**; listados con `{items, meta:{limit,offset,has_more,total?}}`, `limit` 1–100 (def. 20); `total` solo con `?include_total=true` (contar es lo más caro).
- Los handlers devuelven solo `*httpx.AppError`; el dominio no conoce HTTP. Un `code` publicado no se renombra.
- Toda lista usa `httpx.ParsePage` / `httpx.NewPage`; el repo pide `limit+1` filas para `has_more` y solo cuenta si `WithTotal`.
- En SQL: calificar `tabla.id` en `ORDER BY` si se selecciona `id::text`; no crear índices `pg_trgm` para ILIKE (no se usan bajo RLS, ver `docs/API_CONVENCIONES.md`). Listados grandes append-only → cursor (keyset), no `OFFSET`.
- Todo endpoint nuevo debe tener un test e2e de: caso feliz, validación (`fields`), 401/403, aislamiento entre tenants.

## Pruebas (3 niveles)
| Nivel | Qué | Comando |
|---|---|---|
| Unitarias | dominio/casos de uso con fakes | `go test ./... && (cd authkit && go test ./...)` |
| E2E | API completa + Postgres real temporal (Docker, RLS real) | `go test -tags=e2e ./tests/e2e -count=1` |
| Rendimiento | carga concurrente con umbral p95, benchmarks | `go test -tags=perf ./tests/perf -v -count=1` · `go test ./tests/bench -bench . -benchmem` · `cd authkit && go test -bench . -benchmem` |
- `docker-compose.test.yml` (Postgres en RAM, puerto 55432). `tests/testenv` lo levanta (`up --wait`), migra, crea el rol `app_user` sin superusuario (para que RLS aplique) y lo **borra siempre** al terminar (`down -v`), también con Ctrl+C. `KEEP_TEST_DB=1` lo conserva para depurar.
- E2E y perf comparten el puerto 55432: **no ejecutarlos a la vez**.
- Variables de perf: `PERF_DURATION`, `PERF_WORKERS`, `PERF_ROWS`, `PERF_P95_MS`.

## Comandos
```
go run ./cmd/api                      # requiere DATABASE_URL, JWT_SECRET (ver .env.example)
                                      # AUTO_MIGRATE=true aplica migraciones + GrantApp antes de escuchar
                                      # AUTO_SEED=true asegura el superadmin (nunca en production)
go run ./cmd/migrate up               # migraciones manuales (MIGRATE_DATABASE_URL o DATABASE_URL)
go run ./cmd/seed                     # superadmin manual (obligatorio en production)
goose -dir migrations postgres "$DATABASE_URL" up
swag init -g cmd/api/main.go -o internal/platform/openapi --parseInternal
go test ./... && (cd authkit && go test ./...)
```
