#!/usr/bin/env bash

set -euo pipefail

if ! command -v git >/dev/null 2>&1; then
  echo "git is required for sync verification"
  exit 1
fi

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "not inside a git repository, skip sync verification"
  exit 0
fi

changed_files="$(git diff --name-only --cached)"

if [[ -z "${changed_files}" ]]; then
  echo "no staged changes"
  exit 0
fi

require_any() {
  local doc_changed="false"
  local file
  for file in "$@"; do
    if grep -qx "${file}" <<<"${changed_files}"; then
      doc_changed="true"
      break
    fi
  done

  if [[ "${doc_changed}" != "true" ]]; then
    echo "sync verification failed: expected one of the following files to be updated:"
    printf ' - %s\n' "$@"
    exit 1
  fi
}

if grep -E '^(backend/internal/model|backend/migrations)/' <<<"${changed_files}" >/dev/null; then
  require_any "docs/02-database-schema.md"
  require_any "CHANGELOG.md"
  require_any "docs/05-test-cases.md"
fi

if grep -E '^(backend/internal/api|frontend/src/api|api/openapi.yaml)/?' <<<"${changed_files}" >/dev/null; then
  require_any "docs/03-api.md"
  require_any "api/openapi.yaml"
  require_any "CHANGELOG.md"
  require_any "docs/05-test-cases.md"
fi

if grep -E '^(backend/internal/service|frontend/src/pages|frontend/src/components)/' <<<"${changed_files}" >/dev/null; then
  require_any "README.md"
  require_any "CHANGELOG.md"
  require_any "docs/05-test-cases.md"
fi

echo "sync verification passed"
