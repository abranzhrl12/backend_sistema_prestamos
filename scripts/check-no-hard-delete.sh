#!/usr/bin/env bash
# Falla si hay "DELETE FROM" sobre una tabla distinta de refresh_tokens (regla de soft delete de CLAUDE.md).
# Excepción deliberada: el test negativo de capital borra un movimiento a propósito para comprobar que
# el trigger de la BD lo rechaza; solo se permite ese archivo y esa tabla.
# Uso: bash scripts/check-no-hard-delete.sh
cd "$(dirname "$0")/.." || exit 1

allowed_table="refresh_tokens"
allowed_file="tests/e2e/capital_list_test.go"
allowed_negative_table="capital_movements"
dirs="internal cmd tests authkit migrations"
fail=0

hits=$(grep -rnoiE --include='*.go' --include='*.sql' 'DELETE[[:space:]]+FROM[[:space:]]+[a-z_]+' $dirs 2>/dev/null)
while IFS= read -r hit; do
  [ -z "$hit" ] && continue
  file="${hit%%:*}"
  table=$(printf '%s' "${hit##* }" | tr 'A-Z' 'a-z')
  if [ "$table" = "$allowed_table" ]; then continue; fi
  if [ "$file" = "$allowed_file" ] && [ "$table" = "$allowed_negative_table" ]; then continue; fi
  echo "DELETE FROM prohibido (usa soft delete): $hit"
  fail=1
done <<< "$hits"

# DELETE FROM con el nombre de la tabla en la línea siguiente: no se puede verificar, se rechaza.
dangling=$(grep -rnE --include='*.go' --include='*.sql' 'DELETE[[:space:]]+FROM[[:space:]]*$' $dirs 2>/dev/null)
if [ -n "$dangling" ]; then
  echo "DELETE FROM sin tabla en la misma línea (no verificable):"
  echo "$dangling"
  fail=1
fi

[ "$fail" -eq 0 ] && echo "OK: no hay DELETE FROM sobre tablas de negocio."
exit $fail
