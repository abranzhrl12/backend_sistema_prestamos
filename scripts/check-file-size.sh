#!/usr/bin/env bash
# Falla si algún archivo fuente (.go, .sql) supera MAX líneas. Uso: scripts/check-file-size.sh [max]
max="${1:-500}"
fail=0
while IFS= read -r f; do
  n=$(wc -l < "$f")
  if [ "$n" -gt "$max" ]; then echo "DEMASIADO LARGO ($n > $max): $f"; fail=1; fi
done < <(find . -type f \( -name '*.go' -o -name '*.sql' \) -not -path './docs/*' -not -path './.git/*')
exit $fail
