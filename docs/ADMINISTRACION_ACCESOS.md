# Administración de empresas y accesos

## Flujo vigente

El administrador de plataforma crea la empresa y su primer usuario propietario desde **Empresas**. Después administra los usuarios de cada empresa: nombre, correo, rol y estado. Los registros se realizan en modales y las validaciones de negocio pertenecen al backend.

El acceso web pide únicamente **correo y contraseña**. La empresa se determina en el backend; no se configura por usuario mediante `VITE_DEFAULT_TENANT`. La identidad de empresa de las operaciones de negocio sigue saliendo de la sesión autenticada y se conserva el aislamiento de PostgreSQL mediante RLS.

## Contraseñas temporales

Al crear un acceso o restablecer su contraseña, el administrador entrega al usuario una contraseña temporal aleatoria de 8 caracteres, en minúsculas y números, sin caracteres ambiguos como `0`, `1`, `i`, `l` u `o`. Se muestra en el resultado de esa operación para que pueda copiarla; no se almacena en texto claro ni vuelve a aparecer en listados. No se utiliza el correo como contraseña. Las claves emitidas antes de este cambio siguen siendo válidas hasta que se cambien o restablezcan.

Al iniciar sesión con el acceso temporal, el usuario debe definir una contraseña propia antes de entrar al sistema. El backend aplica esta restricción también a las llamadas directas a la API: ocultar las pantallas en el frontend no sustituye el control de permisos.

Restablecer la contraseña invalida las sesiones anteriores y vuelve a exigir el cambio. Desactivar al usuario o su empresa bloquea el acceso. Estas acciones no eliminan préstamos, pagos ni el historial financiero.

## Contrato de acceso

- `POST /api/v1/auth/login`: `email`, `password` y `transport: "cookie"` para la web. El parámetro legado `tenant` se conserva por compatibilidad, pero la web no lo envía.
- `GET /api/v1/auth/me`: incluye `must_change_password` para dirigir al usuario a la pantalla correspondiente.
- `POST /api/v1/auth/change-password`: `current_password` y `new_password`. Exige una contraseña diferente, entre 10 y 128 caracteres, con letra y número; al terminar cierra la sesión y exige entrar con la nueva clave.
- `POST /api/v1/admin/tenants` y `POST /api/v1/admin/tenants/{id}/users`: permiten omitir la contraseña para generar una temporal y devolver `temporary_password` en esa respuesta.
- `POST /api/v1/admin/tenants/{id}/users/{userId}/reset-password`: devuelve una nueva `temporary_password` una sola vez. Solo está disponible para el administrador de plataforma.

El cambio obligatorio permite consultar la sesión, cerrar sesión o cambiar la contraseña; las demás operaciones responden `403 PASSWORD_CHANGE_REQUIRED`. Las mutaciones con cookies siguen exigiendo CSRF. El middleware consulta el estado de la cuenta y comprueba la versión de sesión de los tokens.

Los correos de cuentas sin eliminar se reservan entre empresas, incluidas las empresas suspendidas o eliminadas. Los duplicados históricos no se modifican: un login ambiguo se rechaza. Tanto la resolución por correo como la comprobación de unicidad recorren las empresas bajo su contexto RLS; su coste crece con el número de empresas.

## Verificación

Las pruebas deben cubrir alta de empresa y propietario, login sin empresa explícita, errores de campo, cambio obligatorio, restablecimiento, sesiones anteriores, usuarios o empresas inactivos y rechazo de administración desde una cuenta sin rol de plataforma. La comprobación real usa PostgreSQL local en Docker y Harness Browser.

La configuración de desarrollo y las cuentas ficticias de prueba no deben confundirse con datos de producción. Antes de ejecutar la API o migraciones locales, verificar que las conexiones apunten al PostgreSQL de desarrollo.

### Comprobaciones del 14 de noviembre de 2026

- `go test ./...` y pruebas del módulo `authkit`: correctas.
- Suite E2E completa con `TEST_DB_PORT=55433`, PostgreSQL temporal en Docker: correcta. Incluye cookies/CSRF, bearer, cambio obligatorio, claves incorrectas o repetidas, revocación de access/refresh, refresh de versión obsoleta, permisos, cuentas y empresas inactivas, restauración con correo duplicado y dos altas concurrentes con el mismo correo (una creación y un conflicto).
- Harness Browser sobre `localhost:5173`, API local `3720`, PostgreSQL de desarrollo `55432`: alta de empresa y propietario, entrega de contraseña temporal, cambio obligatorio y rechazo de navegación directa a préstamos; cambio y posterior ingreso; restablecimiento y rechazo de contraseña anterior; error de contraseña actual junto al campo; alta de usuario con rechazo de correo duplicado; desactivación y reactivación.
- Revisión visual en 393 × 852 y 1440 × 900: listas adaptadas a tarjetas en móvil, filtros, acciones, modales y pantalla de cambio de contraseña.

Swagger se genera con `swag init -g cmd/api/main.go -o internal/platform/openapi --parseInternal`. Las respuestas paginadas usan DTOs concretos en las anotaciones para mantener la generación compatible con Go 1.27. Consulta `docs/OPENAPI.md` para las rutas publicadas y la regeneración.
