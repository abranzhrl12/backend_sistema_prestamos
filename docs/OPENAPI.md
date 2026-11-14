# API contract and Swagger UI

The generated Swagger 2.0 contract is versioned with the API source and compiled into the application binary. It is published without authentication because it contains no tenant data or credentials.

| Resource | URL |
| --- | --- |
| Swagger UI | `/docs` |
| JSON contract | `/openapi.json` |
| YAML contract | `/openapi.yaml` |

The API itself remains under `/api/v1`. Protected operations describe `BearerAuth` in the contract. The browser application can also use HttpOnly cookies; state-changing cookie requests require `X-CSRF-Token`.

## Regenerating the contract

Run this from the repository root after changing an HTTP handler, DTO, route, or Swagger annotation:

```powershell
swag init -g cmd/api/main.go -o internal/platform/openapi --parseInternal
```

Commit all three generated files in `internal/platform/openapi`: `docs.go`, `swagger.json`, and `swagger.yaml`. The generator does not parse external Go dependencies; response pages have concrete documentation DTOs instead of generic types so generation remains deterministic on Go 1.27.
