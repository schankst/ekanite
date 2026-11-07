#!/usr/bin/env bash
# godoc-check.sh — meldet exportierte Go-Symbole ohne GoDoc-Kommentar.
# Scannt rekursiv alle *.go (ohne *_test.go). Exit 1 wenn Verletzungen gefunden.
set -uo pipefail

violations=0
total=0

while IFS= read -r -d '' file; do
  extra=$(awk -v file="$file" '
  function isComment(line) { return line ~ /^[[:space:]]*\// }
  function hasDoc(symline, i,   j) {
    # scan backwards over contiguous comment lines directly above
    j = i - 1
    while (j >= 1 && isComment(prev[j])) j--
    j++
    return (j < i)   # at least one comment line directly above
  }
  { prev[NR] = $0; isData[NR] = 0 }
  # track block-comment / raw-string contexts minimally: skip lines that end a /* */
  { data = data $0 }
  END {}
  /^[[:space:]]*func[[:space:]]+\(/ {
    line = $0
    sub(/^[[:space:]]*func[[:space:]]+\([^)]*\)[[:space:]]+/, "", line)
    if (match(line, /^[A-Z][A-Za-z0-9_]*/)) {
      name = substr(line, RSTART, RLENGTH)
      if (!hasDoc($0, NR)) { printf "%s:%d: exported func %s has no doc comment\n", file, NR, name; v++ }
    }
    next
  }
  /^[[:space:]]*func[[:space:]]+[A-Z]/ {
    if (match($0, /^[[:space:]]*func[[:space:]]+([A-Z][A-Za-z0-9_]*)/, m)) {
      if (!hasDoc($0, NR)) { printf "%s:%d: exported func %s has no doc comment\n", file, NR, m[1]; v++ }
    }
    next
  }
  /^[[:space:]]*type[[:space:]]+[A-Z]/ {
    if (match($0, /^[[:space:]]*type[[:space:]]+([A-Z][A-Za-z0-9_]*)/, m)) {
      if (!hasDoc($0, NR)) { printf "%s:%d: exported type %s has no doc comment\n", file, NR, m[1]; v++ }
    }
    next
  }
  /^[[:space:]]*(var|const)[[:space:]]+[A-Z]/ {
    if (match($0, /^[[:space:]]*(var|const)[[:space:]]+([A-Z][A-Za-z0-9_]*)/, m)) {
      if (!hasDoc($0, NR)) { printf "%s:%d: exported %s %s has no doc comment\n", file, NR, m[1], m[2]; v++ }
    }
    next
  }
  END { if (v>0) exit 1; exit 0 }
  ' "$file")
  rc=$?
  if [ -n "$extra" ]; then
    printf '%s\n' "$extra"
    violations=$((violations + 1))
  fi
  total=$((total + rc))
done < <(find . -name '*.go' ! -name '*_test.go' -print0 | sort -z)

if [ "$violations" -gt 0 ]; then
  echo "godoc-check: $violations file(s) with undocumented exported symbols" >&2
  exit 1
fi

echo "godoc-check: OK (no undocumented exported symbols)"
exit 0