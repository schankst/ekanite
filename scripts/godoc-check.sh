#!/usr/bin/env bash
# godoc-check.sh — meldet exportierte Go-Symbole ohne GoDoc-Kommentar.
# Scannt rekursiv alle *.go (ohne *_test.go). Exit 1 wenn Verletzungen gefunden.
set -euo pipefail

violations=0

while IFS= read -r -d '' file; do
  if ! awk -v file="$file" '
  function check(name, lineno,    i, ok, start) {
    ok = 0
    i = lineno - 1
    if (i >= 1 && prev[i] ~ /^[[:space:]]*\/\//) {
      start = i
      while (start >= 1 && prev[start] ~ /^[[:space:]]*\/\//) start--
      start++
      for (i = start; i < lineno; i++) {
        if (prev[i] ~ "^[[:space:]]*//[[:space:]]*" name "\\b") { ok = 1; break }
      }
    }
    if (!ok) {
      printf "%s:%d: exported %s %s has no doc comment\n", file, lineno, kind, name
      found++
    }
  }
  { prev[NR] = $0 }
  /^[[:space:]]*func[[:space:]]+\(/ {
    line = $0
    sub(/^[[:space:]]*func[[:space:]]+\([^)]*\)[[:space:]]+/, "", line)
    if (match(line, /^([A-Z][A-Za-z0-9_]*)/, m)) { kind="func"; check(m[1], NR) }
    next
  }
  /^[[:space:]]*func[[:space:]]+[A-Z]/ {
    if (match($0, /^[[:space:]]*func[[:space:]]+([A-Z][A-Za-z0-9_]*)/, m)) { kind="func"; check(m[1], NR) }
    next
  }
  /^[[:space:]]*type[[:space:]]+[A-Z]/ {
    if (match($0, /^[[:space:]]*type[[:space:]]+([A-Z][A-Za-z0-9_]*)/, m)) { kind="type"; check(m[1], NR) }
    next
  }
  /^[[:space:]]*var[[:space:]]+[A-Z]/ {
    if (match($0, /^[[:space:]]*var[[:space:]]+([A-Z][A-Za-z0-9_]*)/, m)) { kind="var"; check(m[1], NR) }
    next
  }
  /^[[:space:]]*const[[:space:]]+[A-Z]/ {
    if (match($0, /^[[:space:]]*const[[:space:]]+([A-Z][A-Za-z0-9_]*)/, m)) { kind="const"; check(m[1], NR) }
    next
  }
  END { exit(found > 0 ? 1 : 0) }
  ' "$file"; then
    violations=$((violations + 1))
  fi
done < <(find . -name '*.go' ! -name '*_test.go' -print0 | sort -z)

if [ "$violations" -gt 0 ]; then
  echo "godoc-check: $violations file(s) with undocumented exported symbols" >&2
  exit 1
fi

echo "godoc-check: OK (no undocumented exported symbols)"
exit 0
