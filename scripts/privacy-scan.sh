#!/usr/bin/env bash
set -euo pipefail

tracked_files="$(git ls-files)"
failed=0

scan() {
  local label="$1"
  local pattern="$2"
  local allow="${3:-}"

  local matches
  matches="$(printf '%s\n' "$tracked_files" | grep -Ev '^(schemas/|go.sum$)' | xargs -r grep -nPI -- "$pattern" || true)"
  if [[ -n "$allow" && -n "$matches" ]]; then
    matches="$(printf '%s\n' "$matches" | grep -Ev "$allow" || true)"
  fi
  if [[ -n "$matches" ]]; then
    echo "Potential $label found:" >&2
    printf '%s\n' "$matches" >&2
    failed=1
  fi
}

scan "private key" '-----BEGIN (RSA |DSA |EC |OPENSSH |PGP )?PRIVATE KEY-----'
scan "secret assignment" '(?i)\b(api[_-]?key|access[_-]?token|secret|password|passwd)\b\s*[:=]\s*["'\'']?[A-Za-z0-9_./+=-]{12,}'
scan "email address" '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}'
scan "Swiss IBAN" '\bCH[0-9]{2}[0-9A-Z]{17}\b'

if [[ "$failed" -ne 0 ]]; then
  exit 1
fi

echo "No obvious secrets or personal identifiers found."
